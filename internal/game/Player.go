package game

type Player struct {
	Name                                           string
	Color                                          *int
	ClaimedEstate                                  int
	Location                                       *Tile
	Tail                                           []*Tile
	MaxTailRow, MinTailRow, MaxTailCol, MinTailCol int
	CurrentDirection                               Direction
}

func CreateNewPlayer(name string, color int, spawnPoint *Tile) *Player {
	spawnPoint.OwnerColor = &color
	spawnPoint.IsTail = true

	return &Player{
		Name:             name,
		Color:            &color,
		Location:         spawnPoint,
		CurrentDirection: Direction{Dx: 1, Dy: 0},
		Tail: []*Tile{
			spawnPoint,
		},
	}
}

func (p *Player) GetNextTile() *Tile {
	nextX := p.Location.X + p.CurrentDirection.Dx
	nextY := p.Location.Y + p.CurrentDirection.Dy

	if nextX < 0 {
		nextX = MapColCount - 1
	} else if nextX >= MapColCount {
		nextX = 0
	}
	if nextY < 0 {
		nextY = MapRowCount - 1
	} else if nextY >= MapRowCount {
		nextY = 0
	}

	if nextY < MapRowCount && nextX < MapColCount {
		return getInitGameMap()[nextY][nextX]
	}

	return nil
}

func (p *Player) resetTailData() {
	p.MaxTailRow = 0
	p.MinTailRow = MapRowCount
	p.MaxTailCol = 0
	p.MinTailCol = MapColCount
	p.Tail = []*Tile{}
}

func (p *Player) UpdateDirection(newDir Direction) {
	if newDir.Dx+p.CurrentDirection.Dx == 0 && newDir.Dy+p.CurrentDirection.Dy == 0 {
		return
	}
	p.CurrentDirection = newDir
}
