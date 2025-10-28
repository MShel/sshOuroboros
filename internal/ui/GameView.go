package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Mshel/sshnake/internal/game"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
)

// --- Internal Game States for GameViewModel ---

type GameState int

const (
	StatePlaying GameState = iota
	StateGameOver
	StateLeaderboard
)

var (
	mapViewStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 0)

	statusPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("8")).
				Padding(1, 2)

	wallStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("237")).Render("█")
	voidStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("235")).Render(" ")

	headRunes = map[game.Direction]rune{
		{Dx: 0, Dy: -1}: '▲', // Up
		{Dx: 0, Dy: 1}:  '▼', // Down
		{Dx: -1, Dy: 0}: '◀', // Left
		{Dx: 1, Dy: 0}:  '▶', // Right
	}

	tailRunes = map[game.Direction]string{
		{Dx: 0, Dy: -1}: "│", // Up
		{Dx: 0, Dy: 1}:  "│", // Down
		{Dx: -1, Dy: 0}: "─", // Left
		{Dx: 1, Dy: 0}:  "─", // Right
	}

	cornerRunes = map[[2]game.Direction]string{
		// Up -> Right | Right -> Up
		{{Dx: 0, Dy: -1}, {Dx: 1, Dy: 0}}: "┘",
		{{Dx: 1, Dy: 0}, {Dx: 0, Dy: -1}}: "┘",

		// Up -> Left | Left -> Up
		{{Dx: 0, Dy: -1}, {Dx: -1, Dy: 0}}: "└",
		{{Dx: -1, Dy: 0}, {Dx: 0, Dy: -1}}: "└",

		// Down -> Right | Right -> Down
		{{Dx: 0, Dy: 1}, {Dx: 1, Dy: 0}}: "┐",
		{{Dx: 1, Dy: 0}, {Dx: 0, Dy: 1}}: "┐",

		// Down -> Left | Left -> Down
		{{Dx: 0, Dy: 1}, {Dx: -1, Dy: 0}}: "┌",
		{{Dx: -1, Dy: 0}, {Dx: 0, Dy: 1}}: "┌",
	}

	claimedEstateRune = "▒"
)

const (
	mapViewPercentage  = 0.70
	statusPanelPadding = 4
	viewportWidth      = 80
	viewportHeight     = 40
)

// --- GameViewModel Definition ---

type GameViewModel struct {
	tea.Model
	TickCount    int
	EstateInfo   map[*int]int
	ScreenWidth  int
	ScreenHeight int
	gameManager  *game.GameManager
	UserSession  ssh.Session // nil if viewing leaderboard from IntroScreen

	gameState     GameState
	gameOverState GameOverState
}

func NewGameModel(gm *game.GameManager, session ssh.Session, screenWidth int, screenHeight int) GameViewModel {
	return GameViewModel{
		gameManager:  gm,
		UserSession:  session,
		TickCount:    0,
		EstateInfo:   make(map[*int]int),
		ScreenWidth:  screenWidth,
		ScreenHeight: screenHeight,
		gameState:    StatePlaying,
		gameOverState: GameOverState{
			GameManager:    gm,
			ScreenWidth:    screenWidth,
			ScreenHeight:   screenHeight,
			SelectedButton: 0,
		},
	}
}

// --- Init/Update/View Methods ---

func (m GameViewModel) Init() tea.Cmd {
	return m.listenForGameUpdates()
}

// QuitGameMsg is a custom message sent to the Controller to switch back to the IntroScreen
// when the user exits the Leaderboard view before registering.
type QuitGameMsg struct{}

