package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Mshel/sshnake/internal/game"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
)

// --- Styling Definitions ---

var (
	// Base style for the game map border
	mapViewStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 0) // No internal padding for the map itself

	// Base style for the status panel border
	statusPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("8")). // Light gray
				Padding(1, 2)                          // Internal padding

	// Styles for map elements
	wallStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("237")) // Darkest gray for walls
	voidStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("235")) // Dark gray for empty space

	// Player head runes based on direction
	headRunes = map[game.Direction]string{
		{Dx: 0, Dy: -1}: "▲", // Up
		{Dx: 0, Dy: 1}:  "▼", // Down
		{Dx: -1, Dy: 0}: "◀", // Left
		{Dx: 1, Dy: 0}:  "▶", // Right
	}
)

// Define the percentage of the screen to use for the main map viewport
// Adjusted to make room for a side panel
const (
	mapViewPercentage  = 0.70 // Map takes 70% of the width
	statusPanelPadding = 4    // Padding/Border width for the status panel and a space in between
)

// GameModel holds the state for the TUI rendering for a single SSH session.
type GameModel struct {
	tea.Model
	TickCount    int
	EstateInfo   map[*int]int
	ScreenWidth  int
	ScreenHeight int
	gameManager  *game.GameManager
	UserSession  ssh.Session
}

// NewGameModel is the constructor for GameModel.
func NewGameModel(gm *game.GameManager, session ssh.Session, screenWidth int, screenHeight int) GameModel {
	return GameModel{
		gameManager:  gm,
		UserSession:  session,
		TickCount:    0,
		EstateInfo:   make(map[*int]int),
		ScreenWidth:  screenWidth,
		ScreenHeight: screenHeight,
	}
}

func (m GameModel) Init() tea.Cmd {
	return m.listenForGameUpdates()
}

func (gameModel GameModel) listenForGameUpdates() tea.Cmd {
	return func() tea.Msg {
		// 1. Get the current player for this session
		playerVal, ok := gameModel.gameManager.SessionsToPlayers.Load(gameModel.UserSession)
		if !ok {
			log.Error("Session not mapped to player, quitting TUI.", "session",
				gameModel.UserSession.Context().SessionID())
			return tea.Quit()
		}

		currentPlayer, ok := playerVal.(*game.Player)
		if !ok || currentPlayer.UpdateChannel == nil {
			log.Error("Player object invalid or update channel is nil, quitting TUI.",
				"session",
				gameModel.UserSession.Context().SessionID())
			return tea.Quit()
		}

		return <-currentPlayer.UpdateChannel
	}
}

// --- GameScreenModel Update Method (Unchanged) ---
func (m GameModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

	// Retrieve the current player for this session
	currentPlayerVal, ok := m.gameManager.SessionsToPlayers.Load(m.UserSession)
	if !ok {
		// This should not happen if Init succeeded.
		log.Error("User session lost during update, quitting.")
		return m, tea.Quit
	}

	currentPlayer, _ := currentPlayerVal.(*game.Player)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		var engineCommand game.Direction

		// Elegantly map Arrow Keys or WASD to a direction command
		switch msg.String() {
		case "up", "w":
			engineCommand = game.Direction{Dx: 0, Dy: -1, PlayerColor: *currentPlayer.Color}
		case "down", "s":
			engineCommand = game.Direction{Dx: 0, Dy: 1, PlayerColor: *currentPlayer.Color}
		case "left", "a":
			engineCommand = game.Direction{Dx: -1, Dy: 0, PlayerColor: *currentPlayer.Color}
		case "right", "d":
			engineCommand = game.Direction{Dx: 1, Dy: 0, PlayerColor: *currentPlayer.Color}
		case "q", "ctrl+c":
			// Send a quit message to the game manager for sunsetting logic
			m.gameManager.SunsetPlayersChannel <- currentPlayer
			return m, tea.Quit
		default:
			return m, nil
		}

		// Send the direction command to the game engine
		m.gameManager.DirectionChannel <- engineCommand

	case game.GameTickMsg:
		// A message from the game loop indicating state update
		m.TickCount++
		return m, m.listenForGameUpdates()

	case game.ClaimedEstateMsg:
		// Update the local map of claimed estate information
		m.EstateInfo = msg.PlayersEstate
		return m, m.listenForGameUpdates()

	case game.PlayerDeadMsg:
		// Check if the current player is the one that died
		playerVal, ok := m.gameManager.SessionsToPlayers.Load(m.UserSession)
		if ok {
			currentPlayer := playerVal.(*game.Player)
			if *currentPlayer.Color == msg.PlayerColor {
				log.Info("Current player died, quitting TUI.", "player", currentPlayer.Name)
				return m, tea.Quit
			}
		}
		// If another player died, just update the UI
		return m, m.listenForGameUpdates()
	}

	return m, nil
}

