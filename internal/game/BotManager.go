package game

import (
	"context"
	"log"
	"time"
)

type BotMaster struct {
	ControlledPlayers    map[int]*Bot
	IsRunning            bool
	GameMainManager      *GameManager
	BotProcessingChannel chan *Bot
	shutdownCtx          context.Context // Add this
}

func NewBotMaster(gm *GameManager, gameContext context.Context) *BotMaster {
	return &BotMaster{
		ControlledPlayers:    make(map[int]*Bot),
		IsRunning:            false,
		GameMainManager:      gm,
		BotProcessingChannel: make(chan *Bot, 5000),
		shutdownCtx:          gameContext,
	}
}

func (bm *BotMaster) StartBotFleet() {
	if bm.IsRunning {
		return
	}

	botWorkersCount := 30
	for w := 1; w <= botWorkersCount; w++ {
		go bm.processBot()
	}

	bm.IsRunning = true
	duration := 200 * time.Millisecond
	ticker := time.NewTicker(duration)
	defer ticker.Stop()

	for bm.IsRunning {
		select {
		case <-ticker.C:
			bm.processBots()
		case <-bm.shutdownCtx.Done():
			log.Println("Bot Master main loop stopped.")
			return
		}
	}
}

// Implement a stop function
func (bm *BotMaster) StopBotFleet() {
	if !bm.IsRunning {
		return
	}
	bm.IsRunning = false
	close(bm.BotProcessingChannel)
}

func (bm *BotMaster) processBot() {
	for bot := range bm.BotProcessingChannel {
		bm.updateBotLocation(bot)
	}
}

func (bm *BotMaster) updateBotLocation(bot *Bot) {
	nextDirection := bot.BotStrategy.getNextBestDirection(bot.Player, bm.GameMainManager)
	if nextDirection.Dx != bot.CurrentDirection.Dx || nextDirection.Dy != bot.CurrentDirection.Dy {
		bot.Player.UpdateDirection(nextDirection)
	}
}

func (bm *BotMaster) processBots() {
	for _, bot := range bm.ControlledPlayers {
		if bot.Player == nil {
			continue
		}

		bm.BotProcessingChannel <- bot
	}
}
