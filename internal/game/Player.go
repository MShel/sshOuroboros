package game

type Player struct {
	Name                                           string
	Color                                          *int
	ClaimedEstate                                  float32
	Location                                       *Tile
	Tail                                           []*Tile
	MaxTailRow, MinTailRow, MaxTailCol, MinTailCol int
	CurrentDirection                               Direction
}

func CreateNewPlayer(name string, color int) *Player {
	spawnPoint := GetSpawnPoint()
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
		MaxTailRow: 0,
		MinTailRow: MapRowCount,
		MaxTailCol: 0,
		MinTailCol: MapColCount,
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

func (p *Player) addTileToTail(tile *Tile) {
	p.MaxTailCol = max(p.MaxTailCol, tile.X)
	p.MinTailCol = min(p.MinTailCol, tile.X)

	p.MaxTailRow = max(p.MaxTailRow, tile.Y)
	p.MinTailRow = min(p.MinTailRow, tile.Y)

	p.Tail = append(p.Tail, tile)
}

func (p *Player) UpdateDirection(newDir Direction) {
	// Basic Ssshnake rule: cannot reverse direction (e.g., if moving right (Dx:1), cannot move left (Dx:-1))
	// We check if the sum of components is zero.
	if newDir.Dx+p.CurrentDirection.Dx == 0 && newDir.Dy+p.CurrentDirection.Dy == 0 {
		return // Ignore reversal commands
	}
	p.CurrentDirection = newDir
}
