package game

type Tile struct {
	OwnerColor *int      `json:"OwnerColor"`
	IsHead     bool      `json:"isHead"`
	IsTail     bool      `json:"isTail"`
	X          int       `json:"x"`
	Y          int       `json:"y"`
	Direction  Direction `json:"direction"`
}

func CreateNewTile(row int, col int) *Tile {
	return &Tile{
		X:          col,
		Y:          row,
		OwnerColor: nil,
		IsTail:     false,
		Direction:  Direction{},
	}
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
