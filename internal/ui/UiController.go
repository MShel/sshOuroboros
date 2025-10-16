package ui

import tea "github.com/charmbracelet/bubbletea"

type Screen int

const (
	SetupScreen Screen = iota // 0: The initial form (your current UI)
	GameScreen                // 1: The main game viewport, score, etc.
)

type ControllerModel struct {
	CurrentScreen Screen

	SetupModel tea.Model
	GameModel  tea.Model
}

func NewControllerModel() ControllerModel {
	return ControllerModel{
		CurrentScreen: SetupScreen,
		SetupModel:    NewInitialSetupModel(),
		GameModel:     NewGameModel(), // GameModel is initialized only upon submission
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

// Custom message to carry the submitted data from the setup screen.
type SetupSubmitMsg struct {
	Name  string
	Color string
}

// In ControllerModel's Update()
func (m ControllerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle global keys (like Ctrl+C to quit from any screen)
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

	case SetupSubmitMsg:
		// 2. Transition to the next screen
		m.CurrentScreen = GameScreen

		// 3. (Optional) Initialize the new GameModel with the data
		m.GameModel = NewGameModel()

		// The transition is done.
		return m, nil
	}

	// Pass messages down to the active child model
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
