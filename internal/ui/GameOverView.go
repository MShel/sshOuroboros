package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Mshel/sshnake/internal/game"
	"github.com/charmbracelet/lipgloss"
)

// GameOverState holds the data and local state for rendering the game over screens.
type GameOverState struct {
	GameManager    *game.GameManager
	FinalEstate    int
	FinalKills     int
	SelectedButton int
	ScreenWidth    int
	ScreenHeight   int
}

// Styles for Game Over/Leaderboard
var (
	GameOverbuttonStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("0")).
				Padding(0, 3).
				Margin(1, 1).
				Bold(true)

	selectedButtonStyle = GameOverbuttonStyle.
				Background(lipgloss.Color("4")). // Red background for selection
				Foreground(lipgloss.Color("15")) // White/Bright text

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

// RenderGameOverScreen draws the death message and buttons.
func (g *GameOverState) RenderGameOverScreen() string {
	messageStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("9")).
		Padding(2, 5).
		Align(lipgloss.Center).
		Width(g.ScreenWidth - 4)

	title := messageStyle.Render("💀 G A M E   O V E R 💀")

	stats := fmt.Sprintf("\nFinal Stats:\nEstate Claimed: %d tiles\nPlayer Kills: %d\n\n", g.FinalEstate, g.FinalKills)

	exitButton := buttonStyle.Render("EXIT (Enter)")
	leaderboardButton := buttonStyle.Render("LEADERBOARD")

	if g.SelectedButton == 0 {
		exitButton = selectedButtonStyle.Render("EXIT (Enter)")
	} else {
		leaderboardButton = selectedButtonStyle.Render("LEADERBOARD")
	}

	buttons := lipgloss.JoinHorizontal(lipgloss.Center, exitButton, leaderboardButton)

	content := lipgloss.JoinVertical(lipgloss.Center, title, stats, buttons)

	// Center the content on the screen
	return lipgloss.Place(g.ScreenWidth, g.ScreenHeight,
		lipgloss.Center, lipgloss.Center,
		lipgloss.NewStyle().Border(lipgloss.ThickBorder()).Render(content),
	)
}

// RenderLeaderboardScreen draws the current leaderboard table.
func (g *GameOverState) RenderLeaderboardScreen(estateInfo map[*int]int) string {
	var tableContent strings.Builder

	type PlayerScore struct {
		Name   string
		Color  int
		Estate int
	}

	var scores []PlayerScore
	// Collect scores from the game manager and the passed estate info
	g.GameManager.Players.Range(func(key, value interface{}) bool {
		player := value.(*game.Player)
		if player != nil {
			estate, found := estateInfo[player.Color]
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

	// NOTE: Add sorting logic here if desired!

	// Define column widths for alignment
	nameWidth := 15
	estateWidth := 10

	// --- Header ---
	header := lipgloss.JoinHorizontal(lipgloss.Top,
		leaderboardHeaderStyle.Width(3).Render("#"),
		leaderboardHeaderStyle.Width(nameWidth).Render("Player"),
		leaderboardHeaderStyle.Width(estateWidth).Render("Estate"),
	)
	tableContent.WriteString(header + "\n")

	// --- Rows ---
	for i, score := range scores {
		rank := i + 1

		// Use the player's color for their row text
		colorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(strconv.Itoa(score.Color)))

		row := lipgloss.JoinHorizontal(lipgloss.Top,
			leaderboardRowStyle.Width(3).Render(strconv.Itoa(rank)),
			colorStyle.Copy().Width(nameWidth).Render(score.Name),
			leaderboardRowStyle.Width(estateWidth).Render(strconv.Itoa(score.Estate)),
		)

		tableContent.WriteString(leaderboardBorderStyle.Render(row) + "\n")
	}

	// --- Title & Instructions ---
	title := lipgloss.NewStyle().Bold(true).Padding(1, 0).Render("👑 CURRENT LEADERBOARD 👑")
	instruction := lipgloss.NewStyle().Faint(true).Margin(1, 0).Render("Press ESC or ENTER to return to Game Over screen.")

	finalContent := lipgloss.JoinVertical(lipgloss.Center,
		title,
		tableContent.String(),
		instruction,
	)

	return lipgloss.Place(g.ScreenWidth, g.ScreenHeight,
		lipgloss.Center, lipgloss.Center,
		lipgloss.NewStyle().Border(lipgloss.ThickBorder()).Render(finalContent),
	)
}