func (m GameViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowLeaderboardMsg:
		// Command received from the controller to show the leaderboard immediately (e.g., from IntroScreen)
		m.gameState = StateLeaderboard
		return m, nil

	case tea.KeyMsg:

		// Handle Game Over or Leaderboard screen key presses
		if m.gameState == StateGameOver || m.gameState == StateLeaderboard {
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "esc":
				if m.gameState == StateLeaderboard {
					// If viewing leaderboard from a game, go back to Game Over menu
					if m.UserSession != nil {
						m.gameState = StateGameOver
						return m, nil
					} else {
						// If viewing leaderboard from Intro screen, go back to Intro screen
						return m, func() tea.Msg { return QuitGameMsg{} }
					}
				}
			case "left", "h":
				if m.gameState == StateGameOver {
					m.gameOverState.SelectedButton = max(0, m.gameOverState.SelectedButton-1)
				}
			case "right", "l":
				if m.gameState == StateGameOver {
					m.gameOverState.SelectedButton = min(1, m.gameOverState.SelectedButton+1)
				}
			case "enter":
				if m.gameState == StateGameOver {
					// 0: Exit, 1: Leaderboard
					if m.gameOverState.SelectedButton == 0 {
						return m, tea.Quit
					} else {
						m.gameState = StateLeaderboard
					}
				} else if m.gameState == StateLeaderboard {
					// Pressing Enter on the leaderboard screen
					if m.UserSession != nil {
						// Player was in a game, go back to Game Over menu
						m.gameState = StateGameOver
						return m, nil
					} else {
						// No active session (came from Intro), send message to go back to IntroScreen
						return m, func() tea.Msg { return QuitGameMsg{} }
					}
				}
				return m, nil
			}
			return m, nil
		}

		// ... Existing key handling for in-game movement ...

		// Player session must be valid for movement
		currentPlayerVal, ok := m.gameManager.SessionsToPlayers.Load(m.UserSession)
		if !ok {
			return m, nil
		}
		currentPlayer := currentPlayerVal.(*game.Player)

		var engineCommand game.Direction
		switch msg.String() {
		case "w", "up":
			engineCommand = game.Direction{Dx: 0, Dy: -1, PlayerColor: *currentPlayer.Color}
		case "s", "down":
			engineCommand = game.Direction{Dx: 0, Dy: 1, PlayerColor: *currentPlayer.Color}
		case "a", "left":
			engineCommand = game.Direction{Dx: -1, Dy: 0, PlayerColor: *currentPlayer.Color}
		case "d", "right":
			engineCommand = game.Direction{Dx: 1, Dy: 0, PlayerColor: *currentPlayer.Color}
		default:
			return m, nil
		}

		// Prevent moving backwards
		if engineCommand.Dx == -currentPlayer.CurrentDirection.Dx && engineCommand.Dy == -currentPlayer.CurrentDirection.Dy {
			return m, nil
		}

		m.gameManager.DirectionChannel <- engineCommand
		return m, nil

	case game.GameTickMsg:
		m.TickCount++
		return m, m.listenForGameUpdates()

	case game.ClaimedEstateMsg:
		m.EstateInfo = msg.PlayersEstate
		return m, m.listenForGameUpdates()

	case game.PlayerDeadMsg:
		currentPlayerVal, ok := m.gameManager.SessionsToPlayers.Load(m.UserSession)
		if ok {
			currentPlayer := currentPlayerVal.(*game.Player)
			if *currentPlayer.Color == msg.PlayerColor {
				log.Info("Current player died, showing Game Over screen.", "player", currentPlayer.Name)
				m.gameState = StateGameOver
				m.gameOverState.FinalEstate = msg.FinalClaimedEstate
				m.gameOverState.FinalKills = msg.FinalKills
				m.gameOverState.SelectedButton = 0
				return m, m.listenForGameUpdates()
			}
		}
		return m, m.listenForGameUpdates()
	}

	return m, nil
}

func (m GameViewModel) View() string {
	if m.gameState == StateGameOver {
		return m.gameOverState.RenderGameOverScreen()
	}

	if m.gameState == StateLeaderboard {
		return m.gameOverState.RenderLeaderboardScreen(m.EstateInfo)
	}

	currentPlayerVal, ok := m.gameManager.SessionsToPlayers.Load(m.UserSession)
	if !ok {
		// Should only happen if the player died, but state wasn't updated,
		// or if session is nil (from Intro) but state is Playing (which shouldn't happen)
		return lipgloss.Place(m.ScreenWidth, m.ScreenHeight, lipgloss.Center, lipgloss.Center, "Waiting for game manager...")
	}

	currentPlayer := currentPlayerVal.(*game.Player)

	mapWidth := int(float64(m.ScreenWidth) * mapViewPercentage)
	statusPanelWidth := m.ScreenWidth - mapWidth - statusPanelPadding

	mapContent := m.renderMap(currentPlayer, mapWidth, m.ScreenHeight)
	statusContent := m.renderStatusPanel(currentPlayer, statusPanelWidth)

	return lipgloss.JoinHorizontal(lipgloss.Top,
		mapViewStyle.Width(mapWidth).Height(m.ScreenHeight).Render(mapContent),
		statusPanelStyle.Width(statusPanelWidth).Height(m.ScreenHeight).Render(statusContent),
	)
}

