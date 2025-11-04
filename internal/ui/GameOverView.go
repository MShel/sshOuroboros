package ui

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	// Added for date formatting
	"github.com/Mshel/sshnake/internal/game"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Messages for GameOverModel to signal transition back to the Controller
type ShowLeaderboardFromGameOverMsg struct {
	LeaderboardData []PlayerScore // Data ready to pass to the LeaderboardModel
	EstateInfo      map[*int]int  // Estate data ready to pass to the LeaderboardModel
	// The HighScoreService is now expected to be passed to NewLeaderboardModel externally.
}
type ReturnToIntroMsg struct{} // To signal the Controller to go back to Intro/Quit

// Messages for LeaderboardModel to signal transition back to the Controller
type ReturnFromLeaderboardMsg struct{}

// New message to handle fetching leaderboard data asynchronously
type LeaderboardScoresMsg struct {
	Scores      []game.Score
	TotalScores int
	Err         error
}

// Shared Styles
var (
	leaderboardHeaderStyle = lipgloss.NewStyle().
		// Removed Bold(true)
		Foreground(lipgloss.Color("15")). // Light Gray/White
		Background(lipgloss.Color("236")).
		Padding(0, 1). // Keep padding 0, 1 for headers
		Align(lipgloss.Center)

	leaderboardRowStyle = lipgloss.NewStyle().
				Padding(0, 1) // Keep padding 0, 1 for rows

	// Style to highlight top scores (achievements)
	highlightStyle = lipgloss.NewStyle().
		// Removed Bold(true)
		Foreground(lipgloss.Color("3")).  // Light Gold/Yellow for highlight
		Background(lipgloss.Color("237")) // Darker background for contrast

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
	HighScoreService *game.HighScoreService
	Scores           []game.Score
	TotalScores      int
	CurrentPage      int
	PageSize         int
	ScreenWidth      int
	ScreenHeight     int
	Loading          bool
	Error            error
}

func fetchScoresCmd(m LeaderboardModel) tea.Cmd {
	return func() tea.Msg {
		offset := (m.CurrentPage - 1) * m.PageSize

		scores, err := m.HighScoreService.GetHighScores(m.PageSize, offset)
		if err != nil {
			return LeaderboardScoresMsg{Err: fmt.Errorf("failed to fetch scores: %w", err)}
		}

		totalCount := m.TotalScores
		if m.TotalScores == 0 || m.CurrentPage == 1 {
			count, err := m.HighScoreService.GetTotalScoreCount()
			if err != nil {
				return LeaderboardScoresMsg{Scores: scores, TotalScores: len(scores), Err: fmt.Errorf("could not get total count: %w", err)}
			}
			totalCount = count
		}

		return LeaderboardScoresMsg{Scores: scores, TotalScores: totalCount}
	}
}

func NewLeaderboardModel(hss *game.HighScoreService, screenWidth, screenHeight int) LeaderboardModel {
	return LeaderboardModel{
		HighScoreService: hss,
		CurrentPage:      1,
		PageSize:         10, // Default page size
		ScreenWidth:      screenWidth,
		ScreenHeight:     screenHeight,
		Loading:          true,
	}
}

func (m LeaderboardModel) Init() tea.Cmd {
	return fetchScoresCmd(m)
}

func (m LeaderboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case LeaderboardScoresMsg:
		m.Loading = false
		m.Error = msg.Err
		if msg.Err == nil {
			m.Scores = msg.Scores
			m.TotalScores = msg.TotalScores
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "enter":
			// Signal the Controller to change screen back to Game Over or Intro
			return m, func() tea.Msg { return ReturnFromLeaderboardMsg{} }
		case "left", "h":
			if m.CurrentPage > 1 {
				m.CurrentPage--
				m.Loading = true
				return m, fetchScoresCmd(m)
			}
		case "right", "l":
			totalPages := int(math.Ceil(float64(m.TotalScores) / float64(m.PageSize)))
			if totalPages == 0 {
				totalPages = 1
			}
			if m.CurrentPage < totalPages {
				m.CurrentPage++
				m.Loading = true
				return m, fetchScoresCmd(m)
			}
		}
	}
	return m, cmd
}

