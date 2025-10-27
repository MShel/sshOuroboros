package game

import (
	"sync"
	"sync/atomic"
)

const maxConcurrentFillers = 25

func (gm *GameManager) spaceFill(player *Player) {
	semaphore := make(chan struct{}, maxConcurrentFillers)
	resultsChannel := make(chan map[*Tile]interface{}, maxConcurrentFillers) // buffered to avoid blocking
	var tilesFound atomic.Bool                                               // Declare an atomic.Bool variable

	// Set the flag initially to false
	tilesFound.Store(false)

	tailPointer := 0
	var wg sync.WaitGroup
	// Explore every tail segment
	for tailPointer < len(player.Tail) && !tilesFound.Load() {
		segment := player.Tail[tailPointer]
		for _, dir := range directions {
			if tilesFound.Load() {
				break
			}

			testRow, testCol := segment.Y+dir[0], segment.X+dir[1]
			testTile := gm.GameMap[testRow][testCol]

			if testTile.OwnerColor != player.Color {
				if tilesFound.Load() {
					break
				}

				semaphore <- struct{}{}
				wg.Add(1)

				go func(tt *Tile) {
					defer wg.Done()
					defer func() { <-semaphore }()

					gm.getTilesToBeFilled(tt, player.Color, resultsChannel, &tilesFound)
				}(testTile)
			}
		}
		tailPointer += 1
	}

	// Wait for goroutines to finish and close channel
	go func() {
		wg.Wait()
		close(resultsChannel)
	}()

	for mapOfTiles := range resultsChannel {
		if len(mapOfTiles) > 1 {

			for tile := range mapOfTiles {
				gm.GameMap[tile.Y][tile.X].OwnerColor = player.Color
				gm.GameMap[tile.Y][tile.X].IsTail = false
			}
		}

	}

	// Mark the tail as no longer tail
	for _, tile := range player.Tail {
		tile.IsTail = false
	}
}

func (gm *GameManager) getTilesToBeFilled(
	seed *Tile,
	playerColor *int,
	resultsChan chan map[*Tile]interface{},
	tilesFound *atomic.Bool,
) {
	if tilesFound.Load() {
		return
	}
	q := []*Tile{seed}
	mapOfTilesToIgnore := make(map[*Tile]interface{})

	for len(q) > 0 {
		if tilesFound.Load() {
			return
		}

		// check cancellation *very* often for snappy shutdown

		testCoord := q[0]
		q = q[1:]

		testTile := gm.GameMap[testCoord.Y][testCoord.X]
		mapOfTilesToIgnore[testTile] = true

		for _, dir := range directions {
			if tilesFound.Load() {
				return
			}

			testRow, testCol := testTile.Y+dir[0], testTile.X+dir[1]
			if gm.isWall(testRow, testCol) {
				return
			}

			nextTile := gm.GameMap[testRow][testCol]
			if _, ok := mapOfTilesToIgnore[nextTile]; ok {
				continue
			}

			if nextTile.OwnerColor == playerColor {
				continue
			}

			q = append(q, nextTile)
			mapOfTilesToIgnore[nextTile] = true
		}

		if len(q) == 0 {
			select {
			case resultsChan <- mapOfTilesToIgnore:
				tilesFound.Store(true)
			default:
			}
		}
	}
}