// ... renderMap, renderStatusPanel, listenForGameUpdates, max, min helpers (as before) ...

// renderMap calculates the viewport around the player and draws the map.
func (m GameViewModel) renderMap(currentPlayer *game.Player, width int, height int) string {
	var sb strings.Builder

	centerTileX := currentPlayer.Location.X
	centerTileY := currentPlayer.Location.Y

	effectiveViewportW := min(viewportWidth, width)
	effectiveViewportH := min(viewportHeight, height)

	startCol := max(0, centerTileX-effectiveViewportW/2)
	endCol := min(game.MapColCount, centerTileX+effectiveViewportW/2+1)
	startRow := max(0, centerTileY-effectiveViewportH/2)
	endRow := min(game.MapRowCount, centerTileY+effectiveViewportH/2+1)

	m.gameManager.MapMutex.RLock()
	defer m.gameManager.MapMutex.RUnlock()

	for row := startRow; row < endRow; row++ {
		for col := startCol; col < endCol; col++ {
			if row <= 0 || row >= game.MapRowCount-1 || col <= 0 || col >= game.MapColCount-1 {
				sb.WriteString(wallStyle)
				continue
			}

			tile := m.gameManager.GameMap[row][col]
			var tileOwner *game.Player
			if tile.OwnerColor != nil {
				tileOwnerAny, ownerExists := m.gameManager.Players.Load(*tile.OwnerColor)
				if ownerExists {
					tileOwner = tileOwnerAny.(*game.Player)
				}
			}

			// 1. Draw Player Head
			if tileOwner != nil && tile == tileOwner.Location {

				colorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(strconv.Itoa(*tileOwner.Color))).Bold(true)
				sb.WriteString(colorStyle.Render(string(headRunes[game.Direction{Dx: tileOwner.CurrentDirection.Dx, Dy: tileOwner.CurrentDirection.Dy}])))

				continue
			}

			// 2. Draw Owned Tail/Estate
			if tile.OwnerColor != nil {
				colorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(strconv.Itoa(*tile.OwnerColor)))

				if tile.IsTail {
					// Determine which adjacent tail tiles belong to same owner.
					hasUp, hasDown, hasLeft, hasRight := false, false, false, false

					// Up
					if row-1 >= 0 {
						n := m.gameManager.GameMap[row-1][col]
						if n.IsTail && n.OwnerColor != nil && tile.OwnerColor != nil && *n.OwnerColor == *tile.OwnerColor {
							hasUp = true
						}
					}
					// Down
					if row+1 < game.MapRowCount {
						n := m.gameManager.GameMap[row+1][col]
						if n.IsTail && n.OwnerColor != nil && tile.OwnerColor != nil && *n.OwnerColor == *tile.OwnerColor {
							hasDown = true
						}
					}
					// Left
					if col-1 >= 0 {
						n := m.gameManager.GameMap[row][col-1]
						if n.IsTail && n.OwnerColor != nil && tile.OwnerColor != nil && *n.OwnerColor == *tile.OwnerColor {
							hasLeft = true
						}
					}
					// Right
					if col+1 < game.MapColCount {
						n := m.gameManager.GameMap[row][col+1]
						if n.IsTail && n.OwnerColor != nil && tile.OwnerColor != nil && *n.OwnerColor == *tile.OwnerColor {
							hasRight = true
						}
					}

					// Choose rune:
					var tailRune string

					switch {
					// Straight vertical (or single vertical neighbor)
					case (hasUp && hasDown) || (hasUp && !hasLeft && !hasRight && !hasDown) || (hasDown && !hasLeft && !hasRight && !hasUp):
						tailRune = "│"
					// Straight horizontal (or single horizontal neighbor)
					case (hasLeft && hasRight) || (hasLeft && !hasUp && !hasDown && !hasRight) || (hasRight && !hasUp && !hasDown && !hasLeft):
						tailRune = "─"
					// Up + Right => arms Up & Right
					case hasUp && hasRight:
						tailRune = "└"
					// Up + Left => arms Up & Left
					case hasUp && hasLeft:
						tailRune = "┘"
					// Down + Right => arms Down & Right
					case hasDown && hasRight:
						tailRune = "┌"
					// Down + Left => arms Down & Left
					case hasDown && hasLeft:
						tailRune = "┐"
					// Fallback (isolated or weird) — draw a small bullet
					default:
						tailRune = "•"
					}

					sb.WriteString(colorStyle.Render(tailRune))
				} else {
					sb.WriteString(colorStyle.Render(claimedEstateRune))
				}
			} else {
				// 3. Draw Void/Empty space
				sb.WriteString(voidStyle)
			}
		}
		sb.WriteString("\n")
	}

	renderedMap := sb.String()
	paddedMap := lipgloss.NewStyle().Width(width).Height(height).Render(renderedMap)

	return paddedMap
}