// --- GameScreenModel View Method (Restructured for Separation of Concerns) ---

func (m GameModel) View() string {
	currentPlayerVal, ok := m.gameManager.SessionsToPlayers.Load(m.UserSession)
	if !ok {
		return "Error: Could not find player session."
	}
	currentPlayer, _ := currentPlayerVal.(*game.Player)

	// 1. Calculate dimensions for MapView and StatusPanel
	mapViewWidth := int(float64(m.ScreenWidth) * mapViewPercentage)
	statusPanelWidth := m.ScreenWidth - mapViewWidth

	// 2. Generate the Map View
	mapViewContent := m.renderMapView(currentPlayer, mapViewWidth, m.ScreenHeight)

	// Apply the border style to the map view
	mapViewBox := mapViewStyle.
		Width(mapViewWidth).
		Height(m.ScreenHeight).
		Render(mapViewContent)

	// 3. Generate the Status Panel View
	statusPanelContent := m.renderStatusPanel(currentPlayer)

	// Apply the border style to the status panel
	statusPanelViewBox := statusPanelStyle.
		Width(statusPanelWidth - statusPanelPadding). // Account for padding/gap
		Height(m.ScreenHeight - statusPanelPadding).  // Account for padding/border
		Render(statusPanelContent)

	// 4. Combine them using lipgloss.JoinHorizontal
	return lipgloss.JoinHorizontal(lipgloss.Top, mapViewBox, statusPanelViewBox)
}

