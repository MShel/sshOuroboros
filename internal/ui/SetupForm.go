package ui

import (
	"fmt"
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

// 256 color setup
var colorOptions []string

func init() {
	// Populate 256 color codes (0 to 255) as strings
	for i := 0; i < 256; i++ {
		colorOptions = append(colorOptions, strconv.Itoa(i))
	}
}

// Model for our form
type SetupModel struct {
	nameInput  textinput.Model
	colorIndex int // Index of the selected color (0-255)
	focusIndex int // 0: Name, 1: Color Select, 2: Submit/Quit
	submitted  bool
	width      int // Terminal width for wrapping
	height     int // Terminal height
	tea.Model
}

func NewInitialSetupModel(gameManager *game.GameManager, w, h int) SetupModel {
	ti := textinput.New()
	ti.Placeholder = "Your Orboros Name"
	ti.Focus()
	ti.CharLimit = 20
	ti.Width = 200
	ti.PromptStyle = focusedStyle
	ti.TextStyle = focusedStyle

	return SetupModel{
		nameInput:  ti,
		colorIndex: 0,
		focusIndex: 0,
		submitted:  false,
		width:      w, // FIX: Initialize width
		height:     h, // Initialize height
	}
}

// Init sends a command to start the cursor blinking
func (m SetupModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages (key presses, window resizes, etc.)
func (m SetupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
							Color: colorOptions[m.colorIndex],
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
				m.colorIndex = (m.colorIndex - swatchesPerLine + len(colorOptions)) % len(colorOptions)
				keyConsumed = true
			case "down":
				m.colorIndex = (m.colorIndex + swatchesPerLine) % len(colorOptions)
				keyConsumed = true
			case "left":
				m.colorIndex = (m.colorIndex - 1 + len(colorOptions)) % len(colorOptions)
				keyConsumed = true
			case "right":
				m.colorIndex = (m.colorIndex + 1) % len(colorOptions)
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

// View returns the string representation of the UI
func (m SetupModel) View() string {
	if m.submitted {
		selectedColorCode := colorOptions[m.colorIndex]

		// Render a large block of the selected color for confirmation
		colorBlock := lipgloss.NewStyle().
			Background(lipgloss.Color(selectedColorCode)).
			Width(20).
			Height(3).
			Align(lipgloss.Center).
			Render(lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Render(selectedColorCode))

		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, fmt.Sprintf(
			"✅ Form Submitted!\n\nName: %s\nColor Code: %s\n\n%s\n\n",
			m.nameInput.Value(),
			selectedColorCode,
			colorBlock,
		))
	}

	var b strings.Builder

	b.WriteString(m.nameInput.View())
	b.WriteString("\n\n")

	snakeColorPrompt := "Select your sshnake color(use arrows)"
	if m.focusIndex == 1 {
		b.WriteString(focusedStyle.Render(snakeColorPrompt))
	} else {
		b.WriteString(blurredStyle.Render(snakeColorPrompt))
	}
	b.WriteString("\n")

	// FIX: This calculation now uses the initialized width
	swatchesPerLine := (m.width - 2) / 2
	if swatchesPerLine < 1 {
		swatchesPerLine = 1
	}

	var colorSwatches strings.Builder
	var selectedColorCode string
	for i, colorCode := range colorOptions {
		style := colorSwatchStyle.
			Background(lipgloss.Color(colorCode))

		if i == m.colorIndex {
			colorSwatches.WriteString(style.Foreground(lipgloss.Color("15")).Render("█"))
			selectedColorCode = colorCode
		} else {
			colorSwatches.WriteString(style.Foreground(lipgloss.Color(colorCode)).Render("░"))
		}

		if (i+1)%swatchesPerLine == 0 && i < len(colorOptions)-1 {
			colorSwatches.WriteString("\n")
		}
	}
	b.WriteString(colorSwatches.String())
	b.WriteString("\n")
	b.WriteString("Sshnake color " + selectedColorStyle.
		Foreground(lipgloss.Color(selectedColorCode)).
		Render("██"))

	b.WriteString("\n")
	b.WriteString("\n")

	// --- 3. Submit Button ---
	submitText := "Submit"
	if m.focusIndex == 2 {
		b.WriteString(submitButtonStyle.Render(submitText))
	} else {
		b.WriteString(blurredButtonStyle.Padding(0, 1).Render(submitText))
	}
	b.WriteString("\n\n")

	// --- Help Text ---
	b.WriteString(helpStyle.Render("(arrows to select color, tab/shift+tab to navigate, enter to confirm, ctrl+c to quit)"))

	// Center the content using the correct width/height
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, b.String())
}
