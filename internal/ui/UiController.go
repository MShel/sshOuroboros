package ui

import (
	"strconv"

	"github.com/Mshel/sshnake/internal/game"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
)

type Screen int

const (
	IntroScreen Screen = iota
	SetupScreen
	GameScreen
)

// Messages for state transitions
type IntroSubmitMsg int // 0 for Register, 1 for Leaderboard
type SetupSubmitMsg struct {
	Name  string
	Color string
}

type ShowLeaderboardMsg struct{}

type ControllerModel struct {
	CurrentScreen Screen
	GameManager   *game.GameManager

	IntroModel tea.Model
	SetupModel tea.Model
	GameModel  tea.Model

	CurrentUserSession ssh.Session
	ScreenWidth        int
	ScreenHeight       int
}

func NewControllerModel(gameManager *game.GameManager, userSession ssh.Session, screenWidth int, screenHeight int) ControllerModel {
	return ControllerModel{
		GameManager:   gameManager,
		CurrentScreen: IntroScreen,

		IntroModel: NewIntroModel(screenWidth, screenHeight),
		SetupModel: NewInitialSetupModel(gameManager, screenWidth, screenHeight),

		CurrentUserSession: userSession,
		ScreenWidth:        screenWidth,
		ScreenHeight:       screenHeight,
	}
}

func (m ControllerModel) Init() tea.Cmd {
	return m.IntroModel.Init()
}

func (m ControllerModel) View() string {
	switch m.CurrentScreen {
	case IntroScreen:
		return m.IntroModel.View()
	case SetupScreen:
		return m.SetupModel.View()
	case GameScreen:
		if m.GameModel != nil {
			return m.GameModel.View()
		}
		return "Game Loading..."
	default:
		return "Unknown Screen"
	}
}

func (m ControllerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	// --- 1. Global Key Check (Check before the main switch) ---
	if msg, ok := msg.(tea.KeyMsg); ok {
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			return m, tea.Quit
		}
	}

	// --- 2. State Transition Message Handling ---
	switch msg := msg.(type) {
	case IntroSubmitMsg:
		if msg == 0 {
			// Start Registration
			m.CurrentScreen = SetupScreen
			return m, m.SetupModel.Init()
		} else if msg == 1 {
			// View Leaderboard
			m.CurrentScreen = GameScreen
			m.GameModel = NewGameModel(m.GameManager, nil, m.ScreenWidth, m.ScreenHeight)

			// Init the model, then immediately send the ShowLeaderboardMsg
			return m, tea.Sequence(m.GameModel.Init(), func() tea.Msg { return ShowLeaderboardMsg{} })
		}

	case SetupSubmitMsg:
		m.CurrentScreen = GameScreen
		color, conversionErr := strconv.Atoi(msg.Color)
		if conversionErr != nil {
			return m, tea.Quit
		}

		m.GameManager.CreateNewPlayer(msg.Name, color, m.CurrentUserSession)
		m.GameModel = NewGameModel(m.GameManager, m.CurrentUserSession, m.ScreenWidth, m.ScreenHeight)
		return m, m.GameModel.Init()

	case QuitGameMsg:
		// Transition from Leaderboard back to IntroScreen
		m.CurrentScreen = IntroScreen
		return m, m.IntroModel.Init()

	default:
		// --- 3. Message Delegation (Pass to the active model for all other messages) ---
		switch m.CurrentScreen {
		case IntroScreen:
			m.IntroModel, cmd = m.IntroModel.Update(msg)
			cmds = append(cmds, cmd)
		case SetupScreen:
			m.SetupModel, cmd = m.SetupModel.Update(msg)
			cmds = append(cmds, cmd)
		case GameScreen:
			if m.GameModel != nil {
				m.GameModel, cmd = m.GameModel.Update(msg)
				cmds = append(cmds, cmd)
			}
		}
	}

	return m, tea.Batch(cmds...)
}
