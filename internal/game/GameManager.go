package game

import (
	"context"
	"log"
	"math"
	"math/rand"
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
	PlayerColor        int
	FinalClaimedEstate float64
	FinalKills         int
}

// players estate is recalculated every time for all players once any of them claim any new estate
type ClaimedEstateMsg struct {
	PlayersEstate map[*int]int
}
type GameManager struct {
	Players           sync.Map
	SessionsToPlayers sync.Map
	GameMap           [][]*Tile

	SpaceFillerService *SpaceFiller

	DirectionChannel     chan Direction
	SunsetPlayersChannel chan *Player
	PlayerRebirth        chan int

	IsRunning     bool
	MapMutex      sync.RWMutex
	cancelContext context.CancelFunc
	GameContext   context.Context

	BotStrategyWg *sync.WaitGroup
}

var singletonGameManager *GameManager
var MapColCount = 700
var MapRowCount = 500

func GetNewGameManager() *GameManager {
	if singletonGameManager != nil {
		return singletonGameManager
	}

	gameContex, cancel := context.WithCancel(context.Background()) // Create cancellable context

	singletonGameManager = &GameManager{
		DirectionChannel:     make(chan Direction, 1),
		SunsetPlayersChannel: make(chan *Player, 1),
		PlayerRebirth:        make(chan int, 1),
		IsRunning:            false,
		cancelContext:        cancel,
		GameContext:          gameContex,
		BotStrategyWg:        &sync.WaitGroup{},
	}
	singletonGameManager.GameMap = getInitGameMap()
	singletonGameManager.SpaceFillerService = getNewSpaceFiller(singletonGameManager.GameMap)

	return singletonGameManager
}

