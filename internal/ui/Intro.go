package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// IntroModel holds the state for the main menu.
type IntroModel struct {
	selected int // 0: Start Registration, 1: View Leaderboard
	width    int
	height   int
}

func NewIntroModel(w, h int) IntroModel {
	return IntroModel{selected: 0, width: w, height: h}
}

func (m IntroModel) Init() tea.Cmd { return nil }

func (m IntroModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "left", "h":
			// Select the previous button: 0 -> 1, 1 -> 0
			if m.selected == 0 {
				m.selected = 1
			} else {
				m.selected = 0
			}
		case "right", "l":
			// Select the next button: 0 -> 1, 1 -> 0
			if m.selected == 0 {
				m.selected = 1
			} else {
				m.selected = 0
			}
		case "enter":
			// Submit the selected option
			return m, func() tea.Msg { return IntroSubmitMsg(m.selected) }
		}
	}
	return m, nil
}

var ouroborosAscii = `
               ██████████████                                      ██████████████                 
            ██████          ██████                            ████████          ██████            
         ████                    ████                      ████                      ████         
     ███                               ███            ███                                 ███     
    ███        ██████████████            ███    ███████            ██████████████          ███    
   ██       ████            ████           █████                ████            ████         ██   
  ██      ███                  ████     ████                 ███                   ███        ██  
 ██      ██                       ███ ███                  ███                       ██        ██ 
 ██     ██                          ███   ██             ███                          ██       ██ 
██     ███                         ███  █ ██       ██   ██                            ███       ██
██     ██                         ██            ████    ██                             ██       ██
██     ██                        ██          ██████    ██                              ██       ██
██     ██                       ██ █     ████    ██   ███                              ██       ██
██     ███                      █     ████      ██   █████                             ██       ██
 ██     ██                      ███ █████      ██   ██   ███                          ██       ██ 
 ██     ██                       ████████    ██  ███      ███                       ██        ██ 
  ██      ███                   ███       █  ██████          ███                   ███        ██  
   ██      ████            ████           █████  ███           ████            ████         ██   
    ██        ████████████            ███        ███            ██████████████          ███    
     ███                             ███            ███                              ███     
         ████                      ████                ████                      ████         
            ██████          ██████                         ██████            ████           
                 ████████████                                    ███████████                
`

var (
	asciiStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("87"))

	introButtonStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("0")).
				Padding(0, 3).
				Margin(1, 2).
				Border(lipgloss.RoundedBorder())

	introSelectedButtonStyle = introButtonStyle.
					Background(lipgloss.Color("87")).
					Foreground(lipgloss.Color("0"))
)

func (m IntroModel) View() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(asciiStyle.Render(ouroborosAscii))
	sb.WriteString("\n")

	register := introButtonStyle.Render("Start Registration")
	leaderboard := introButtonStyle.Render("View Leaderboard")

	// Apply selected style based on m.selected
	if m.selected == 0 {
		register = introSelectedButtonStyle.Render("Start Registration")
	} else {
		leaderboard = introSelectedButtonStyle.Render("View Leaderboard")
	}

	buttons := lipgloss.JoinHorizontal(lipgloss.Center, register, leaderboard)

	content := lipgloss.JoinVertical(lipgloss.Center, sb.String(), buttons)

	// Center the entire view within the terminal
	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		content,
	)
}
