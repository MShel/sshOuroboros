package game

import (
	"log"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type Direction struct {
	Dx, Dy      int
	PlayerColor int
}

type GameTickMsg struct{}
type PlayerDeadMsg struct {
	PlayerColor int
}

type GameManager struct {
	// map int for color and pointer to player(null if color is not allocated)
	// we will init map with 256 colors for max players
	Players          map[int]*Player
	GameMap          [][]*Tile
	DirectionChannel chan Direction
	UpdateChannel    chan tea.Msg
	//tha will go away once we will have a server and multiplayer and blackjack
	CurrentPlayerColor int
	IsRunning          bool
}

var singletonGameManager *GameManager
var MapColCount = 80
var MapRowCount = 40

func GetNewGameManager() *GameManager {
	if singletonGameManager != nil {
		return singletonGameManager
	}

	singletonGameManager = &GameManager{
		DirectionChannel: make(chan Direction, 10),
		UpdateChannel:    make(chan tea.Msg),
		IsRunning:        false,
	}
	singletonGameManager.Players = make(map[int]*Player)
	// 256 colors --- 256 players
	for i := 0; i < 256; i++ {
		singletonGameManager.Players[i] = nil
	}

	singletonGameManager.GameMap = getInitGameMap()

	return singletonGameManager
}

func (gm *GameManager) StartGameLoop() {
	if gm.IsRunning {
		return
	}
	gm.IsRunning = true
	log.Println("Game loop started.")
	duration := 100 * time.Millisecond

	ticker := time.NewTicker(duration)
	defer ticker.Stop()

	for gm.IsRunning {
		select {
		case <-ticker.C:
			// 1. GAME TICK: Process the game state
			gm.processGameTick()
			gm.UpdateChannel <- GameTickMsg{}
		case dir := <-gm.DirectionChannel:
			// 2. INPUT: Process immediate player input
			gm.processPlayerInput(dir)
		}
	}
	log.Println("Game loop stopped.")
}

// processPlayerInput immediately updates the direction of the specified player.
func (gm *GameManager) processPlayerInput(dir Direction) {
	player := gm.Players[dir.PlayerColor]
	if player != nil {
		player.UpdateDirection(dir)
	}
}

// processGameTick is called every GameTickDuration to move all players and check collisions.
func (gm *GameManager) processGameTick() {
	// In a multiplayer game, you would typically lock access to GameMap here.

	for _, player := range gm.Players {
		if player == nil {
			continue
		}

		nextTile := player.GetNextTile()
		if gm.isWall(nextTile) {
			gm.UpdateChannel <- PlayerDeadMsg{
				*player.Color,
			}
			return
		}

		if gm.isOtherPlayerTail(nextTile, player.Color) {
			gm.UpdateChannel <- PlayerDeadMsg{
				*player.Color,
			}
			return
		}

		if nextTile.OwnerColor == player.Color && len(player.Tail) > 3 {
			player.addTileToTail(nextTile)
			gm.spaceFill(player)
			player.resetTailData()
		}

		if nextTile.OwnerColor != player.Color {
			nextTile.OwnerColor = player.Color
			nextTile.IsTail = true
		}

		player.addTileToTail(nextTile)
		// Update player's location
		player.Location = nextTile
	}

	// TODO: Implement broadcasting the updated GameMap state to all players/UI
}

func (gm *GameManager) spaceFill(player *Player) {
	// if true this tile does not need to be filled
	mapOfTilesToIgnore := make(map[*Tile]interface{})
	directions := [][]int{
		{1, 0},
		{0, 1},
		{-1, 0},
		{1, 0},
	}

	ignoreQ := [][]int{
		{max(player.MinTailRow-1, 0), max(player.MinTailCol-1, 0)},
		{max(player.MinTailRow-1, 0), min(player.MaxTailCol+1, MapColCount-1)},
		{min(player.MaxTailRow+1, MapRowCount-1), max(player.MinTailCol-1, 0)},
		{min(player.MaxTailRow+1, MapRowCount-1), min(player.MaxTailCol+1, MapColCount-1)},
	}
	//derpColor := 21412
	for len(ignoreQ) > 0 {
		testCoord := ignoreQ[0]
		ignoreQ = ignoreQ[1:]

		testTile := gm.GameMap[testCoord[0]][testCoord[1]]
		//	testTile.OwnerColor = &derpColor
		mapOfTilesToIgnore[testTile] = true

		for _, dir := range directions {
			testRow, testCol := testTile.Y+dir[0], testTile.X+dir[1]
			if testRow < player.MinTailRow-1 || testCol < player.MinTailCol-1 {
				continue
			}

			if testRow > player.MaxTailRow+1 || testCol > player.MaxTailCol+1 {
				continue
			}

			testTile := gm.GameMap[testRow][testCol]
			if _, ok := mapOfTilesToIgnore[testTile]; ok {
				continue
			}

			if testTile.IsTail && testTile.OwnerColor == player.Color {
				continue
			}

			ignoreQ = append(ignoreQ, []int{testTile.Y, testTile.X})
			mapOfTilesToIgnore[testTile] = true
		}

	}

	q := player.Tail
	for len(q) > 0 {
		tile := q[0]
		q = q[1:]
		tile.IsTail = false
		tile.OwnerColor = player.Color
		mapOfTilesToIgnore[tile] = true

		for _, dir := range directions {
			testRow, testCol := tile.Y+dir[0], tile.X+dir[1]
			if testRow < player.MinTailRow || testCol < player.MinTailCol {
				continue
			}

			if testRow > player.MaxTailRow || testCol > player.MaxTailCol {
				continue
			}

			testTile := gm.GameMap[testRow][testCol]
			if _, ok := mapOfTilesToIgnore[testTile]; ok {
				continue
			}

			q = append(q, testTile)
			mapOfTilesToIgnore[testTile] = true
		}
	}
}

func (gm *GameManager) isWall(tile *Tile) bool {
	if tile.X == 0 || tile.Y == 0 {
		return true
	}

	if tile.X == MapColCount || tile.Y == MapRowCount {
		return true
	}

	return false
}

func (gm *GameManager) isOtherPlayerTail(tile *Tile, playerColor *int) bool {
	return tile.IsTail && tile.OwnerColor != nil && playerColor != tile.OwnerColor
}
