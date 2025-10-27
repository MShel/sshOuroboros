package ui

import (
	"strconv"

	"github.com/Mshel/sshnake/internal/game"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
)

type Screen int

const (
	SetupScreen Screen = iota
	GameScreen
)

type ControllerModel struct {
	CurrentScreen      Screen
	GameManager        *game.GameManager
	SetupModel         tea.Model
	GameModel          tea.Model
	CurrentUserSession ssh.Session
	ScreenWidth        int
	ScreenHeight       int
}

func NewControllerModel(gameManager *game.GameManager, userSession ssh.Session, screenWidth int, screenHeight int) ControllerModel {
	return ControllerModel{
		GameManager:        gameManager,
		CurrentScreen:      SetupScreen,
		SetupModel:         NewInitialSetupModel(gameManager),
		GameModel:          nil,
		CurrentUserSession: userSession,
		ScreenWidth:        screenWidth,
		ScreenHeight:       screenHeight,
	}
}

// Init implements tea.Model.
func (m ControllerModel) Init() tea.Cmd {
	return m.SetupModel.Init()
}

// View implements tea.Model.
func (m ControllerModel) View() string {
	switch m.CurrentScreen {
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

type SetupSubmitMsg struct {
	Name  string
	Color string
}

func (m ControllerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

	case SetupSubmitMsg:
		m.CurrentScreen = GameScreen
		color, conversionErr := strconv.Atoi(msg.Color)
		if conversionErr != nil {
			panic(conversionErr)
		}

		m.GameManager.CreateNewPlayer(msg.Name, color, m.CurrentUserSession)
		m.GameModel = NewGameModel(m.GameManager, m.CurrentUserSession, m.ScreenWidth, m.ScreenHeight)

		go m.GameManager.StartGameLoop()

		return m, m.GameModel.Init()
	}

	switch m.CurrentScreen {
	case SetupScreen:
		m.SetupModel, cmd = m.SetupModel.Update(msg)
		cmds = append(cmds, cmd)
	case GameScreen:
		m.GameModel, cmd = m.GameModel.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}
