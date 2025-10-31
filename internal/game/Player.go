package game

import (
	"math/rand"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
)

type Tail struct {
	tailLock  sync.Mutex
	tailTiles []*Tile
}

type AllTiles struct {
	allTilesLock   sync.Mutex
	AllPlayerTiles []*Tile
}

type Player struct {
	Name             string
	SshSession       ssh.Session
	Color            *int
	ClaimedEstate    int
	Location         *Tile
	CurrentDirection Direction
	UpdateChannel    chan tea.Msg
	BotStrategy      Strategy
	Kills            int
	isDead           bool
	isSafe           bool
	Tail             Tail
	AllTiles         AllTiles
}

func CreateNewPlayer(sshSession ssh.Session, name string, color int, spawnPoint *Tile) *Player {
	spawnPoint.OwnerColor = &color
	spawnPoint.IsTail = true
	possibleDirections := []Direction{
		{Dx: 1, Dy: 0},
		{Dx: 0, Dy: 1},
		{Dx: -1, Dy: 0},
		{Dx: 0, Dy: -1},
	}

	return &Player{
		Name:             name,
		Color:            &color,
		SshSession:       sshSession,
		Location:         spawnPoint,
		CurrentDirection: possibleDirections[rand.Intn(len(possibleDirections))],
		Tail: Tail{
			tailTiles: []*Tile{
				spawnPoint,
			}},
		UpdateChannel: make(chan tea.Msg, 256),
		Kills:         0,
		isDead:        false,
		isSafe:        false,
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
	p.Tail.tailLock.Lock()
	defer p.Tail.tailLock.Unlock()

	p.Tail.tailTiles = []*Tile{}
}

func (p *Player) UpdateDirection(newDir Direction) {
	if newDir.Dx+p.CurrentDirection.Dx == 0 && newDir.Dy+p.CurrentDirection.Dy == 0 {
		return
	}
	p.CurrentDirection = newDir
}

func (p *Player) GetConsolidateTiles() float64 {
	p.AllTiles.allTilesLock.Lock()
	defer p.AllTiles.allTilesLock.Unlock()
	updatedTiles := []*Tile{}
	claimedLand := 0.0
	for _, tile := range p.AllTiles.AllPlayerTiles {
		if tile.OwnerColor == p.Color {
			claimedLand += 1.0
			updatedTiles = append(updatedTiles, tile)
		}
	}

	p.AllTiles.AllPlayerTiles = updatedTiles
	return claimedLand
}
