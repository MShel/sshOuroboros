package game

type Tile struct {
	Owner  *Player
	IsTail bool
}

func CreateNewTile() *Tile {
	return &Tile{}
}
