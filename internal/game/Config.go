package game

import "time"

const (
	GameTickDuration          = 100 * time.Millisecond
	VoidColor                 = 233
	WallColor                 = 172
	sunsetWorkersCount        = 50
	rebirthWorkerCount        = 1
	spaceFillerChannelWorkers = 256
	botCount                  = 254
	MapColCount               = 700
	MapRowCount               = 500
)

var SystemColors = map[int]string{WallColor: "WALL", VoidColor: "void"}
