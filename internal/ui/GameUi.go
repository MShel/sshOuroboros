package ui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type GameModel struct {
	tea.Model
	TickCount int
}

type GameTickMsg struct{}

func (m GameModel) Init() tea.Cmd {
	return tea.Tick(100, func(t time.Time) tea.Msg { return GameTickMsg{} })
}

func (m GameModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case GameTickMsg:
		m.TickCount++
		// Continue ticking
		return m, tea.Tick(100, func(t time.Time) tea.Msg { return GameTickMsg{} })
	}
	return m, nil
}

func (m GameModel) View() string {
	return lipgloss.NewStyle().Background(lipgloss.Color("158")).
		Render(fmt.Sprintf(
			"Welcome to Ssshnake, \n Game Ticks: %d\n(This is the main game screen.)\nPress Ctrl+C to exit.",
			m.TickCount,
		))
}

func NewGameModel() GameModel {
	gameModel := GameModel{
		TickCount: 0,
	}
	return gameModel
}
