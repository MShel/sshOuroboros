package game

import (
	"context"
	"log"
	"math"
	"sync" // sync.Map is included here
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

// players estate is recalculated every time for all players once any of them claim any new estate
type ClaimedEstateMsg struct {
	PlayersEstate map[*int]int
}

type GameManager struct {
	// Players is now a sync.Map for concurrent access, mapping int (color) to *Player.
	// This replaces the map[int]*Player and the PlayersMutex.
	Players              sync.Map
	GameMap              [][]*Tile
	DirectionChannel     chan Direction
	SunsetPlayersChannel chan *Player
	UpdateChannel        chan tea.Msg

	CurrentPlayerColor int
	IsRunning          bool
	MapMutex           sync.RWMutex
	cancelContext      context.CancelFunc
	GameContext        context.Context
}

var singletonGameManager *GameManager
var MapColCount = 200
var MapRowCount = 50

func GetNewGameManager() *GameManager {
	if singletonGameManager != nil {
		return singletonGameManager
	}

	gameContex, cancel := context.WithCancel(context.Background()) // Create cancellable context

	singletonGameManager = &GameManager{
		DirectionChannel:     make(chan Direction, 256),
		UpdateChannel:        make(chan tea.Msg, 256),
		SunsetPlayersChannel: make(chan *Player, 256),
		IsRunning:            false,
		cancelContext:        cancel, // Store the cancel function
		GameContext:          gameContex,
		// Players is initialized to an empty sync.Map implicitly
	}

	// The old loop to pre-allocate 256 colors is removed, as sync.Map is dynamically populated.
	// Players are now added directly in CreateNewPlayer.

	singletonGameManager.GameMap = getInitGameMap()

	return singletonGameManager
}

func (gm *GameManager) StartGameLoop() {
	if gm.IsRunning {
		return
	}
	gm.IsRunning = true
	duration := 100 * time.Millisecond
	sunsetWorkersCount := 40
	for w := 1; w <= sunsetWorkersCount; w++ {
		go gm.sunsetPlayersWorker()
	}

	ticker := time.NewTicker(duration)
	defer ticker.Stop()

	for gm.IsRunning {
		select {
		case <-ticker.C:
			gm.processGameTick()
			gm.UpdateChannel <- GameTickMsg{}
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
	log.Println("Game Manager shutdown initiated.")
}

// processPlayerInput immediately updates the direction of the specified player.
func (gm *GameManager) processPlayerInput(dir Direction) {
	// Replaced gm.Players[dir.PlayerColor] with gm.Players.Load()
	if p, ok := gm.Players.Load(dir.PlayerColor); ok {
		// Assert the type from interface{} to *Player
		if player, ok := p.(*Player); ok && player != nil {
			player.UpdateDirection(dir)
		}
	}
}

// processGameTick is called every GameTickDuration to move all players and check collisions.
func (gm *GameManager) processGameTick() {

	// Replaced manual locking and iteration with sync.Map.Range
	activePlayers := make([]*Player, 0, 10)
	gm.Players.Range(func(key, value interface{}) bool {
		if player, ok := value.(*Player); ok && player != nil {
			activePlayers = append(activePlayers, player)
		}
		return true // continue iteration
	})

	for _, player := range activePlayers {
		if player == nil {
			continue
		}

		nextTile := player.GetNextTile()
		if gm.isWall(nextTile.Y, nextTile.X) {
			gm.SunsetPlayersChannel <- player

			gm.UpdateChannel <- PlayerDeadMsg{
				*player.Color,
			}
			continue
		}

		if gm.isOtherPlayerTail(nextTile, player.Color) {

			var otherPlayer *Player
			// Replaced manual locking and map access with sync.Map.Load
			if nextTile.OwnerColor != nil {
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
				continue
			}
		}

		if nextTile.OwnerColor == player.Color && len(player.Tail) > 0 {
			gm.spaceFill(player)
			player.resetTailData()
			// fire and forget, data might be slightly stale but its fiiine in this case
			// go func() {
			// 	claimedEstateMsg := ClaimedEstateMsg{
			// 		PlayersEstate: gm.getUpdatedPlayerEstate(),
			// 	}
			// 	gm.UpdateChannel <- claimedEstateMsg
			// }()
			player.Location = nextTile
			continue
		}

		if nextTile.OwnerColor != player.Color {
			nextTile.OwnerColor = player.Color
			nextTile.IsTail = true
			player.Tail = append(player.Tail, nextTile)
		}

		// Update player's location
		player.Location = nextTile
	}

	// TODO: Implement broadcasting the updated GameMap state to all players/UI
}

func (gm *GameManager) getTilesToBeFilled(seed *Tile,
	playerColor *int,
	searchContext context.Context,
	resultsChan chan map[*Tile]interface{},
	wg *sync.WaitGroup) {
	defer wg.Done()

	q := []*Tile{
		seed,
	}
	mapOfTilesToIgnore := make(map[*Tile]interface{})

	for len(q) > 0 {
		select {
		case <-searchContext.Done():
			return
		default:
		}

		testCoord := q[0]
		q = q[1:]

		testTile := gm.GameMap[testCoord.Y][testCoord.X]
		mapOfTilesToIgnore[testTile] = true

		for _, dir := range directions {
			testRow, testCol := testTile.Y+dir[0], testTile.X+dir[1]
			if gm.isWall(testRow, testCol) {
				return
			}

			testTile := gm.GameMap[testRow][testCol]
			if _, ok := mapOfTilesToIgnore[testTile]; ok {
				continue
			}

			if testTile.OwnerColor == playerColor {
				mapOfTilesToIgnore[testTile] = true
				continue
			}

			q = append(q, testTile)
			mapOfTilesToIgnore[testTile] = true
		}
	}

	select {
	case <-searchContext.Done():
		return
	case resultsChan <- mapOfTilesToIgnore:
	}
}

type TileFillerInfo struct {
	TestTile       *Tile
	Player         *Player
	SearchContext  context.Context
	ResultsChannel chan map[*Tile]interface{}
	WaitG          *sync.WaitGroup
}

func (gm *GameManager) tileFillerWorker(fillerChannel chan TileFillerInfo) {
	for {
		tileFillerInfo, ok := <-fillerChannel
		if !ok { // Channel was closed
			return
		}

		gm.getTilesToBeFilled(
			tileFillerInfo.TestTile,
			tileFillerInfo.Player.Color,
			tileFillerInfo.SearchContext,
			tileFillerInfo.ResultsChannel,
			tileFillerInfo.WaitG)
	}
}

func (gm *GameManager) spaceFill(player *Player) {
	gm.MapMutex.Lock()
	defer gm.MapMutex.Unlock()

	resultsChannel := make(chan map[*Tile]interface{}, 1)
	searchContext, cancelSearch := context.WithCancel(context.Background())
	fillerChannel := make(chan TileFillerInfo, 50)
	TilesFillerWorkers := 3
	for w := 1; w <= TilesFillerWorkers; w++ {
		go gm.tileFillerWorker(fillerChannel)
	}

	var wg sync.WaitGroup
	vizited := make(map[*Tile]bool)

	for i, tailTile := range player.Tail {
		if i%2 != 0 {
			continue
		}
		for _, dir := range directions {
			testRow, testCol := tailTile.Y+dir[0], tailTile.X+dir[1]
			testTile := gm.GameMap[testRow][testCol]

			if _, ok := vizited[testTile]; ok || (testTile.IsTail && testTile.OwnerColor == player.Color) {
				continue
			}

			vizited[testTile] = true
			if testTile.OwnerColor != player.Color {
				wg.Add(1)
				fillerChannel <- TileFillerInfo{
					TestTile:       testTile,
					Player:         player,
					SearchContext:  searchContext,
					ResultsChannel: resultsChannel,
					WaitG:          &wg,
				}
			}
		}
	}

	go func() {
		wg.Wait()
		close(resultsChannel)
		close(fillerChannel)
	}()

	for mapOfTiles := range resultsChannel {
		if len(mapOfTiles) > 0 {
			cancelSearch()
			for tile := range mapOfTiles {
				gm.GameMap[tile.Y][tile.X].OwnerColor = player.Color
				gm.GameMap[tile.Y][tile.X].IsTail = false
			}
			break
		}
	}

	for _, tile := range player.Tail {
		tile.IsTail = false
	}

	cancelSearch()
}

func (gm *GameManager) getUpdatedPlayerEstate() map[*int]int {
	gm.MapMutex.RLock()
	defer gm.MapMutex.RUnlock()

	playersEstate := make(map[*int]int)

	// Rewritten loop to use sync.Map.Range for concurrent safety
	gm.Players.Range(func(key, value interface{}) bool {
		if player, ok := value.(*Player); ok && player != nil {
			playersEstate[player.Color] = 0
		}
		return true
	})

	for row := 1; row < MapRowCount-1; row++ {
		for col := 1; col < MapColCount-1; col++ {
			tile := gm.GameMap[row][col]
			if tile.IsTail {
				continue
			}
			if tile.OwnerColor == nil {
				continue
			}

			playersEstate[tile.OwnerColor] += 1
		}
	}

	return playersEstate
}

func (gm *GameManager) CreateNewPlayer(playerName string, playerColor int) *Player {
	spawnTile := gm.getSpawnTile()
	newPlayer := CreateNewPlayer(playerName, playerColor, spawnTile)
	// Use Store to add the new player
	gm.Players.Store(playerColor, newPlayer)
	gm.CurrentPlayerColor = playerColor

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

// NOTE: Assumed Tile, Player, directions, getInitGameMap, getManhattanDistance, and CreateNewPlayer
// are defined elsewhere in the 'game' package and are available.
