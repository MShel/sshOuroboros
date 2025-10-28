package game

import (
	"math/rand"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
)

type Player struct {
	Name          string
	SshSession    ssh.Session
	Color         *int
	ClaimedEstate int
	Location      *Tile
	Tail          []*Tile
	// that might contain tiles that are not theirs anymore TODO use that instead of scanning the map
	AllPlayerTiles   []*Tile
	CurrentDirection Direction
	UpdateChannel    chan tea.Msg
	BotStrategy      Strategy
	Kills            int
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
		Tail: []*Tile{
			spawnPoint,
		},
		UpdateChannel: make(chan tea.Msg, 16),
		Kills:         0,
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
	p.Tail = []*Tile{}
}

func (p *Player) UpdateDirection(newDir Direction) {
	if newDir.Dx+p.CurrentDirection.Dx == 0 && newDir.Dy+p.CurrentDirection.Dy == 0 {
		return
	}
	p.CurrentDirection = newDir
}
