package game

import (
	"log"
)

type PlayerManager struct {
	SunsetPlayersChannel chan *Player
	PlayerRebirth        chan int

	HighScoreService *HighScoreService
	GameManager      *GameManager
}

var playerManager *PlayerManager

func NewPlayerManager(gameManager *GameManager) *PlayerManager {
	if playerManager != nil {
		return playerManager
	}

	playerManager = &PlayerManager{
		SunsetPlayersChannel: make(chan *Player, 1),
		PlayerRebirth:        make(chan int, 1),
		HighScoreService:     NewHighScoreService(),
		GameManager:          gameManager,
	}

	for w := 1; w <= sunsetWorkersCount; w++ {
		go playerManager.sunsetPlayersWorker()
	}

	for w := 1; w <= rebirthWorkerCount; w++ {
		go playerManager.rebirthPlayersWorker()
	}

	return playerManager
}

func (sunsetterInst *PlayerManager) sunsetPlayersWorker() {
	for {
		player, ok := <-sunsetterInst.SunsetPlayersChannel
		if !ok {
			return
		}
		if player != nil {
			sunsetterInst.sunsetPlayer(player, true)
		}
	}
}

func (playerManagerInst *PlayerManager) sunsetPlayer(player *Player, needRebirth bool) {
	playerFinalClaimedLand := 0.0
	player.AllTiles.allTilesLock.Lock()
	player.Tail.tailLock.Lock()
	defer player.AllTiles.allTilesLock.Unlock()
	defer player.Tail.tailLock.Unlock()

	for _, tile := range player.AllTiles.AllPlayerTiles {
		if tile.OwnerColor == player.Color {
			playerFinalClaimedLand += 1.0
			tile.OwnerColor = nil
		}
	}
	for _, tile := range player.Tail.tailTiles {
		if tile.OwnerColor == player.Color {
			tile.OwnerColor = nil
		}
	}

	player.Location.IsTail = false
	player.Location.OwnerColor = nil

	if player.SshSession != nil {
		highScoreError := playerManagerInst.HighScoreService.SavePlayersHighScore(
			player.Name,
			*player.Color,
			(playerFinalClaimedLand*100)/float64(MapColCount*MapRowCount),
			player.Kills,
		)

		if highScoreError != nil {
			log.Printf("High score persist err: %v ", highScoreError)
		}

		player.UpdateChannel <- PlayerDeadMsg{
			PlayerColor:        *player.Color,
			FinalClaimedEstate: playerFinalClaimedLand,
			FinalKills:         player.Kills,
		}
	}

	playerManagerInst.GameManager.Players.Delete(*player.Color)

	if needRebirth {
		playerManagerInst.PlayerRebirth <- *player.Color
	}
}

func (playerManagerInst *PlayerManager) rebirthPlayersWorker() {
	for {
		playerColorInt, ok := <-playerManagerInst.PlayerRebirth
		if !ok {
			return
		}
		if playerColorInt != 0 {
			botPlayer := CreateNewPlayer(nil, funnyBotNames[playerColorInt], playerColorInt,
				playerManagerInst.GameManager.getSpawnTile())

			botPlayer.BotStrategy = defaultStrategy
			botPlayer.StrategyName = "herpety derpety"
			playerManagerInst.GameManager.Players.Store(playerColorInt, botPlayer)
		}
	}
}
