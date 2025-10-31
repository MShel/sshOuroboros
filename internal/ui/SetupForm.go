package ui

import (
	"strconv"
	"strings"

	"github.com/Mshel/sshnake/internal/game"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Define styles
var (
	focusedColor = lipgloss.Color("205") // Bright Pink/Purple
	blurredColor = lipgloss.Color("240")
	focusedStyle = lipgloss.NewStyle().Foreground(focusedColor)
	blurredStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	helpStyle    = blurredStyle

	// This style ensures every swatch has a width of 1 character column for alignment.
	colorSwatchStyle   = lipgloss.NewStyle().Width(1)
	selectedColorStyle = lipgloss.NewStyle().Width(2)
	buttonStyle        = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder())

	submitButtonStyle = buttonStyle.
				BorderForeground(focusedColor).
				Padding(0, 1)

	blurredButtonStyle = buttonStyle.
				BorderForeground(blurredColor).
				Padding(0, 1)
)

// Model for our form
type SetupModel struct {
	nameInput    textinput.Model
	colorIndex   int // Index of the selected color (0-255)
	focusIndex   int // 0: Name, 1: Color Select, 2: Submit/Quit
	submitted    bool
	width        int // Terminal width for wrapping
	height       int // Terminal height
	gameManager  *game.GameManager
	colorOptions []string
	tea.Model
}

func NewInitialSetupModel(gameManager *game.GameManager, w, h int) SetupModel {
	ti := textinput.New()
	ti.Placeholder = "Your Orboros Name"
	ti.Focus()
	ti.CharLimit = 20
	ti.PromptStyle = focusedStyle
	ti.TextStyle = focusedStyle

	setupModel := SetupModel{
		nameInput:   ti,
		colorIndex:  0,
		focusIndex:  0,
		submitted:   false,
		width:       w, // FIX: Initialize width
		height:      h, // Initialize height
		gameManager: gameManager,
	}

	return setupModel
}

// Init sends a command to start the cursor blinking
func (m SetupModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages (key presses, window resizes, etc.)
func (m SetupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	colorOptions := []string{}
	m.gameManager.Players.Range(func(key, value interface{}) bool {
		if player, ok := value.(*game.Player); ok && player != nil {
			if player.BotStrategy != nil {
				colorOptions = append(colorOptions, strconv.Itoa(*player.Color))
			}
		}
		return true
	})
	m.colorOptions = colorOptions

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		s := msg.String()

		// 1. Handle Quit (always works)
		if s == "ctrl+c" {
			return m, tea.Quit
		}

		// 2. Handle Focus Navigation (Tab/Shift+Tab/Enter)
		if s == "enter" || s == "tab" || s == "shift+tab" {
			switch m.focusIndex {
			case 0: // Name Input
				switch s {
				case "enter", "tab":
					m.focusIndex = 1 // Move to Color Select
					m.nameInput.Blur()
				case "shift+tab":
					m.focusIndex = 2 // Move to Submit
				}

			case 1: // Color Select
				switch s {
				case "enter", "tab":
					m.focusIndex = 2 // Move to Submit
				case "shift+tab":
					m.focusIndex = 0 // Move to Name Input
					m.nameInput.Focus()
				}

			case 2: // Submit Button
				switch s {
				case "enter":
					m.submitted = true
					return m, func() tea.Msg {
						return SetupSubmitMsg{
							Name:  m.nameInput.Value(),
							Color: m.colorOptions[m.colorIndex],
						}
					}
				case "tab":
					m.focusIndex = 0 // Cycle to Name Input
					m.nameInput.Focus()
				case "shift+tab":
					m.focusIndex = 1 // Move to Color Select
				}
			}
			return m, nil
		}

		// 3. Handle Color Selection Navigation (Arrows, only when focused on colors)
		if m.focusIndex == 1 {
			var keyConsumed bool

			// Calculate swatches per line (2 columns per swatch: 1 for block, 1 for small visual space/margin)
			swatchesPerLine := (m.width - 2) / 2
			if swatchesPerLine < 1 {
				swatchesPerLine = 1
			}

			switch s {
			case "up":
				m.colorIndex = (m.colorIndex - swatchesPerLine + len(m.colorOptions)) % len(m.colorOptions)
				keyConsumed = true
			case "down":
				m.colorIndex = (m.colorIndex + swatchesPerLine) % len(m.colorOptions)
				keyConsumed = true
			case "left":
				m.colorIndex = (m.colorIndex - 1 + len(m.colorOptions)) % len(m.colorOptions)
				keyConsumed = true
			case "right":
				m.colorIndex = (m.colorIndex + 1) % len(m.colorOptions)
				keyConsumed = true
			}

			// If a directional key was pressed, the event is consumed, and we must return.
			if keyConsumed {
				return m, nil
			}
		}

		// 4. Handle remaining keys by passing them to the focused text input.
		if m.focusIndex == 0 {
			var cmd tea.Cmd
			m.nameInput, cmd = m.nameInput.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m SetupModel) View() string {
	// Helper to center content within the terminal width
	center := func(s string) string {
		return lipgloss.NewStyle().Width(m.width).Align(lipgloss.Center).Render(s)
	}

	var b strings.Builder

	// Name Input
	b.WriteString(center(m.nameInput.View()))
	b.WriteString("\n\n")

	// Color Prompt
	orborusColorPrompt := "Select your sshnake color(use arrows)"
	var colorPrompt string
	if m.focusIndex == 1 {
		colorPrompt = focusedStyle.Render(orborusColorPrompt)
	} else {
		colorPrompt = blurredStyle.Render(orborusColorPrompt)
	}
	b.WriteString(center(colorPrompt))
	b.WriteString("\n")

	var colorSwatches strings.Builder
	var selectedColorCode string
	colorsPerLine := 100

	for i, colorCode := range m.colorOptions {
		style := colorSwatchStyle.
			Background(lipgloss.Color(colorCode))

		if i == m.colorIndex {
			colorSwatches.WriteString(style.Foreground(lipgloss.Color("15")).Render("█"))
			selectedColorCode = colorCode
		} else {
			colorSwatches.WriteString(style.Foreground(lipgloss.Color(colorCode)).Render("░"))
		}

		if (i+1)%colorsPerLine == 0 && i < len(m.colorOptions)-1 {
			colorSwatches.WriteString("\n")
		}
	}
	b.WriteString(center(colorSwatches.String()))
	b.WriteString("\n")

	// Selected Color Indicator
	b.WriteString(center("Ourboros color " + selectedColorStyle.
		Foreground(lipgloss.Color(selectedColorCode)).
		Render("██")))

	b.WriteString("\n")
	b.WriteString("\n")

	// Submit Button
	submitText := "Submit"
	var submitButton string
	if m.focusIndex == 2 {
		submitButton = submitButtonStyle.Render(submitText)
	} else {
		submitButton = blurredButtonStyle.Padding(0, 1).Render(submitText)
	}
	b.WriteString(center(submitButton))
	b.WriteString("\n\n")

	// Help Text
	b.WriteString(center(helpStyle.Render("(arrows to select color, tab/shift+tab to navigate, enter to confirm, ctrl+c to quit)")))

	// Final centering (this centers the whole block vertically, but the individual lines are now centered horizontally)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, b.String())
}
