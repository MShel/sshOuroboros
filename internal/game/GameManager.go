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
var mapColCount = 1024
var mapRowCount = 768

func GetNewGameManager() *GameManager {
	if singletonGameManager != nil {
		return singletonGameManager
	}

	// 🚨 FIX 1: Correctly assign to the global singletonGameManager
	singletonGameManager = &GameManager{
		// 🚨 FIX 2: Initialize channels here (Important!)
		DirectionChannel: make(chan Direction, 10),
		UpdateChannel:    make(chan tea.Msg), // Use tea.Msg for compatibility
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
		if nextTile == nil {
			continue // Should not happen, but safe check
		}

		// 1. Collision Check (Crucial for Ssshnake)
		if nextTile.OwnerColor != nil && nextTile.OwnerColor != player.Color {
			//log.Printf("Player %s collided with color %d!", player.Name, nextTile.OwnerColor)
			continue
		}
		if nextTile.OwnerColor == player.Color && !nextTile.IsTail {
			//log.Printf("Player %s color %d owner color %d collided with itself!", player.Name, *player.Color, *nextTile.OwnerColor)
			continue
		}

		// 2. Move Player (Update Map and Player location)
		//log.Printf("grubbed it %d, %d", nextTile.X, nextTile.Y)

		// Mark the new tile as owned by the player
		nextTile.OwnerColor = player.Color
		nextTile.IsTail = false // New head is not the tail

		// Update player's location
		player.Location = nextTile

		// 3. Trail Management (Simple logic for now: remove tail after a delay)
		// 🚨 TODO: Implement robust tail management and ClaimedEstate update
	}

	// 🚨 TODO: Implement broadcasting the updated GameMap state to all players/UI
}