// renderMapView is extracted logic to draw the main game map viewport.
func (m GameModel) renderMapView(currentPlayer *game.Player, viewportWidth, viewportHeight int) string {

	// Adjust viewport to account for the border applied in View(), which adds 2 to width/height
	innerViewportWidth := viewportWidth - 2
	innerViewportHeight := viewportHeight - 2

	// Clamp to a reasonable maximum if map is smaller than viewport
	innerViewportHeight = min(innerViewportHeight, game.MapRowCount)
	innerViewportWidth = min(innerViewportWidth, game.MapColCount)

	startRow, startCol := 0, 0

	// Calculate the starting row/col to center the view on the player
	if currentPlayer != nil && currentPlayer.Location != nil {
		startRow = currentPlayer.Location.Y - innerViewportHeight/2
		startCol = currentPlayer.Location.X - innerViewportWidth/2
	}

	// Clamp the viewport coordinates to map boundaries
	startRow = max(0, min(startRow, game.MapRowCount-innerViewportHeight))
	startCol = max(0, min(startCol, game.MapColCount-innerViewportWidth))

	// If the map is smaller than the calculated viewport, force start point to (0,0)
	if game.MapRowCount <= innerViewportHeight {
		startRow = 0
	}
	if game.MapColCount <= innerViewportWidth {
		startCol = 0
	}

	endRow := startRow + innerViewportHeight
	endCol := startCol + innerViewportWidth

	var mapView strings.Builder
	gameMap := m.gameManager.GameMap

	// Loop through only the tiles within the calculated viewport
	for row := startRow; row < endRow && row < game.MapRowCount; row++ {
		for col := startCol; col < endCol && col < game.MapColCount; col++ {
			currTile := gameMap[row][col]
			colorStyle := lipgloss.NewStyle()

			// Check if the current tile is the player's head
			if currentPlayer != nil && currentPlayer.Location == currTile {
				// **FIX:** Use the arrow rune for the current player's head
				colorStyle = colorStyle.Foreground(lipgloss.Color(strconv.Itoa(*currentPlayer.Color)))
				arrowRune := headRunes[currentPlayer.CurrentDirection]
				mapView.WriteString(colorStyle.Render(arrowRune))
				continue
			}

			// Rendering logic based on Tile state
			if currTile.OwnerColor != nil {
				colorStyle = colorStyle.Foreground(lipgloss.Color(strconv.Itoa(*currTile.OwnerColor))).Width(1)

				if currTile.IsTail {
					// Player tail/drawing line
					mapView.WriteString(colorStyle.Render("○"))
				} else {
					// Claimed estate
					mapView.WriteString(colorStyle.Render("░"))
				}
			} else {
				// Empty/Wall Space
				if row == 0 || col == 0 || row == game.MapRowCount-1 || col == game.MapColCount-1 {
					// Wall
					mapView.WriteString(wallStyle.Render("█"))
				} else {
					// Void space
					mapView.WriteString(voidStyle.Render("░"))
				}
			}
		}
		mapView.WriteString("\n")
	}

	return mapView.String()
}

// renderStatusPanel is extracted logic to draw the right-hand status/help panel.
func (m GameModel) renderStatusPanel(currentPlayer *game.Player) string {
	var statusContent strings.Builder

	// Player Stats
	statusContent.WriteString(lipgloss.NewStyle().Bold(true).Render("--- Player Stats ---") + "\n")
	statusContent.WriteString(fmt.Sprintf("Name: %s\n", currentPlayer.Name))
	statusContent.WriteString(fmt.Sprintf("Color: %s%d\n", lipgloss.NewStyle().Foreground(lipgloss.Color(strconv.Itoa(*currentPlayer.Color))).Render("●"), *currentPlayer.Color))
	statusContent.WriteString(fmt.Sprintf("Location: (%d, %d)\n", currentPlayer.Location.X, currentPlayer.Location.Y))
	statusContent.WriteString(fmt.Sprintf("Direction: %s\n", headRunes[currentPlayer.CurrentDirection]))
	statusContent.WriteString(fmt.Sprintf("Game Tick: %d\n", m.TickCount))

	// Leaderboard
	statusContent.WriteString("\n" + lipgloss.NewStyle().Bold(true).Render("--- Leaderboard ---") + "\n")

	// Sort and display estates here for a proper leaderboard, but for now, simple iteration
	m.gameManager.Players.Range(func(key, value interface{}) bool {
		player := value.(*game.Player)
		if player != nil {
			estate, found := m.EstateInfo[player.Color]
			if found {
				colorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(strconv.Itoa(*player.Color)))
				statusContent.WriteString(fmt.Sprintf("%s%s: %d tiles\n", colorStyle.Render("● "), player.Name, estate))
			}
		}
		return true
	})

	// Help/Controls
	statusContent.WriteString("\n" + lipgloss.NewStyle().Bold(true).Render("--- Controls ---") + "\n")
	statusContent.WriteString("WASD / Arrows: Move\n")
	statusContent.WriteString("Q / Ctrl+C: Quit Game\n")
	statusContent.WriteString("\n" + lipgloss.NewStyle().Faint(true).Render("SSHNake v0.1"))

	// Add padding to ensure the content fills the height if needed, though lipgloss.Height handles this better
	return statusContent.String()
}

// min is a helper function to find the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// max is a helper function to find the maximum of two integers.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
