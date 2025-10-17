package game

type Tile struct {
	OwnerColor *int
	IsTail     bool
	X          int
	Y          int
}

func CreateNewTile(row int, col int) *Tile {
	return &Tile{
		X:          col,
		Y:          row,
		OwnerColor: nil,
		IsTail:     false,
	}
}

func GetSpawnPoint() *Tile {
	return getInitGameMap()[10][10]
}

var GameMap [][]*Tile

// that will be covered under viewport so we dont worry about displaying the whole thing and can make it aribtrary large
var MapColCount = 200
var MapRowCount = 200

func getInitGameMap() [][]*Tile {
	if GameMap != nil {
		return GameMap
	}

	GameMap = make([][]*Tile, mapRowCount)

	for row := 0; row < mapRowCount; row++ {
		GameMap[row] = make([]*Tile, mapColCount)
		for col := 0; col < mapColCount; col++ {
			GameMap[row][col] = CreateNewTile(row, col)
		}
	}

	return GameMap
}
