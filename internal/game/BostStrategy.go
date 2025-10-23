package game

import (
	"math"
)

// HierarchicalStrategy implements the Strategy interface.
type HierarchicalStrategy struct{}

// isSafeTile checks if a tile is within the assumed safe playing area (not a boundary wall).
func (s *HierarchicalStrategy) isSafeTile(row, col int) bool {
	// Check array bounds first
	if row < 0 || row >= MapRowCount || col < 0 || col >= MapColCount {
		return false
	}

	// Assume the outer perimeter is the wall: playable area starts at (1, 1) and ends at (Max-2, Max-2)
	return row > 0 && row < MapRowCount-1 && col > 0 && col < MapColCount-1
}

// getNextBestDirection determines the best move for the bot based on a strict priority hierarchy.
func (s *HierarchicalStrategy) getNextBestDirection(player *Player, gm *GameManager) Direction {

	currentTile := player.Location
	validMoves := make(map[Direction]*Tile)

	// --- 1. Filter and Collect All Valid Moves (Wall and Collision Avoidance) ---
	for _, dirCoords := range directions {
		dx, dy := dirCoords[1], dirCoords[0]
		nextX := currentTile.X + dx
		nextY := currentTile.Y + dy

		dir := Direction{Dx: dx, Dy: dy, PlayerColor: *player.Color}

		// PREVENT 180-DEGREE TURN
		if dx == -player.CurrentDirection.Dx && dy == -player.CurrentDirection.Dy {
			continue
		}

		// A. EXPLICIT WALL AVOIDANCE (Rule 5)
		if !s.isSafeTile(nextY, nextX) {
			continue
		}

		nextTile := gm.GameMap[nextY][nextX]

		// B. Self-collision avoidance
		if nextTile.IsTail && nextTile.OwnerColor == player.Color {
			continue
		}

		// C. Opponent's head collision avoidance
		isOpponentHead := false
		for _, otherPlayer := range gm.Players {
			if otherPlayer != nil && otherPlayer.Color != player.Color && otherPlayer.Location == nextTile {
				isOpponentHead = true
				break
			}
		}

		if isOpponentHead {
			continue
		}

		validMoves[dir] = nextTile
	}

	if len(validMoves) == 0 {
		return player.CurrentDirection // Trapped
	}

	// --- 2. Apply Hierarchical Priorities to Select the Best Safe Move ---

	// P1: Attack
	for dir, tile := range validMoves {
		if gm.isOtherPlayerTail(tile, player.Color) {
			return dir
		}
	}

	// P2: Loop Closure
	var bestClosingDir Direction
	maxGain := -1
	isThreatened := s.calculateThreatScore(player, gm) > 0

	for dir, tile := range validMoves {
		if tile.OwnerColor != nil && *tile.OwnerColor == *player.Color && !tile.IsTail {
			estimatedGain := s.estimateTerritoryGain(player)

			if estimatedGain >= 10 || isThreatened {
				if estimatedGain > maxGain {
					maxGain = estimatedGain
					bestClosingDir = dir
				}
			}
		}
	}

	if maxGain > -1 {
		return bestClosingDir
	}

	// P3: Survival and Fleeing
	if isThreatened {
		return s.getSafestFleeDirection(player, gm, validMoves)
	}

	// P4: Safe Expansion (Interior/Center-Seeking Bias)
	bestDir := player.CurrentDirection
	minDistToClaimed := math.MaxInt32

	// Initialize bestDir to be a valid move
	for dir := range validMoves {
		bestDir = dir
		break
	}

	nearestClaimedTile := s.findNearestClaimedTile(player.Location, player.Color, gm)

	// Define the map center for the new bias
	centerTile := &Tile{X: MapColCount / 2, Y: MapRowCount / 2}

	for dir, tile := range validMoves {
		dist := math.MaxInt32

		if nearestClaimedTile != nil {
			dist = getManhattanDistance(tile, nearestClaimedTile)
		}

		// Minor bonus for continuing straight (keeps lines clean, but not dominant)
		if dir.Dx == player.CurrentDirection.Dx && dir.Dy == player.CurrentDirection.Dy {
			dist -= 2
		}

		// **CRITICAL FIX: CENTER-SEEKING BIAS**
		if len(player.Tail) > 15 { // Only activate bias when tail is long
			distToCenter := getManhattanDistance(tile, centerTile)
			currentDistToCenter := getManhattanDistance(currentTile, centerTile)

			// Penalty for moving *away* from the map center (forces inward turn)
			if distToCenter > currentDistToCenter {
				dist += 50 // Strong penalty to discourage perimeter movement
			}
		}

		if dist < minDistToClaimed {
			minDistToClaimed = dist
			bestDir = dir
		}
	}

	return bestDir
}

// All helper functions (findNearestClaimedTile, getSafestFleeDirection, etc.) remain the same
// but are listed here for completeness and to show the consistent use of isSafeTile.

