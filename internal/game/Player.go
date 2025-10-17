package game

type Player struct {
	Name             string
	Color            *int
	ClaimedEstate    float32
	Location         *Tile
	CurrentDirection Direction
}

func CreateNewPlayer(name string, color int) *Player {
	spawnPoint := GetSpawnPoint()
	spawnPoint.OwnerColor = &color
	return &Player{
		Name:             name,
		Color:            &color,
		Location:         spawnPoint,
		CurrentDirection: Direction{Dx: 1, Dy: 0},
	}
}

func (p *Player) GetNextTile() *Tile {
	nextX := p.Location.X + p.CurrentDirection.Dx
	nextY := p.Location.Y + p.CurrentDirection.Dy

	// Handle map wrapping or boundaries if necessary (assuming simple wrapping for now)
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

	// Safety check for map access
	if nextY < MapRowCount && nextX < MapColCount {
		return getInitGameMap()[nextY][nextX]
	}
	return nil // Should not happen with wrapping/boundary logic
}

func (p *Player) UpdateDirection(newDir Direction) {
	// Basic Ssshnake rule: cannot reverse direction (e.g., if moving right (Dx:1), cannot move left (Dx:-1))
	// We check if the sum of components is zero.
	if newDir.Dx+p.CurrentDirection.Dx == 0 && newDir.Dy+p.CurrentDirection.Dy == 0 {
		return // Ignore reversal commands
	}
	p.CurrentDirection = newDir
}
