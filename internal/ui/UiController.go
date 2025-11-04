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
	GameOverScreen
	LeaderboardScreen
)

// Messages for state transitions
type IntroSubmitMsg int // 0 for Register, 1 for Leaderboard
type SetupSubmitMsg struct {
	Name  string
	Color string
}

type ControllerModel struct {
	CurrentScreen Screen
	GameManager   *game.GameManager

	IntroModel       tea.Model
	SetupModel       tea.Model
	GameModel        tea.Model
	GameOverModel    tea.Model
	LeaderboardModel tea.Model

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
	case GameOverScreen:
		if m.GameOverModel != nil {
			return m.GameOverModel.View()
		}
		return "Game Over Loading..."
	case LeaderboardScreen:
		if m.LeaderboardModel != nil {
			return m.LeaderboardModel.View()
		}
		return "Leaderboard Loading..."
	default:
		return "Unknown Screen"
	}
}

func (m ControllerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	if msg, ok := msg.(tea.KeyMsg); ok {
		if msg.String() == "ctrl+c" {

			if m.CurrentUserSession != nil {
				if anyPlayer, ok := m.GameManager.SessionsToPlayers.Load(m.CurrentUserSession); ok {
					if anyPlayer != nil {
						m.GameManager.SessionsToPlayers.Delete(m.CurrentUserSession)
					}
				}
			}

			return m, tea.Quit
		}
	}

	switch msg := msg.(type) {
	case IntroSubmitMsg:
		switch msg {
		case 0:
			m.CurrentScreen = SetupScreen
			return m, m.SetupModel.Init()
		case 1:
			m.CurrentScreen = LeaderboardScreen
			tempGameModel := NewGameModel(m.GameManager, nil, m.ScreenWidth, m.ScreenHeight)
			initialData := tempGameModel.calculateLeaderboard()
			m.LeaderboardModel = NewLeaderboardModel(m.GameManager, map[*int]int{}, initialData, m.ScreenWidth, m.ScreenHeight)
			return m, m.LeaderboardModel.Init()
		}

	case ShowGameOverMsg:
		m.CurrentScreen = GameOverScreen
		m.GameOverModel = NewGameOverModel(
			m.GameManager,
			msg.FinalEstate,
			msg.FinalKills,
			msg.LeaderboardData,
			msg.EstateInfo,
			m.ScreenWidth,
			m.ScreenHeight,
		)
		return m, m.GameOverModel.Init()

	case ShowLeaderboardFromGameOverMsg:
		m.CurrentScreen = LeaderboardScreen
		m.LeaderboardModel = NewLeaderboardModel(m.GameManager, msg.EstateInfo, msg.LeaderboardData, m.ScreenWidth, m.ScreenHeight)
		return m, m.LeaderboardModel.Init()

	case ReturnFromLeaderboardMsg:
		if m.CurrentUserSession != nil {
			m.CurrentScreen = GameOverScreen
			return m, nil
		}

		m.CurrentScreen = IntroScreen
		m.CurrentUserSession = nil
		return m, m.IntroModel.Init()

	case ReturnToIntroMsg:
		m.CurrentScreen = IntroScreen
		m.CurrentUserSession = nil
		return m, m.IntroModel.Init()

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
		return m, tea.Quit

	default:
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
		case GameOverScreen:
			if m.GameOverModel != nil {
				m.GameOverModel, cmd = m.GameOverModel.Update(msg)
				cmds = append(cmds, cmd)
			}
		case LeaderboardScreen:
			if m.LeaderboardModel != nil {
				m.LeaderboardModel, cmd = m.LeaderboardModel.Update(msg)
				cmds = append(cmds, cmd)
			}
		}
	}

	return m, tea.Batch(cmds...)
}