func (gm *GameManager) broadcast(msg tea.Msg) {
	gm.Players.Range(func(key, value interface{}) bool {
		if player, ok := value.(*Player); ok && player != nil && player.BotStrategy == nil && !player.isDead {
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
	sunsetWorkersCount := 50
	for w := 1; w <= sunsetWorkersCount; w++ {
		go gm.sunsetPlayersWorker()
	}
	singletonGameManager.intializeBotControledPlayers(250)

	rebirthWorkerCount := 1
	for w := 1; w <= rebirthWorkerCount; w++ {
		go gm.rebirthPlayersWorker()
	}

	ticker := time.NewTicker(GameTickDuration)
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
	gm.BotStrategyWg.Wait()
	gm.SpaceFillerService.SpaceFillerWg.Wait()

	gm.Players.Range(func(key, value interface{}) bool {
		if player, ok := value.(*Player); ok && player != nil {
			if player == nil || player.isDead {
				return true
			}

			nextTile := player.GetNextTile()
			if IsWall(nextTile.Y, nextTile.X) {
				player.isDead = true
				gm.SunsetPlayersChannel <- player
				return true
			}

			player.isSafe = false

			if nextTile.OwnerColor != nil && nextTile.OwnerColor != player.Color {
				nextTileOwnerAny, _ := gm.Players.Load(*nextTile.OwnerColor)
				if nextTileOwnerAny == nil {
					return true
				}

				nextTileOwner := nextTileOwnerAny.(*Player)
				if nextTileOwner.isDead || nextTileOwner.isSafe {
					return true
				}

				if nextTileOwner.Location == nextTile {
					nextTileOwner.isDead = true
					player.isDead = true
					player.Kills += 1
					nextTileOwner.Kills += 1

					gm.SunsetPlayersChannel <- nextTileOwner
					gm.SunsetPlayersChannel <- player
					return true
				}

				if nextTile.IsTail {
					nextTileOwner.isDead = true
					gm.SunsetPlayersChannel <- nextTileOwner

					player.Kills += 1
					nextTile.OwnerColor = player.Color
					nextTile.IsTail = true
					player.Tail.tailLock.Lock()
					player.Tail.tailTiles = append(player.Tail.tailTiles, nextTile)
					player.Tail.tailLock.Unlock()

					player.Location = nextTile

					return true
				}

			}

			if nextTile.OwnerColor == player.Color && len(player.Tail.tailTiles) > 0 {
				select {
				case gm.SpaceFillerService.SpaceFillerChan <- player:
					// Successfully sent direction
				default:
					gm.SpaceFillerService = getNewSpaceFiller(gm.GameMap)
					log.Printf("space fill channel is full")
				}

				player.Location = nextTile
				player.isSafe = true
				return true
			}

			if nextTile.OwnerColor != player.Color {
				nextTile.OwnerColor = player.Color
				nextTile.IsTail = true
				nextTile.Direction = player.CurrentDirection
				player.Tail.tailLock.Lock()
				player.Tail.tailTiles = append(player.Tail.tailTiles, nextTile)
				player.Tail.tailLock.Unlock()
			}

			player.Location = nextTile

			if player.BotStrategy != nil {
				gm.BotStrategyWg.Add(1)
				go func() {
					defer gm.BotStrategyWg.Done()
					nextDirection := player.BotStrategy.getNextBestDirection(player, gm)
					player.CurrentDirection = nextDirection
				}()
			}
		}
		return true
	})
}

func (gm *GameManager) CreateNewPlayer(playerName string, playerColor int, userSession ssh.Session) *Player {
	spawnTile := gm.getSpawnTile()
	newPlayer := CreateNewPlayer(userSession, playerName, playerColor, spawnTile)
	if player, ok := gm.Players.Load(playerColor); ok {
		gm.sunsetPlayer(player.(*Player), false)
	}

	gm.Players.Store(playerColor, newPlayer)
	gm.SessionsToPlayers.Store(userSession, newPlayer)

	return newPlayer
}

func (gm *GameManager) getSpawnTile() *Tile {
	const (
		safeMargin     = 10
		sampleAttempts = 300
	)

	bestTile := gm.GameMap[MapRowCount/2][MapColCount/2]
	maxMinDist := -1

	for range sampleAttempts {
		row := rand.Intn(MapRowCount-2*safeMargin) + safeMargin
		col := rand.Intn(MapColCount-2*safeMargin) + safeMargin
		tile := gm.GameMap[row][col]

		// Skip occupied or tail tiles
		if tile.OwnerColor != nil || tile.IsTail {
			continue
		}

		minDist := math.MaxInt32

		// Directly iterate players — no slice needed
		gm.Players.Range(func(_, value interface{}) bool {
			p, ok := value.(*Player)
			if !ok || p == nil {
				return true
			}

			d := GetManhattanDistance(tile, p.Location)
			if d < minDist {
				minDist = d
				// Early stop if worse than current best
				if minDist < maxMinDist {
					return false
				}
			}
			return true
		})

		if minDist > maxMinDist {
			maxMinDist = minDist
			bestTile = tile
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
			gm.sunsetPlayer(player, true)
		}
	}
}

func (gm *GameManager) sunsetPlayer(player *Player, needRebirth bool) {
	playerFinalClaimedLand := 0.0
	player.AllTiles.allTilesLock.Lock()
	player.Tail.tailLock.Lock()
	defer player.AllTiles.allTilesLock.Unlock()
	defer player.Tail.tailLock.Unlock()

	for _, tile := range player.AllTiles.AllPlayerTiles {
		if tile.OwnerColor == player.Color {
			playerFinalClaimedLand += 1.0
			tile.OwnerColor = nil
		}
	}
	for _, tile := range player.Tail.tailTiles {
		if tile.OwnerColor == player.Color {
			tile.OwnerColor = nil
		}
	}

	player.Location.IsTail = false
	player.Location.OwnerColor = nil

	if player.SshSession != nil {
		player.UpdateChannel <- PlayerDeadMsg{
			PlayerColor:        *player.Color,
			FinalClaimedEstate: playerFinalClaimedLand,
			FinalKills:         player.Kills,
		}
	}
	gm.Players.Delete(*player.Color)

	if needRebirth {
		gm.PlayerRebirth <- *player.Color
	}

}

func (gm *GameManager) rebirthPlayersWorker() {
	for {
		playerColorInt, ok := <-gm.PlayerRebirth
		if !ok {
			return
		}
		if playerColorInt != 0 {
			botPlayer := CreateNewPlayer(nil, "derp"+strconv.Itoa(playerColorInt), playerColorInt, gm.getSpawnTile())
			botPlayer.BotStrategy = defaultStrategy
			gm.Players.Store(playerColorInt, botPlayer)
		}
	}
}

func (gm *GameManager) isOtherPlayerTail(tile *Tile, playerColor *int) bool {
	return tile.IsTail && tile.OwnerColor != nil && playerColor != tile.OwnerColor
}

var defaultStrategy = &DefaultStrategy{}

func (gm *GameManager) intializeBotControledPlayers(botCount int) {
	for botId := 0; botId < botCount; botId++ {
		if _, ok := SystemColors[botId]; ok {
			continue
		}

		botPlayer := CreateNewPlayer(nil, "derp"+strconv.Itoa(botId), botId, gm.getSpawnTile())
		botPlayer.BotStrategy = defaultStrategy
		gm.Players.Store(botId, botPlayer)
	}
}

func (gm *GameManager) GetMapCopy(startRow, endRow, startCol, endCol int) [][]Tile {
	gm.MapMutex.RLock()
	defer gm.MapMutex.RUnlock()

	startRow = max(0, startRow)
	endRow = min(MapRowCount, endRow)
	startCol = max(0, startCol)
	endCol = min(MapColCount, endCol)

	rows := endRow - startRow
	if rows <= 0 {
		return nil
	}
	cols := endCol - startCol
	if cols <= 0 {
		return nil
	}

	mapCopy := make([][]Tile, rows)

	for i := 0; i < rows; i++ {
		mapCopy[i] = make([]Tile, cols)
		for j := 0; j < cols; j++ {
			mapCopy[i][j] = *gm.GameMap[startRow+i][startCol+j]
		}
	}

	return mapCopy
}
