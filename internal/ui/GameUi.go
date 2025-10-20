package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Mshel/sshnake/internal/game"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type GameModel struct {
	tea.Model
	TickCount   int
	gameManager *game.GameManager
}

func (m GameModel) Init() tea.Cmd {
	// Start the command that listens for game updates
	// The external game loop will now drive the UI update rate.
	return m.listenForGameUpdates()
}

func (gameModel GameModel) listenForGameUpdates() tea.Cmd {
	return func() tea.Msg {
		// This line blocks until gm.UpdateChannel receives a message from the game loop
		return <-gameModel.gameManager.UpdateChannel
	}
}

// --- GameScreenModel Update Method (Focused on Input) ---
func (m GameModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		var engineCommand game.Direction

		// Elegantly map Arrow Keys and WASD to a single direction command
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k", "w":
			engineCommand = game.Direction{Dx: 0, Dy: -1, PlayerColor: m.gameManager.CurrentPlayerColor}
		case "down", "j", "s":
			engineCommand = game.Direction{Dx: 0, Dy: 1, PlayerColor: m.gameManager.CurrentPlayerColor}
		case "left", "h", "a":
			engineCommand = game.Direction{Dx: -1, Dy: 0, PlayerColor: m.gameManager.CurrentPlayerColor}
		case "right", "l", "d":
			engineCommand = game.Direction{Dx: 1, Dy: 0, PlayerColor: m.gameManager.CurrentPlayerColor}
		// will add more commands for diagonal movements
		default:
			// Ignore all other key presses
			return m, nil
		}
		// If a command was determined, wrap it in a tea.Cmd to send to the engine
		if (engineCommand != game.Direction{}) {
			// Execute the network/engine command
			m.gameManager.DirectionChannel <- engineCommand
		}
	case game.GameTickMsg:
		m.TickCount++
		return m, m.listenForGameUpdates()
	case game.PlayerDeadMsg:
		return m, tea.Quit
	}

	return m, cmd
}

func (m GameModel) View() string {
	var mapView strings.Builder

	mapView.WriteString(time.Now().String() + fmt.Sprintf(" | Tick: %d\n", m.TickCount))

	gameMap := m.gameManager.GameMap

	const viewportSize = 100

	player := m.gameManager.Players[m.gameManager.CurrentPlayerColor]

	startRow, startCol := 0, 0
	if player != nil && player.Location != nil {
		startRow = player.Location.Y - viewportSize/2
		startCol = player.Location.X - viewportSize/2
	}

	if startRow < 0 {
		startRow = 0
	}
	if startCol < 0 {
		startCol = 0
	}

	endRow := startRow + viewportSize
	endCol := startCol + viewportSize

	if endRow > game.MapRowCount {
		endRow = game.MapRowCount
	}
	if endCol > game.MapColCount {
		endCol = game.MapColCount
	}

	if endRow > game.MapRowCount {
		endRow = game.MapRowCount
	}
	if endCol > game.MapColCount {
		endCol = game.MapColCount
	}

	for row := startRow; row < endRow; row++ {
		for col := startCol; col < endCol; col++ {
			currTile := gameMap[row][col]

			if currTile.OwnerColor != nil && (currTile.IsTail || player.Location == currTile) {
				mapView.WriteString(
					lipgloss.
						NewStyle().
						Width(1).
						Foreground(lipgloss.Color(strconv.Itoa(*currTile.OwnerColor))).Render("○"))
				continue
			}
			if currTile.OwnerColor != nil && !currTile.IsTail {
				mapView.WriteString(
					lipgloss.
						NewStyle().
						Width(1).
						Foreground(lipgloss.Color(strconv.Itoa(*currTile.OwnerColor))).Render("░"))
				continue
			}
			// Render empty space (faint gray)
			mapView.WriteString(lipgloss.NewStyle().Width(1).Foreground(lipgloss.Color("235")).Render("░"))
		}
		mapView.WriteString("\n")
	}

	return mapView.String()
}

func NewGameModel(gameManager *game.GameManager) GameModel {
	gameModel := GameModel{
		TickCount:   0,
		gameManager: gameManager,
	}
	return gameModel
}
