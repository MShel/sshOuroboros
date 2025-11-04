package game

import (
	"sync"
	"sync/atomic"
)

type SpaceFiller struct {
	SpaceFillerChan chan *Player
	GameMap         [][]*Tile
	SpaceFillerWg   *sync.WaitGroup
}

var spaceFiller *SpaceFiller

func getNewSpaceFiller(gameMap [][]*Tile) *SpaceFiller {
	if spaceFiller != nil {
		return spaceFiller
	}

	spaceFiller := SpaceFiller{
		SpaceFillerChan: make(chan *Player),
		GameMap:         gameMap,
		SpaceFillerWg:   &sync.WaitGroup{},
	}

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

		if player != nil && len(player.Tail.tailTiles) > 0 {
			spaceFillerInstance.SpaceFillerWg.Add(1)
			spaceFillerInstance.spaceFillFromTail(player)
			player.resetTailData()
		}
	}
}

func (sf *SpaceFiller) spaceFillFromTail(player *Player) {
	spaceFilled := false
	defer sf.SpaceFillerWg.Done()
	player.Tail.tailLock.Lock()
	defer player.Tail.tailLock.Unlock()

	for i := (len(player.Tail.tailTiles) - 1); i >= 0; i-- {
		segment := player.Tail.tailTiles[i]
		segmentRow, segmentCol := segment.Y, segment.X

		topTile, bottomTile, leftTile, rightTile := sf.GameMap[segmentRow-1][segmentCol],
			sf.GameMap[segmentRow+1][segmentCol],
			sf.GameMap[segmentRow][segmentCol-1],
			sf.GameMap[segmentRow][segmentCol+1]

		if !spaceFilled && player.Location != segment {
			if topTile.OwnerColor != segment.OwnerColor &&
				bottomTile.OwnerColor != segment.OwnerColor &&
				leftTile.OwnerColor == segment.OwnerColor &&
				rightTile.OwnerColor == segment.OwnerColor {
				spaceFilled = true
				sf.fillWithSeeds(player, topTile, bottomTile)
			} else if leftTile.OwnerColor != segment.OwnerColor &&
				rightTile.OwnerColor != segment.OwnerColor &&
				bottomTile.OwnerColor == segment.OwnerColor &&
				topTile.OwnerColor == segment.OwnerColor {
				spaceFilled = true
				sf.fillWithSeeds(player, leftTile, rightTile)
			}
		}
		player.AllTiles.allTilesLock.Lock()
		player.AllTiles.AllPlayerTiles = append(player.AllTiles.AllPlayerTiles, segment)
		player.AllTiles.allTilesLock.Unlock()
		segment.IsTail = false
	}
}

func (sf *SpaceFiller) fillWithSeeds(player *Player, seedA *Tile, seedB *Tile) {
	areaFound := &atomic.Bool{}
	areaFound.Store(false)
	wg := &sync.WaitGroup{}
	if !IsWall(seedA.Y, seedA.X) {
		wg.Add(1)
		go sf.findAndFillTiles(player, seedA, wg, areaFound)
	}

	if !IsWall(seedB.Y, seedB.X) {
		wg.Add(1)
		go sf.findAndFillTiles(player, seedB, wg, areaFound)
	}

	wg.Wait()
}

func (sf *SpaceFiller) findAndFillTiles(
	player *Player,
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

			if nextTile.OwnerColor == player.Color {
				continue
			}

			q = append(q, nextTile)
			mapOfTilesToIgnore[nextTile] = true
		}

		if len(q) == 0 && len(mapOfTilesToIgnore) > 1 {
			tilesFound.Store(true)

			for tile := range mapOfTilesToIgnore {
				tile.OwnerColor = player.Color
				tile.IsTail = false
				player.AllTiles.allTilesLock.Lock()
				player.AllTiles.AllPlayerTiles = append(player.AllTiles.AllPlayerTiles, tile)
				player.AllTiles.allTilesLock.Unlock()
			}
		}
	}
}
