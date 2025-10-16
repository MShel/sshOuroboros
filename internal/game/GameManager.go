package game

type GameManager struct {
	// map int for color and pointer to player(null if color is anololocated)
	// we will init map with 256 colors for max players
	Players map[int]*Player
	GameMap [][]*Tile
}

var singletonGameManager *GameManager
var colorCount = 256
var mapColCount = 1024
var mapRowCount = 768

func GetNewGameManager() *GameManager {
	if singletonGameManager != nil {
		return singletonGameManager
	}

	singletonGameManager := &GameManager{}
	for i := 0; i < 256; i++ {
		singletonGameManager.Players[i] = nil
	}

	singletonGameManager.GameMap = make([][]*Tile, mapRowCount)

	for row := 0; row < mapRowCount; row++ {
		singletonGameManager.GameMap[row] = make([]*Tile, mapColCount)
		for col := 0; col < mapColCount; col++ {
			singletonGameManager.GameMap[row][col] = CreateNewTile()
		}
	}

	return singletonGameManager
}
