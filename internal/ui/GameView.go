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

	wallStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(strconv.Itoa(game.WallColor))).Render("▒")
	voidStyle = lipgloss.NewStyle().Background(lipgloss.Color(strconv.Itoa(game.VoidColor))).Render(" ")

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

type PlayerScore struct {
	Name  string
	Color int
	Land  float64
}

type GameViewModel struct {
	tea.Model
	TickCount       int
	EstateInfo      map[*int]int
	ScreenWidth     int
	ScreenHeight    int
	gameManager     *game.GameManager
	UserSession     ssh.Session
	gameState       GameState
	gameOverState   GameOverState
	LeaderboardData []PlayerScore
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
		LeaderboardData: make([]PlayerScore, 0),
	}
}

func (m GameViewModel) Init() tea.Cmd {
	return m.listenForGameUpdates()
}

type QuitGameMsg struct{}

func (m GameViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowLeaderboardMsg:
		m.gameState = StateLeaderboard
		return m, nil

	case tea.KeyMsg:
		if m.gameState == StateGameOver || m.gameState == StateLeaderboard {
			switch msg.String() {
			case "esc":
				if m.gameState == StateLeaderboard {
					if m.UserSession != nil {
						m.gameState = StateGameOver
						return m, nil
					} else {
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
					if m.gameOverState.SelectedButton == 0 {
						return m, tea.Quit
					} else {
						m.gameState = StateLeaderboard
					}
				case StateLeaderboard:
					if m.UserSession != nil {
						m.gameState = StateGameOver
						return m, nil
					} else {
						return m, func() tea.Msg { return QuitGameMsg{} }
					}
				}
				return m, nil
			}
			return m, nil
		}

		currentPlayerVal, ok := m.gameManager.SessionsToPlayers.Load(m.UserSession)
		if !ok || currentPlayerVal == nil {
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

		if (engineCommand.Dx == -currentPlayer.CurrentDirection.Dx && engineCommand.Dy == -currentPlayer.CurrentDirection.Dy) || (engineCommand == currentPlayer.CurrentDirection) {
			return m, nil
		}

		if engineCommand == currentPlayer.CurrentDirection {
			return m, nil
		}

		if (engineCommand == game.Direction{}) {
			return m, nil
		}

		select {
		case m.gameManager.DirectionChannel <- engineCommand:
			log.Info("sending")
			// Successfully sent direction
		default:
			log.Warn("direction channels is full")
		}
		return m, nil

	case game.GameTickMsg:
		m.TickCount++
		m.LeaderboardData = m.calculateLeaderboard()
		return m, m.listenForGameUpdates()

	case game.ClaimedEstateMsg:
		m.EstateInfo = msg.PlayersEstate
		m.LeaderboardData = m.calculateLeaderboard()
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
		m.LeaderboardData = m.calculateLeaderboard()
		return m, m.listenForGameUpdates()
	}

	return m, nil
}

func (m GameViewModel) calculateLeaderboard() []PlayerScore {
	var playerScores []PlayerScore

	m.gameManager.Players.Range(func(key, value interface{}) bool {
		player, _ := value.(*game.Player)
		playerScores = append(playerScores, PlayerScore{
			Name:  player.Name,
			Color: *player.Color,
			Land:  player.GetConsolidateTiles(),
		})
		return true
	})

	sort.Slice(playerScores, func(i, j int) bool {
		return playerScores[i].Land > playerScores[j].Land
	})

	return playerScores[:min(10, len(playerScores))]
}

func (m GameViewModel) View() string {
	if m.gameState == StateGameOver {
		return m.gameOverState.RenderGameOverScreen()
	}

	if m.gameState == StateLeaderboard {
		return m.gameOverState.RenderLeaderboardScreen(m.EstateInfo)
	}

	currentPlayerVal, ok := m.gameManager.SessionsToPlayers.Load(m.UserSession)
	if !ok || currentPlayerVal == nil {
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

	mapSegment := m.gameManager.GetMapCopy(startRow, endRow, startCol, endCol)
	if len(mapSegment) == 0 {
		return "Error rendering map segment."
	}

	viewHeight := len(mapSegment)
	viewWidth := len(mapSegment[0])

	for row := 0; row < viewHeight; row++ {
		for col := 0; col < viewWidth; col++ {
			globalRow := startRow + row
			globalCol := startCol + col

			if globalRow <= 0 || globalRow >= game.MapRowCount-1 || globalCol <= 0 || globalCol >= game.MapColCount-1 {
				sb.WriteString(wallStyle)
				continue
			}

			tile := mapSegment[row][col]
			var tileOwner *game.Player
			if tile.OwnerColor != nil {
				tileOwnerAny, ownerExists := m.gameManager.Players.Load(*tile.OwnerColor)
				if ownerExists {
					tileOwner = tileOwnerAny.(*game.Player)
				}
			}

			if tileOwner != nil && tile.X == tileOwner.Location.X && tile.Y == tileOwner.Location.Y {
				colorStyle := lipgloss.NewStyle().Background(lipgloss.Color(strconv.Itoa(game.VoidColor))).
					Foreground(lipgloss.Color(strconv.Itoa(*tileOwner.Color))).Bold(true)
				sb.WriteString(colorStyle.Render(string(headRunes[game.Direction{Dx: tileOwner.CurrentDirection.Dx, Dy: tileOwner.CurrentDirection.Dy}])))
				continue
			}

			if tile.OwnerColor != nil {
				colorStyle := lipgloss.NewStyle().Background(lipgloss.Color(strconv.Itoa(game.VoidColor))).
					Foreground(lipgloss.Color(strconv.Itoa(*tile.OwnerColor)))

				if tile.IsTail {
					// Logic for drawing connected tail lines
					hasUp, hasDown, hasLeft, hasRight := false, false, false, false

					if row-1 >= 0 {
						n := mapSegment[row-1][col]
						if n.IsTail && n.OwnerColor != nil && tile.OwnerColor != nil && *n.OwnerColor == *tile.OwnerColor {
							hasUp = true
						}
					}
					if row+1 < viewHeight {
						n := mapSegment[row+1][col]
						if n.IsTail && n.OwnerColor != nil && tile.OwnerColor != nil && *n.OwnerColor == *tile.OwnerColor {
							hasDown = true
						}
					}
					if col-1 >= 0 {
						n := mapSegment[row][col-1]
						if n.IsTail && n.OwnerColor != nil && tile.OwnerColor != nil && *n.OwnerColor == *tile.OwnerColor {
							hasLeft = true
						}
					}
					if col+1 < viewWidth {
						n := mapSegment[row][col+1]
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

	// Recalculate player counts (fast iteration)
	botCount := 0
	realPlayerCount := 0

	m.gameManager.Players.Range(func(key, value interface{}) bool {
		player, _ := value.(*game.Player)
		if player.BotStrategy != nil {
			botCount += 1
		} else {
			realPlayerCount += 1
		}
		return true
	})

	statusContent.WriteString(fmt.Sprintf("PlayerCount: %d\n", realPlayerCount))
	statusContent.WriteString(fmt.Sprintf("Bots count: %d\n", botCount))

	// UPDATED: Use the pre-calculated and sorted data
	for i, score := range m.LeaderboardData {
		colorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(strconv.Itoa(score.Color)))
		statusContent.WriteString(fmt.Sprintf("%d. %s%s: %.2f %%\n", i+1, colorStyle.Render("● "), score.Name,
			score.Land*100/float64(game.MapColCount*game.MapColCount)))
	}

	statusContent.WriteString("\n" + lipgloss.NewStyle().Bold(true).Render("--- Controls ---\n"))
	statusContent.WriteString("WASD / Arrows: Move\n")
	statusContent.WriteString("Q / Ctrl+C: Quit Game\n")
	statusContent.WriteString("\n" + lipgloss.NewStyle().Faint(true).Render("Press ESC/Enter to Exit"))

	return statusContent.String()
}

func (m GameViewModel) listenForGameUpdates() tea.Cmd {
	if m.UserSession == nil {
		return nil
	}

	currentPlayerVal, ok := m.gameManager.SessionsToPlayers.Load(m.UserSession)
	if !ok {
		return tea.Tick(game.GameTickDuration, func(t time.Time) tea.Msg {
			return game.GameTickMsg{}
		})
	}

	currentPlayer := currentPlayerVal.(*game.Player)
	return tea.Tick(game.GameTickDuration, func(t time.Time) tea.Msg {

		select {
		case msg := <-currentPlayer.UpdateChannel:
			return msg
		default:
			return nil
		}
	})
}
