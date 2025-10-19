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

func getInitGameMap() [][]*Tile {
	if GameMap != nil {
		return GameMap
	}

	GameMap = make([][]*Tile, MapRowCount)

	for row := 0; row < MapRowCount; row++ {
		GameMap[row] = make([]*Tile, MapColCount)
		for col := 0; col < MapColCount; col++ {
			GameMap[row][col] = CreateNewTile(row, col)
		}
	}

	return GameMap
}
