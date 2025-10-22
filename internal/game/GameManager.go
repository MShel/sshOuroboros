package game

import (
	"context"
	"log"
	"math"
	"sync"
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
		if gm.isWall(nextTile.Y, nextTile.X) {
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

		if nextTile.OwnerColor == player.Color && len(player.Tail) > 0 {
			gm.spaceFill(player)
			player.resetTailData()
			// fire and forget, data might be slightly stale but its fiiine in this case
			go func() {
				claimedEstateMsg := ClaimedEstateMsg{
					PlayersEstate: gm.getUpdatedPlayerEstate(),
				}
				gm.UpdateChannel <- claimedEstateMsg
			}()
			player.Location = nextTile
			return
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
			if len(q) == 0 {
				resultsChan <- mapOfTilesToIgnore
			}
		}
	}
}

func (gm *GameManager) spaceFill(player *Player) {
	resultsChannel := make(chan map[*Tile]interface{})
	searchContext, cancelSearch := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	for _, dir := range directions {
		for _, tailTile := range player.Tail {
			testRow, testCol := tailTile.Y+dir[0], tailTile.X+dir[1]
			testTile := gm.GameMap[testRow][testCol]
			if testTile.OwnerColor != player.Color {
				wg.Add(1)
				go gm.getTilesToBeFilled(testTile, player.Color, searchContext, resultsChannel, &wg)
			}
		}
	}

	go func() {
		wg.Wait()
		close(resultsChannel)
	}()

	for mapOfTiles := range resultsChannel {
		if len(mapOfTiles) > 0 {
			cancelSearch()

			for tile := range mapOfTiles {
				gm.GameMap[tile.Y][tile.X].OwnerColor = player.Color
				gm.GameMap[tile.Y][tile.X].IsTail = false
			}
		}
	}

	for _, tile := range player.Tail {
		tile.IsTail = false
	}

	cancelSearch()
}

func (gm *GameManager) getUpdatedPlayerEstate() map[*int]int {
	playersEstate := make(map[*int]int)
	for _, player := range gm.Players {
		if player != nil {
			playersEstate[player.Color] = 0
		}
	}

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
	gm.Players[playerColor] = CreateNewPlayer(playerName, playerColor, spawnTile)
	gm.CurrentPlayerColor = playerColor

	return gm.Players[playerColor]
}

func (gm *GameManager) getSpawnTile() *Tile {
	var activePlayers []*Player
	var sumX, sumY int

	for _, player := range gm.Players {
		if player != nil {
			activePlayers = append(activePlayers, player)
			sumX += player.Location.X
			sumY += player.Location.Y
		}
	}

	// If there are no active players, pick the center of the safe play area.
	if len(activePlayers) == 0 {
		return gm.GameMap[MapRowCount/2][MapColCount/2]
	}

	// the center of all of the players
	centerAvgX := float64(sumX) / float64(len(activePlayers))
	centerAvgY := float64(sumY) / float64(len(activePlayers))

	mapCenterX := float64(MapColCount-1) / 2.0
	mapCenterY := float64(MapRowCount-1) / 2.0

	targetIdealX := 2*mapCenterX - centerAvgX
	targetIdealY := 2*mapCenterY - centerAvgY

	targetX := int(math.Max(1, math.Min(float64(MapColCount-2), targetIdealX)))
	targetY := int(math.Max(1, math.Min(float64(MapRowCount-2), targetIdealY)))

	const minSafeDistance = 5
	searchRadius := 0 // Start search at the ideal point itself
	maxRadius := int(math.Max(float64(MapRowCount), float64(MapColCount)))

	// Loop to expand the search square layer by layer (O(k) where k is small)
	for searchRadius <= maxRadius {
		// Define the bounds of the current search square layer
		minR := targetY - searchRadius
		maxR := targetY + searchRadius
		minC := targetX - searchRadius
		maxC := targetX + searchRadius

		// Search the perimeter of the square defined by the radius
		for r := minR; r <= maxR; r++ {
			for c := minC; c <= maxC; c++ {
				// check if we already expored this perimeter
				isPerimeter := (searchRadius == 0 || r == minR || r == maxR || c == minC || c == maxC)
				if !isPerimeter {
					continue
				}

				if gm.isWall(r, c) {
					continue
				}

				currentTile := gm.GameMap[r][c]

				// Check constraints: must be unclaimed and not a tail
				if currentTile.OwnerColor != nil || currentTile.IsTail {
					continue
				}

				// Check safety distance: must be far from all players
				isSafe := true
				for _, player := range activePlayers {
					dist := getManhattanDistance(currentTile, player.Location)
					if dist < minSafeDistance {
						isSafe = false
						break
					}
				}

				if isSafe {
					return currentTile
				}
			}
		}

		searchRadius++
	}

	return nil
}

func (gm *GameManager) isWall(row int, col int) bool {
	if row == 0 || col == 0 {
		return true
	}

	if col == MapColCount-1 || row == MapRowCount-1 {
		return true
	}

	return false
}

func (gm *GameManager) isOtherPlayerTail(tile *Tile, playerColor *int) bool {
	return tile.IsTail && tile.OwnerColor != nil && playerColor != tile.OwnerColor
}