// findNearestClaimedTile performs a limited BFS for the closest claimed tile.
func (s *HierarchicalStrategy) findNearestClaimedTile(start *Tile, playerColor *int, gm *GameManager) *Tile {
	const maxSearchDepth = 15

	q := []*Tile{start}
	visited := make(map[*Tile]bool)
	distance := make(map[*Tile]int)

	visited[start] = true
	distance[start] = 0

	for len(q) > 0 {
		current := q[0]
		q = q[1:]

		dist := distance[current]
		if dist > maxSearchDepth {
			return nil
		}

		if current.OwnerColor != nil && *current.OwnerColor == *playerColor && !current.IsTail {
			return current
		}

		for _, dirCoords := range directions {
			dx, dy := dirCoords[1], dirCoords[0]
			nextRow, nextCol := current.Y+dy, current.X+dx

			if !s.isSafeTile(nextRow, nextCol) {
				continue
			}

			nextTile := gm.GameMap[nextRow][nextCol]

			if _, alreadyVisited := visited[nextTile]; !alreadyVisited {
				visited[nextTile] = true
				distance[nextTile] = dist + 1
				q = append(q, nextTile)
			}
		}
	}

	return nil
}

// getSafestFleeDirection is a new function to handle high-threat scenarios.
func (s *HierarchicalStrategy) getSafestFleeDirection(player *Player, gm *GameManager, validMoves map[Direction]*Tile) Direction {
	nearestOpponentHead := s.findNearestOpponentHead(player, gm)

	if nearestOpponentHead == nil {
		return s.getBestExpansionDirection(player, gm, validMoves)
	}

	bestFleeDir := player.CurrentDirection
	maxOpponentDistance := -1

	for dir := range validMoves {
		bestFleeDir = dir
		break
	}

	nearestClaimedTile := s.findNearestClaimedTile(player.Location, player.Color, gm)
	minBaseDistance := math.MaxInt32

	for dir, tile := range validMoves {
		distToOpponent := getManhattanDistance(tile, nearestOpponentHead)

		if distToOpponent > maxOpponentDistance {
			maxOpponentDistance = distToOpponent
			bestFleeDir = dir
		} else if distToOpponent == maxOpponentDistance {
			if nearestClaimedTile != nil {
				distToBase := getManhattanDistance(tile, nearestClaimedTile)
				if distToBase < minBaseDistance {
					minBaseDistance = distToBase
					bestFleeDir = dir
				}
			}
		}
	}

	return bestFleeDir
}

// getBestExpansionDirection is P4 logic extracted for clarity.
func (s *HierarchicalStrategy) getBestExpansionDirection(player *Player, gm *GameManager, validMoves map[Direction]*Tile) Direction {
	bestDir := player.CurrentDirection
	minDistToClaimed := math.MaxInt32

	nearestClaimedTile := s.findNearestClaimedTile(player.Location, player.Color, gm)

	for dir := range validMoves {
		bestDir = dir
		break
	}

	centerTile := &Tile{X: MapColCount / 2, Y: MapRowCount / 2}

	for dir, tile := range validMoves {
		dist := math.MaxInt32

		if nearestClaimedTile != nil {
			dist = getManhattanDistance(tile, nearestClaimedTile)
		}

		if dir.Dx == player.CurrentDirection.Dx && dir.Dy == player.CurrentDirection.Dy {
			dist -= 2
		}

		if len(player.Tail) > 15 {
			distToCenter := getManhattanDistance(tile, centerTile)
			currentDistToCenter := getManhattanDistance(player.Location, centerTile)

			if distToCenter > currentDistToCenter {
				dist += 50
			}
		}

		if dist < minDistToClaimed {
			minDistToClaimed = dist
			bestDir = dir
		}
	}

	return bestDir
}

func (s *HierarchicalStrategy) findNearestOpponentHead(player *Player, gm *GameManager) *Tile {
	minDist := math.MaxInt32
	var nearestHead *Tile

	for _, otherPlayer := range gm.Players {
		if otherPlayer == nil || otherPlayer.Color == player.Color {
			continue
		}

		dist := getManhattanDistance(player.Location, otherPlayer.Location)
		if dist < minDist {
			minDist = dist
			nearestHead = otherPlayer.Location
		}
	}
	return nearestHead
}

func (s *HierarchicalStrategy) calculateThreatScore(player *Player, gm *GameManager) int {
	tailLength := len(player.Tail)
	if tailLength < 2 {
		return 0
	}

	totalThreat := 0
	for _, otherPlayer := range gm.Players {
		if otherPlayer == nil || otherPlayer.Color == player.Color {
			continue
		}

		opponentHead := otherPlayer.Location

		minDistToTail := math.MaxInt32
		for _, tailTile := range player.Tail {
			dist := getManhattanDistance(opponentHead, tailTile)
			if dist < minDistToTail {
				minDistToTail = dist
			}
		}

		if minDistToTail <= 3 {
			threatFactor := 4 - minDistToTail
			totalThreat += 500 * threatFactor
		}
	}

	return totalThreat
}

func (s *HierarchicalStrategy) estimateTerritoryGain(player *Player) int {
	tailLength := len(player.Tail)
	if tailLength < 3 {
		return 0
	}
	return tailLength * 2
}

// Get the implementation of the strategy interface.
var AgresssorStrategy Strategy = &HierarchicalStrategy{}
