package game

import (
	"context"
	"log"
	"math"
	"strconv"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
)

type Direction struct {
	Dx, Dy      int
	PlayerColor int
}

type GameTickMsg struct{}
type PlayerDeadMsg struct {
	PlayerColor int
}

// players estate is recalculated every time for all players once any of them claim any new estate
type ClaimedEstateMsg struct {
	PlayersEstate map[*int]int
}

type GameManager struct {
	// Players is now a sync.Map for concurrent access, mapping int (color) to *Player.
	// This replaces the map[int]*Player and the PlayersMutex.
	Players              sync.Map
	SessionsToPlayers    sync.Map
	GameMap              [][]*Tile
	DirectionChannel     chan Direction
	SunsetPlayersChannel chan *Player

	IsRunning     bool
	MapMutex      sync.RWMutex
	cancelContext context.CancelFunc
	GameContext   context.Context
}

var singletonGameManager *GameManager
var MapColCount = 200
var MapRowCount = 200

func GetNewGameManager() *GameManager {
	if singletonGameManager != nil {
		return singletonGameManager
	}

	gameContex, cancel := context.WithCancel(context.Background()) // Create cancellable context

	singletonGameManager = &GameManager{
		DirectionChannel:     make(chan Direction, 1024),
		SunsetPlayersChannel: make(chan *Player, 1024),
		IsRunning:            false,
		cancelContext:        cancel,
		GameContext:          gameContex,
	}
	singletonGameManager.GameMap = getInitGameMap()
	singletonGameManager.intializeBotControledPlayers(256)

	return singletonGameManager
}

func (gm *GameManager) broadcast(msg tea.Msg) {
	gm.Players.Range(func(key, value interface{}) bool {
		if player, ok := value.(*Player); ok && player != nil && player.BotStrategy == nil {
			select {
			case player.UpdateChannel <- msg:
			default:
				log.Printf("Player %s update channel full, dropping message of type %T", player.Name, msg)
			}
		}
		return true
	})
}

func (gm *GameManager) StartGameLoop() {
	if gm.IsRunning {
		return
	}
	gm.IsRunning = true
	duration := 100 * time.Millisecond
	sunsetWorkersCount := 30
	for w := 1; w <= sunsetWorkersCount; w++ {
		go gm.sunsetPlayersWorker()
	}

	ticker := time.NewTicker(duration)
	defer ticker.Stop()

	for gm.IsRunning {
		select {
		case <-ticker.C:
			gm.processGameTick()
			gm.broadcast(GameTickMsg{})
		case dir := <-gm.DirectionChannel:
			gm.processPlayerInput(dir)
		}
	}
	log.Println("Game loop stopped.")
}

func (gm *GameManager) StopGameLoop() {
	if !gm.IsRunning {
		return
	}
	gm.IsRunning = false
	gm.cancelContext()
	close(gm.DirectionChannel)
	close(gm.SunsetPlayersChannel)
}

func (gm *GameManager) processPlayerInput(dir Direction) {
	if p, ok := gm.Players.Load(dir.PlayerColor); ok {
		if player, ok := p.(*Player); ok && player != nil {
			if dir != player.CurrentDirection {
				player.UpdateDirection(dir)
			}
		}
	}
}

// processGameTick is called every GameTickDuration to move all players and check collisions.
func (gm *GameManager) processGameTick() {

	// Replaced manual locking and iteration with sync.Map.Range
	//TODO Remove this  slice and use range directly
	gm.Players.Range(func(key, value interface{}) bool {
		if player, ok := value.(*Player); ok && player != nil {
			if player == nil {
				return true
			}

			nextTile := player.GetNextTile()
			if gm.isWall(nextTile.Y, nextTile.X) {
				gm.SunsetPlayersChannel <- player
				return true
			}

			if gm.isOtherPlayerTail(nextTile, player.Color) {

				var otherPlayer *Player
				if nextTile.IsTail && nextTile.OwnerColor != nil {
					if p, ok := gm.Players.Load(*nextTile.OwnerColor); ok {
						if op, ok := p.(*Player); ok {
							otherPlayer = op
						}
					}
				}

				if otherPlayer != nil {
					gm.SunsetPlayersChannel <- otherPlayer

					nextTile.OwnerColor = player.Color
					nextTile.IsTail = true
					player.Tail = append(player.Tail, nextTile)
					player.Location = nextTile
					return true
				}
			}

			if nextTile.OwnerColor == player.Color && len(player.Tail) > 0 {
				gm.spaceFill(player)
				player.resetTailData()
				player.Location = nextTile
				return true
			}

			if nextTile.OwnerColor != player.Color {
				nextTile.OwnerColor = player.Color
				nextTile.IsTail = true
				player.Tail = append(player.Tail, nextTile)
			}

			// Update player's location
			player.Location = nextTile

			if player.BotStrategy != nil {
				go func() {
					nextDirection := player.BotStrategy.getNextBestDirection(player, gm)
					if nextDirection.Dx != player.CurrentDirection.Dx || nextDirection.Dy != player.CurrentDirection.Dy {
						player.UpdateDirection(nextDirection)
					}
				}()
			}
		}
		return true // continue iteration
	})
}

