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
var MapColCount = 20
var MapRowCount = 20

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

		if gm.isOtherPlayerTail(nextTile) {
			gm.UpdateChannel <- PlayerDeadMsg{
				*player.Color,
			}
			return
		}

		if gm.isPlayersTail(nextTile, player.Color) {
			//space fill algorithm
			//that will swap isTail flags to False
			//update player stats
		}

		if nextTile.OwnerColor != player.Color {
			nextTile.OwnerColor = player.Color
			nextTile.IsTail = true
		}

		// Update player's location
		player.Location = nextTile
	}

	// TODO: Implement broadcasting the updated GameMap state to all players/UI
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

func (gm *GameManager) isOtherPlayerTail(tile *Tile) bool {
	return tile.IsTail && tile.OwnerColor != nil
}

func (gm *GameManager) isPlayersTail(tile *Tile, playerColor *int) bool {
	return tile.IsTail && tile.OwnerColor == playerColor
}
