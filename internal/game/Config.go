package game

import "time"

const (
	GameTickDuration          = 70 * time.Millisecond
	VoidColor                 = 233
	WallColor                 = 172
	sunsetWorkersCount        = 100
	rebirthWorkerCount        = 3
	spaceFillerChannelWorkers = 256
	botCount                  = 150
	MapColCount               = 1000
	MapRowCount               = 1000
)

var SystemColors = map[int]string{WallColor: "WALL", VoidColor: "void"}
