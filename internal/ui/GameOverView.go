package ui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Mshel/sshnake/internal/game"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Messages for GameOverModel to signal transition back to the Controller
type ShowLeaderboardFromGameOverMsg struct {
	LeaderboardData []PlayerScore // Data ready to pass to the LeaderboardModel
	EstateInfo      map[*int]int  // Estate data ready to pass to the LeaderboardModel
}
type ReturnToIntroMsg struct{} // To signal the Controller to go back to Intro/Quit

// Messages for LeaderboardModel to signal transition back to the Controller
type ReturnFromLeaderboardMsg struct{}

// Shared Styles
var (
	leaderboardHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("15")).
				Background(lipgloss.Color("236")).
				Padding(0, 1).
				Align(lipgloss.Center)

	leaderboardRowStyle = lipgloss.NewStyle().
				Padding(0, 1)

	leaderboardBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), false, false, true, false).
				BorderForeground(lipgloss.Color("8"))
)

// --- GAME OVER MODEL ---
// Dedicated model for the Game Over screen. It handles button selection and sends transition messages.

type GameOverModel struct {
	tea.Model
	GameManager     *game.GameManager
	FinalEstate     float64       // Final claimed land percentage
	FinalKills      int           // Final kill count
	SelectedButton  int           // 0 for EXIT, 1 for LEADERBOARD
	LeaderboardData []PlayerScore // Current leaderboard snapshot for passing
	EstateInfo      map[*int]int  // Current estate info for passing
	ScreenWidth     int
	ScreenHeight    int
}

func NewGameOverModel(gm *game.GameManager, finalEstate float64, finalKills int, lbData []PlayerScore, estateInfo map[*int]int, screenWidth, screenHeight int) GameOverModel {
	return GameOverModel{
		GameManager:     gm,
		FinalEstate:     finalEstate,
		FinalKills:      finalKills,
		SelectedButton:  0, // Default to EXIT
		LeaderboardData: lbData,
		EstateInfo:      estateInfo,
		ScreenWidth:     screenWidth,
		ScreenHeight:    screenHeight,
	}
}

func (m GameOverModel) Init() tea.Cmd {
	return nil
}

func (m GameOverModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "left", "h":
			m.SelectedButton = max(0, m.SelectedButton-1)
		case "right", "l":
			m.SelectedButton = min(1, m.SelectedButton+1)
		case "enter":
			if m.SelectedButton == 0 {
				return m, func() tea.Msg { return QuitGameMsg{} }
			} else {
				return m, func() tea.Msg {
					return ShowLeaderboardFromGameOverMsg{
						LeaderboardData: m.LeaderboardData,
						EstateInfo:      m.EstateInfo,
					}
				}
			}
		case "esc":
			return m, func() tea.Msg { return QuitGameMsg{} }
		}
	}
	return m, nil
}

func (m GameOverModel) View() string {
	messageStyle := lipgloss.NewStyle().
		Padding(2, 5).
		Align(lipgloss.Center).
		Width(m.ScreenWidth - 4)

	title := messageStyle.Render(" Good Game! ")

	stats := fmt.Sprintf("\nFinal Stats:\n Land Claimed: %.2f%% \nPlayer Kills: %d\n\n", m.FinalEstate, m.FinalKills)

	exitButton := buttonStyle.Render("EXIT (Enter)")
	leaderboardButton := buttonStyle.Render("LEADERBOARD")

	if m.SelectedButton == 0 {
		exitButton = submitButtonStyle.Render("EXIT (Enter)")
	} else {
		leaderboardButton = submitButtonStyle.Render("LEADERBOARD")
	}

	buttons := lipgloss.JoinHorizontal(lipgloss.Center, exitButton, leaderboardButton)

	content := lipgloss.JoinVertical(lipgloss.Center, title, stats, buttons)

	return lipgloss.Place(m.ScreenWidth, m.ScreenHeight,
		lipgloss.Center, lipgloss.Center,
		lipgloss.NewStyle().Border(lipgloss.ThickBorder()).Render(content),
	)
}

type LeaderboardModel struct {
	tea.Model
	GameManager  *game.GameManager
	EstateInfo   map[*int]int
	Leaderboard  []PlayerScore
	ScreenWidth  int
	ScreenHeight int
}

func NewLeaderboardModel(gm *game.GameManager, estateInfo map[*int]int, leaderboardData []PlayerScore, screenWidth, screenHeight int) LeaderboardModel {
	return LeaderboardModel{
		GameManager:  gm,
		EstateInfo:   estateInfo,
		Leaderboard:  leaderboardData,
		ScreenWidth:  screenWidth,
		ScreenHeight: screenHeight,
	}
}

func (m LeaderboardModel) Init() tea.Cmd {
	return nil
}

func (m LeaderboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "esc", "enter":
			// Signal the Controller to change screen back to Game Over or Intro
			return m, func() tea.Msg { return ReturnFromLeaderboardMsg{} }
		}
	}
	return m, nil
}

func (m LeaderboardModel) View() string {
	var tableContent strings.Builder

	type PlayerScoreForDisplay struct {
		Name   string
		Color  int
		Estate int // Use raw tile count for sorting/display
	}

	var scores []PlayerScoreForDisplay
	// Reconstruct the scores list with current estate size from the EstateInfo map
	for _, pScore := range m.Leaderboard {
		var estate int
		found := false
		for colorPtr, value := range m.EstateInfo {
			if *colorPtr == pScore.Color {
				estate = value
				found = true
				break
			}
		}
		if !found {
			// If estate info is missing, use the Land value which is the raw tile count from GameView's PlayerScore struct
			estate = int(pScore.Land)
		}

		scores = append(scores, PlayerScoreForDisplay{
			Name:   pScore.Name,
			Color:  pScore.Color,
			Estate: estate,
		})
	}

	// Sort by Estate (total claimed tiles)
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Estate > scores[j].Estate
	})

	nameWidth := 15
	estateWidth := 10

	header := lipgloss.JoinHorizontal(lipgloss.Top,
		leaderboardHeaderStyle.Width(3).Render("#"),
		leaderboardHeaderStyle.Width(nameWidth).Render("Player"),
		leaderboardHeaderStyle.Width(estateWidth).Render("Estate"),
	)
	tableContent.WriteString(header + "\n")
	for i, score := range scores {
		rank := i + 1
		colorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(strconv.Itoa(score.Color)))

		row := lipgloss.JoinHorizontal(lipgloss.Top,
			leaderboardRowStyle.Width(3).Render(strconv.Itoa(rank)),
			colorStyle.Width(nameWidth).Render(score.Name),
			leaderboardRowStyle.Width(estateWidth).Render(strconv.Itoa(score.Estate)),
		)

		tableContent.WriteString(leaderboardBorderStyle.Render(row) + "\n")
	}

	title := lipgloss.NewStyle().Bold(true).Padding(1, 0).Render("CURRENT LEADERBOARD")
	instruction := lipgloss.NewStyle().Faint(true).Margin(1, 0).Render("Press ESC or ENTER to return.")

	finalContent := lipgloss.JoinVertical(lipgloss.Center,
		title,
		tableContent.String(),
		instruction,
	)

	return lipgloss.Place(m.ScreenWidth, m.ScreenHeight,
		lipgloss.Center, lipgloss.Center,
		lipgloss.NewStyle().Border(lipgloss.ThickBorder()).Render(finalContent),
	)
}
