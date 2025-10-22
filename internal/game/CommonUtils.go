package game

import "math"

// getManhattanDistance calculates the Manhattan distance (L1 norm) between two tiles.
func getManhattanDistance(t1, t2 *Tile) int {
	dx := math.Abs(float64(t1.X - t2.X))
	dy := math.Abs(float64(t1.Y - t2.Y))
	return int(dx + dy)
}
