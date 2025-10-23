package game

import (
	"sync"
	"time"
)

type BotMaster struct {
	ControlledPlayers map[int]*Bot
	IsRunning         bool
	GameMainManager   *GameManager
}

func NewBotMaster(gm *GameManager) *BotMaster {
	return &BotMaster{
		ControlledPlayers: make(map[int]*Bot),
		IsRunning:         false,
		GameMainManager:   gm,
	}
}

func (bm *BotMaster) StartBotFleet() {
	if bm.IsRunning {
		return
	}

	bm.IsRunning = true
	// we see if direction switch is needed per for every 120
	duration := 120 * time.Millisecond

	ticker := time.NewTicker(duration)
	defer ticker.Stop()

	for bm.IsRunning {
		<-ticker.C
		bm.processBots()
	}
}

func (bm *BotMaster) processBots() {
	var wg sync.WaitGroup
	for _, bot := range bm.ControlledPlayers {
		if bot == nil {
			continue
		}

		wg.Add(1)
		// Process each bot's direction decision in a separate goroutine
		go func(b *Bot) {
			defer wg.Done()
			nextDirection := b.BotStrategy.getNextBestDirection(b.Player, bm.GameMainManager)
			if nextDirection.Dx != b.CurrentDirection.Dx || nextDirection.Dy != b.CurrentDirection.Dy {
				bm.GameMainManager.DirectionChannel <- nextDirection
			}

		}(bot)
	}
	wg.Wait()
}
