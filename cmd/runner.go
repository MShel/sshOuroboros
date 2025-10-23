package main

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"

	"github.com/Mshel/sshnake/internal/game"
	"github.com/Mshel/sshnake/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	gameManager := game.GetNewGameManager()

	botMaster := game.NewBotMaster(gameManager)
	for i := range 10 {
		botPlayer := gameManager.CreateNewPlayer("derp "+strconv.Itoa(i), rand.Intn(255))
		botMaster.ControlledPlayers[*botPlayer.Color] = &game.Bot{Player: botPlayer, BotStrategy: game.AgresssorStrategy}
	}

	go botMaster.StartBotFleet()

	p := tea.NewProgram(ui.NewControllerModel(gameManager), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("error %v", err)
		os.Exit(1)
	}
}
