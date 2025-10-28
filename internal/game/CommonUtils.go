package game

import "math"

var Directions = [][]int{
	{1, 0},
	{0, 1},
	{-1, 0},
	{0, -1},
}

func getManhattanDistance(t1, t2 *Tile) int {
	dx := math.Abs(float64(t1.X - t2.X))
	dy := math.Abs(float64(t1.Y - t2.Y))
	return int(dx + dy)
}

var systemColors = map[int]string{237: "WALL", 235: "void"}
