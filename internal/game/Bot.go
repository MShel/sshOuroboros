package game

type Strategy interface {
	getNextBestDirection(player *Player, gm *GameManager) Direction
}

type Bot struct {
	BotStrategy Strategy
	*Player
}