func (gm *GameManager) CreateNewPlayer(playerName string, playerColor int, userSession ssh.Session) *Player {
	spawnTile := gm.getSpawnTile()
	newPlayer := CreateNewPlayer(userSession, playerName, playerColor, spawnTile)
	gm.Players.Store(playerColor, newPlayer)
	gm.SessionsToPlayers.Store(userSession, newPlayer)

	return newPlayer
}

func (gm *GameManager) getSpawnTile() *Tile {
	var activePlayers []*Player

	// Rewritten loop to use sync.Map.Range for concurrent safety
	gm.Players.Range(func(key, value interface{}) bool {
		if player, ok := value.(*Player); ok && player != nil {
			activePlayers = append(activePlayers, player)
		}
		return true
	})

	if len(activePlayers) == 0 {
		return gm.GameMap[MapRowCount/2][MapColCount/2]
	}

	bestTile := gm.GameMap[MapRowCount/2][MapColCount/2]
	maxMinDistance := -1 // Maximize the minimum distance to any player

	for row := 1; row < MapRowCount-1; row++ {
		for col := 1; col < MapColCount-1; col++ {
			currentTile := gm.GameMap[row][col]

			if currentTile.OwnerColor != nil || currentTile.IsTail {
				continue
			}

			minDistanceToPlayer := math.MaxInt32

			for _, player := range activePlayers {
				dist := getManhattanDistance(currentTile, player.Location)
				if dist < minDistanceToPlayer {
					minDistanceToPlayer = dist
				}
			}

			if minDistanceToPlayer > maxMinDistance {
				maxMinDistance = minDistanceToPlayer
				bestTile = currentTile
			}
		}
	}

	return bestTile
}

func (gm *GameManager) sunsetPlayersWorker() {
	for {
		player, ok := <-gm.SunsetPlayersChannel
		if !ok {
			return
		}
		if player != nil {
			gm.sunsetPlayer(player)
		}
	}
}

func (gm *GameManager) sunsetPlayer(player *Player) {
	// Only MapMutex is needed to protect the GameMap tiles.
	gm.MapMutex.Lock()
	defer gm.MapMutex.Unlock()
	playerFinalClaimedLand := 0.0

	for row := 1; row < MapRowCount-1; row++ {
		for col := 1; col < MapColCount-1; col++ {
			testTile := gm.GameMap[row][col]
			if testTile.OwnerColor == player.Color {
				playerFinalClaimedLand += 1.0
				testTile.OwnerColor = nil
				testTile.IsTail = false
			}
		}
	}
	player.Location.IsTail = false
	player.Location.OwnerColor = nil
	// Use Delete to remove the player from the sync.Map
	gm.Players.Delete(*player.Color)
}

func (gm *GameManager) isWall(row int, col int) bool {
	if row <= 0 || col <= 0 {
		return true
	}

	if col >= MapColCount-1 || row >= MapRowCount-1 {
		return true
	}

	return false
}

func (gm *GameManager) isOtherPlayerTail(tile *Tile, playerColor *int) bool {
	return tile.IsTail && tile.OwnerColor != nil && playerColor != tile.OwnerColor
}

func (gm *GameManager) intializeBotControledPlayers(botCount int) {
	defaultStrategy := &DefaultStrategy{}
	for botId := 0; botId < botCount; botId++ {
		if _, ok := systemColors[botId]; ok {
			continue
		}

		botPlayer := CreateNewPlayer(nil, "derp"+strconv.Itoa(botId), botId, gm.getSpawnTile())
		botPlayer.BotStrategy = defaultStrategy
		gm.Players.Store(botId, botPlayer)
	}
}