func (m LeaderboardModel) View() string {
	if m.Loading {
		return lipgloss.Place(m.ScreenWidth, m.ScreenHeight,
			lipgloss.Center, lipgloss.Center,
			lipgloss.NewStyle().Border(lipgloss.ThickBorder()).Render("Loading Leaderboard..."),
		)
	}

	if m.Error != nil {
		return lipgloss.Place(m.ScreenWidth, m.ScreenHeight,
			lipgloss.Center, lipgloss.Center,
			// Changed Foreground("9") (Red) to Foreground("15") (Light Gray/White)
			lipgloss.NewStyle().Border(lipgloss.ThickBorder()).Foreground(lipgloss.Color("15")).Render(fmt.Sprintf("Error loading scores: %s", m.Error)),
		)
	}

	var tableContent strings.Builder

	// Define Column Widths
	rankWidth := 4
	nameWidth := 15
	estateWidth := 12 // Land Claimed (REAL)
	killsWidth := 7   // Increased from 7 to 9 to add a slight gap
	dateWidth := 15   // Increased from 15 to 17 to accommodate the shift and ensure alignment

	// 1. Header Row
	header := lipgloss.JoinHorizontal(lipgloss.Top,
		leaderboardHeaderStyle.Width(rankWidth).Render("#"),
		leaderboardHeaderStyle.Width(nameWidth).Render("Player"),
		leaderboardHeaderStyle.Width(estateWidth).Render("Land (%)"),
		leaderboardHeaderStyle.Width(killsWidth).Render("Kills"),
		leaderboardHeaderStyle.Width(dateWidth).Render("Date"),
	)
	tableContent.WriteString(header + "\n")

	// Calculate the starting rank for the current page
	startRank := (m.CurrentPage-1)*m.PageSize + 1

	// 2. Data Rows
	for i, score := range m.Scores {
		rank := startRank + i
		rowStyle := leaderboardRowStyle

		// Format date
		formattedDate := score.CreatedAt.Format("2006-01-02")
		if score.CreatedAt.IsZero() {
			formattedDate = "N/A"
		}

		// Format Land Claimed to percentage (%.2f)
		claimedLand := fmt.Sprintf("%.2f%%", score.ClaimedLand)

		row := lipgloss.JoinHorizontal(lipgloss.Top,
			rowStyle.Width(rankWidth).Render(strconv.Itoa(rank)),
			rowStyle.Width(nameWidth).Render(score.PlayerName),
			rowStyle.Width(estateWidth).Align(lipgloss.Right).Render(claimedLand),
			rowStyle.Width(killsWidth).Align(lipgloss.Right).Render(strconv.Itoa(score.Kills)), // Adjusted width here
			rowStyle.Width(dateWidth).Align(lipgloss.Left).Render(formattedDate),               // Adjusted width here
		)

		tableContent.WriteString(row + "\n")
	}

	// 3. Footer/Pagination
	totalPages := int(math.Ceil(float64(m.TotalScores) / float64(m.PageSize)))
	if totalPages == 0 {
		totalPages = 1
	}

	pageInfo := fmt.Sprintf("Page %d/%d (Total Scores: %d)", m.CurrentPage, totalPages, m.TotalScores)

	// Navigation Arrows with subtle color change when available
	arrowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("15")) // White/Bright

	leftArrow := "←"
	if m.CurrentPage == 1 {
		leftArrow = lipgloss.NewStyle().Faint(true).Render("←")
	} else {
		leftArrow = arrowStyle.Render("← (H)")
	}

	rightArrow := "→"
	if m.CurrentPage == totalPages {
		rightArrow = lipgloss.NewStyle().Faint(true).Render("→")
	} else {
		rightArrow = arrowStyle.Render("(L) →")
	}

	paginationControls := lipgloss.JoinHorizontal(lipgloss.Center,
		lipgloss.NewStyle().Width(8).Align(lipgloss.Right).Render(leftArrow),
		lipgloss.NewStyle().Margin(0, 2).Render(pageInfo),
		lipgloss.NewStyle().Width(8).Align(lipgloss.Left).Render(rightArrow),
	)

	// Removed Bold(true) from the title style
	title := lipgloss.NewStyle().Padding(1, 0).Render("GLOBAL HIGH SCORES")
	instruction := lipgloss.NewStyle().Faint(true).Margin(1, 0).Render("Press ESC or ENTER to return.")

	finalContent := lipgloss.JoinVertical(lipgloss.Center,
		title,
		tableContent.String(),
		lipgloss.NewStyle().Padding(1, 0).Render(paginationControls),
		instruction,
	)

	return lipgloss.Place(m.ScreenWidth, m.ScreenHeight,
		lipgloss.Center, lipgloss.Center,
		lipgloss.NewStyle().Border(lipgloss.ThickBorder()).Render(finalContent),
	)
}
