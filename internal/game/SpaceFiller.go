package game

import (
	"sync"
	"sync/atomic"
)

type SpaceFiller struct {
	SpaceFillerChan chan *Player
	GameMap         [][]*Tile
}

var spaceFiller *SpaceFiller

func getNewSpaceFiller(gameMap [][]*Tile) *SpaceFiller {
	if spaceFiller != nil {
		return spaceFiller
	}

	spaceFiller := SpaceFiller{
		SpaceFillerChan: make(chan *Player),
		GameMap:         gameMap,
	}

	spaceFillerChannelWorkers := 100
	for w := 0; w < spaceFillerChannelWorkers; w++ {
		go spaceFiller.spaceFillWorker()
	}

	return &spaceFiller
}

func (spaceFillerInstance *SpaceFiller) spaceFillWorker() {
	for {
		player, ok := <-spaceFillerInstance.SpaceFillerChan
		if !ok {
			return
		}

		if player.isDead {
			return
		}

		if player != nil && len(player.Tail) > 1 {
			spaceFillerInstance.spaceFillFromTail(player.Tail)
			player.resetTailData()
		}
	}
}

func (sf *SpaceFiller) spaceFillFromTail(tail []*Tile) {
	for _, segment := range tail {
		segmentRow, segmentCol := segment.Y, segment.X

		topTile, bottomTile, leftTile, rightTile := sf.GameMap[segmentRow-1][segmentCol],
			sf.GameMap[segmentRow+1][segmentCol],
			sf.GameMap[segmentRow][segmentCol-1],
			sf.GameMap[segmentRow][segmentCol+1]

		if topTile.OwnerColor != segment.OwnerColor &&
			bottomTile.OwnerColor != segment.OwnerColor &&
			!IsWall(topTile.Y, topTile.X) &&
			!IsWall(bottomTile.Y, bottomTile.X) {
			sf.fillWithSeeds(segment.OwnerColor, topTile, bottomTile)
		} else if leftTile.OwnerColor != segment.OwnerColor &&
			rightTile.OwnerColor != segment.OwnerColor &&
			!IsWall(leftTile.Y, leftTile.X) &&
			!IsWall(leftTile.Y, leftTile.X) {
			sf.fillWithSeeds(segment.OwnerColor, topTile, bottomTile)
		}

		segment.IsTail = false
	}
}

func (sf *SpaceFiller) fillWithSeeds(color *int, seedA *Tile, seedB *Tile) {
	areaFound := &atomic.Bool{}
	areaFound.Store(false)
	wg := &sync.WaitGroup{}
	wg.Add(2)
	go sf.findAndFillTiles(color, seedA, wg, areaFound)
	go sf.findAndFillTiles(color, seedB, wg, areaFound)
	wg.Wait()
}

func (sf *SpaceFiller) findAndFillTiles(
	playerColor *int,
	seed *Tile,
	wg *sync.WaitGroup,
	tilesFound *atomic.Bool,
) {
	defer wg.Done()
	if tilesFound.Load() {
		return
	}
	q := []*Tile{seed}
	mapOfTilesToIgnore := make(map[*Tile]interface{})

	for len(q) > 0 {
		if tilesFound.Load() {
			return
		}

		testCoord := q[0]
		q = q[1:]

		testTile := sf.GameMap[testCoord.Y][testCoord.X]
		mapOfTilesToIgnore[testTile] = true

		for _, dir := range Directions {
			if tilesFound.Load() {
				return
			}

			testRow, testCol := testTile.Y+dir[0], testTile.X+dir[1]
			if IsWall(testRow, testCol) {
				return
			}

			nextTile := sf.GameMap[testRow][testCol]
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
			tilesFound.Store(true)
			for tile := range mapOfTilesToIgnore {
				tile.OwnerColor = playerColor
				tile.IsTail = false
			}
		}
	}
}
