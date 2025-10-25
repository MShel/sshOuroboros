package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/Mshel/sshnake/internal/game"
	"github.com/Mshel/sshnake/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
)

func mainWithoutServer() {
	gameManager := game.GetNewGameManager()

	botMaster := game.NewBotMaster(gameManager, gameManager.GameContext)
	for i := range 10 {
		botPlayer := gameManager.CreateNewPlayer("derp "+strconv.Itoa(i), i+1)
		botMaster.ControlledPlayers[*botPlayer.Color] = &game.Bot{Player: botPlayer, BotStrategy: game.AgresssorStrategy}
	}

	go botMaster.StartBotFleet()

	f, err := tea.LogToFile("debug.log", "debug")
	if err != nil {
		fmt.Println("fatal:", err)
		os.Exit(1)
	}
	defer f.Close()

	p := tea.NewProgram(ui.NewControllerModel(gameManager, 0, 0), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("error %v", err)
		os.Exit(1)
	}
}
