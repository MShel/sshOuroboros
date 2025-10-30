package ui

import (
	"fmt"
	"sort"
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
	voidColor    = "233"
	mapViewStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 0)

	statusPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("8")).
				Padding(1, 2)

	wallStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("172")).Render("▒")
	voidStyle = lipgloss.NewStyle().Background(lipgloss.Color(voidColor)).Render(" ")

	headRunes = map[game.Direction]rune{
		{Dx: 0, Dy: -1}: '▲', // Up
		{Dx: 0, Dy: 1}:  '▼', // Down
		{Dx: -1, Dy: 0}: '◀', // Left
		{Dx: 1, Dy: 0}:  '▶', // Right
	}

	claimedEstateRune = "▒"
)

const (
	mapViewPercentage  = 0.70
	statusPanelPadding = 4
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
				switch m.gameState {
				case StateGameOver:
					// 0: Exit, 1: Leaderboard
					if m.gameOverState.SelectedButton == 0 {
						return m, tea.Quit
					} else {
						m.gameState = StateLeaderboard
					}
				case StateLeaderboard:
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
				m.gameOverState.FinalEstate = (msg.FinalClaimedEstate * 100) / float64(game.MapColCount*game.MapRowCount)
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
func (m GameViewModel) renderMap(currentPlayer *game.Player, width int, height int) string {
	var sb strings.Builder

	centerTileX := currentPlayer.Location.X
	centerTileY := currentPlayer.Location.Y

	effectiveViewportW := min(game.MapColCount, width)
	effectiveViewportH := min(game.MapRowCount, height)

	desiredStartCol := centerTileX - effectiveViewportW/2

	startCol := max(0, desiredStartCol)

	if startCol+effectiveViewportW > game.MapColCount {
		startCol = max(0, game.MapColCount-effectiveViewportW)
	}

	endCol := min(game.MapColCount, startCol+effectiveViewportW)

	desiredStartRow := centerTileY - effectiveViewportH/2

	startRow := max(0, desiredStartRow)

	if startRow+effectiveViewportH > game.MapRowCount {
		startRow = max(0, game.MapRowCount-effectiveViewportH)
	}

	endRow := min(game.MapRowCount, startRow+effectiveViewportH)

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

			if tileOwner != nil && tile == tileOwner.Location {
				colorStyle := lipgloss.NewStyle().Background(lipgloss.Color(voidColor)).Foreground(lipgloss.Color(strconv.Itoa(*tileOwner.Color))).Bold(true)
				sb.WriteString(colorStyle.Render(string(headRunes[game.Direction{Dx: tileOwner.CurrentDirection.Dx, Dy: tileOwner.CurrentDirection.Dy}])))
				continue
			}

			if tile.OwnerColor != nil {
				colorStyle := lipgloss.NewStyle().Background(lipgloss.Color(voidColor)).Foreground(lipgloss.Color(strconv.Itoa(*tile.OwnerColor)))

				if tile.IsTail {
					hasUp, hasDown, hasLeft, hasRight := false, false, false, false

					if row-1 >= 0 {
						n := m.gameManager.GameMap[row-1][col]
						if n.IsTail && n.OwnerColor != nil && tile.OwnerColor != nil && *n.OwnerColor == *tile.OwnerColor {
							hasUp = true
						}
					}
					if row+1 < game.MapRowCount {
						n := m.gameManager.GameMap[row+1][col]
						if n.IsTail && n.OwnerColor != nil && tile.OwnerColor != nil && *n.OwnerColor == *tile.OwnerColor {
							hasDown = true
						}
					}
					if col-1 >= 0 {
						n := m.gameManager.GameMap[row][col-1]
						if n.IsTail && n.OwnerColor != nil && tile.OwnerColor != nil && *n.OwnerColor == *tile.OwnerColor {
							hasLeft = true
						}
					}
					if col+1 < game.MapColCount {
						n := m.gameManager.GameMap[row][col+1]
						if n.IsTail && n.OwnerColor != nil && tile.OwnerColor != nil && *n.OwnerColor == *tile.OwnerColor {
							hasRight = true
						}
					}

					var tailRune string

					switch {
					case (hasUp && hasDown) || (hasUp && !hasLeft && !hasRight && !hasDown) || (hasDown && !hasLeft && !hasRight && !hasUp):
						tailRune = "│"
					case (hasLeft && hasRight) || (hasLeft && !hasUp && !hasDown && !hasRight) || (hasRight && !hasUp && !hasDown && !hasLeft):
						tailRune = "─"
					case hasUp && hasRight:
						tailRune = "└"
					case hasUp && hasLeft:
						tailRune = "┘"
					case hasDown && hasRight:
						tailRune = "┌"
					case hasDown && hasLeft:
						tailRune = "┐"
					default:
						tailRune = "•"
					}

					sb.WriteString(colorStyle.Render(tailRune))
				} else {
					sb.WriteString(colorStyle.Render(claimedEstateRune))
				}
			} else {
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

	claimedLand := currentPlayer.GetConsolidateTiles()
	statusContent.WriteString(fmt.Sprintf("Kills: %d\n", currentPlayer.Kills))
	statusContent.WriteString(fmt.Sprintf("Claimed: %.2f %% of land\n", claimedLand*100/float64(game.MapColCount*game.MapColCount)))
	statusContent.WriteString(fmt.Sprintf("Direction: %c\n", headRunes[game.Direction{Dx: currentPlayer.CurrentDirection.Dx, Dy: currentPlayer.CurrentDirection.Dy}]))

	statusContent.WriteString("\n" + lipgloss.NewStyle().Bold(true).Render("--- Leaderboard(TOP 10) ---") + "\n")

	type PlayerScore struct {
		Name  string
		Color int
		Land  float64
	}
	var playerScores []PlayerScore
	botCount := 0
	realPlayerCount := 0

	// use that to calculate claimed area list for all players
	// bot count
	// real player count
	m.gameManager.Players.Range(func(key, value interface{}) bool {
		player, _ := value.(*game.Player)
		playerScores = append(playerScores, PlayerScore{
			Name:  player.Name,
			Color: *player.Color,
			Land:  player.GetConsolidateTiles(),
		})

		if player.BotStrategy != nil {
			botCount += 1
		} else {
			realPlayerCount += 1
		}
		return true
	})

	sort.Slice(playerScores, func(i, j int) bool {
		return playerScores[i].Land > playerScores[j].Land
	})

	statusContent.WriteString(fmt.Sprintf("PlayerCount: %d\n", realPlayerCount))
	statusContent.WriteString(fmt.Sprintf("Bots count: %d\n", botCount))

	playerScores = playerScores[:10]

	for i, score := range playerScores {
		colorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(strconv.Itoa(score.Color)))
		statusContent.WriteString(fmt.Sprintf("%d. %s%s: %.2f %%\n", i+1, colorStyle.Render("● "), score.Name,
			score.Land*100/float64(game.MapColCount*game.MapColCount)))
	}

	statusContent.WriteString("\n" + lipgloss.NewStyle().Bold(true).Render("--- Controls ---") + "\n")
	statusContent.WriteString("WASD / Arrows: Move\n")
	statusContent.WriteString("Q / Ctrl+C: Quit Game\n")
	statusContent.WriteString("\n" + lipgloss.NewStyle().Faint(true).Render("Press ESC/Enter to Exit"))

	return statusContent.String()
}

func (m GameViewModel) listenForGameUpdates() tea.Cmd {
	return tea.Tick(time.Millisecond*50, func(t time.Time) tea.Msg {
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
