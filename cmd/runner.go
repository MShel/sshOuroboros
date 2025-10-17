package main

import (
	"fmt"
	"os"

	"github.com/Mshel/sshnake/internal/game"
	"github.com/Mshel/sshnake/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	gameManager := game.GetNewGameManager()
	p := tea.NewProgram(ui.NewControllerModel(gameManager), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("error %v", err)
		os.Exit(1)
	}
}
