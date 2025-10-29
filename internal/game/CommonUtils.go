package game

import "math"

var Directions = [][]int{
	{1, 0},
	{0, 1},
	{-1, 0},
	{0, -1},
}

func GetManhattanDistance(t1, t2 *Tile) int {
	dx := math.Abs(float64(t1.X - t2.X))
	dy := math.Abs(float64(t1.Y - t2.Y))
	return int(dx + dy)
}

var SystemColors = map[int]string{237: "WALL", 235: "void"}

func IsWall(row int, col int) bool {
	if row <= 0 || col <= 0 {
		return true
	}

	if col >= MapColCount-1 || row >= MapRowCount-1 {
		return true
	}

	return false
}