// renderStatusPanel draws the stats and a simplified leaderboard.
func (m GameViewModel) renderStatusPanel(currentPlayer *game.Player, width int) string {
	var statusContent strings.Builder

	statusContent.WriteString(lipgloss.NewStyle().Bold(true).Render("--- Player Stats ---\n"))
	colorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(strconv.Itoa(*currentPlayer.Color)))
	statusContent.WriteString(fmt.Sprintf("%s%s\n", colorStyle.Render("● "), currentPlayer.Name))

	var currentKills int
	m.gameManager.Players.Range(func(key, value interface{}) bool {
		if otherPlayer, ok := value.(*game.Player); ok && *otherPlayer.Color == *currentPlayer.Color {
			currentKills = otherPlayer.Kills
			return false
		}
		return true
	})

	statusContent.WriteString(fmt.Sprintf("Kills: %d\n", currentKills))

	estate, found := m.EstateInfo[currentPlayer.Color]
	if !found {
		estate = 0
	}
	statusContent.WriteString(fmt.Sprintf("Estate: %d tiles\n", estate))

	statusContent.WriteString(fmt.Sprintf("Direction: %c\n", headRunes[currentPlayer.CurrentDirection]))
	statusContent.WriteString(fmt.Sprintf("Game Tick: %d\n", m.TickCount))

	statusContent.WriteString("\n" + lipgloss.NewStyle().Bold(true).Render("--- Leaderboard ---") + "\n")

	type PlayerScore struct {
		Name   string
		Color  int
		Estate int
	}
	var scores []PlayerScore
	m.gameManager.Players.Range(func(key, value interface{}) bool {
		player := value.(*game.Player)
		if player != nil {
			estate, found := m.EstateInfo[player.Color]
			if !found {
				estate = 0
			}
			scores = append(scores, PlayerScore{
				Name:   player.Name,
				Color:  *player.Color,
				Estate: estate,
			})
		}
		return true
	})

	for i := 0; i < len(scores)-1; i++ {
		for j := i + 1; j < len(scores); j++ {
			if scores[i].Estate < scores[j].Estate {
				scores[i], scores[j] = scores[j], scores[i]
			}
		}
	}

	for i, score := range scores {
		colorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(strconv.Itoa(score.Color)))
		statusContent.WriteString(fmt.Sprintf("%d. %s%s: %d\n", i+1, colorStyle.Render("● "), score.Name, score.Estate))
	}

	statusContent.WriteString("\n" + lipgloss.NewStyle().Bold(true).Render("--- Controls ---") + "\n")
	statusContent.WriteString("WASD / Arrows: Move\n")
	statusContent.WriteString("Q / Ctrl+C: Quit Game\n")
	statusContent.WriteString("\n" + lipgloss.NewStyle().Faint(true).Render("Press ESC/Enter to Exit"))

	return statusContent.String()
}

func (m GameViewModel) listenForGameUpdates() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		if m.UserSession == nil {
			// If no session, still tick to keep the leaderboard/game running in the background
			return game.GameTickMsg{}
		}

		currentPlayerVal, ok := m.gameManager.SessionsToPlayers.Load(m.UserSession)
		if !ok {
			return game.GameTickMsg{}
		}

		currentPlayer := currentPlayerVal.(*game.Player)

		select {
		case msg := <-currentPlayer.UpdateChannel:
			return msg
		default:
			return game.GameTickMsg{}
		}
	})
}
