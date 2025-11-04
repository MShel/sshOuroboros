package game

import (
	"math"
)

type DefaultStrategy struct{}

func (s *DefaultStrategy) getNextBestDirection(player *Player, gm *GameManager) Direction {

	if player.isDead {
		return Direction{}
	}

	currentTile := player.Location
	validMoves := make(map[Direction]*Tile)

	for _, dirCoords := range Directions {
		dx, dy := dirCoords[1], dirCoords[0]
		nextX := currentTile.X + dx
		nextY := currentTile.Y + dy

		dir := Direction{Dx: dx, Dy: dy, PlayerColor: *player.Color}

		if dx == -player.CurrentDirection.Dx && dy == -player.CurrentDirection.Dy {
			continue
		}

		if IsWall(nextY, nextX) {
			continue
		}

		nextTile := gm.GameMap[nextY][nextX]

		isOpponentHead := false
		gm.Players.Range(func(key, value interface{}) bool {
			if otherPlayer, ok := value.(*Player); ok && otherPlayer != nil {
				if key.(int) != *player.Color && otherPlayer.Location == nextTile {
					isOpponentHead = true
					return false
				}
			}
			return true
		})

		if isOpponentHead {
			continue
		}

		validMoves[dir] = nextTile
	}

	if len(validMoves) == 0 {
		return player.CurrentDirection
	}

	for dir, tile := range validMoves {
		if gm.isOtherPlayerTail(tile, player.Color) {
			return dir
		}
	}

	var bestClosingDir Direction
	maxGain := -1
	isThreatened := s.calculateThreatScore(player, gm) > 0

	for dir, tile := range validMoves {
		if tile.OwnerColor != nil && *tile.OwnerColor == *player.Color {
			estimatedGain := s.estimateTerritoryGain(player)

			if estimatedGain >= 1 || isThreatened {
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

	if isThreatened {
		return s.getSafestFleeDirection(player, gm, validMoves)
	}

	bestDir := player.CurrentDirection
	minDistToClaimed := math.MaxInt32

	for dir := range validMoves {
		bestDir = dir
		break
	}

	nearestClaimedTile := s.findNearestClaimedTile(player.Location, player.Color, gm)

	centerTile := &Tile{X: MapColCount / 2, Y: MapRowCount / 2}

	for dir, tile := range validMoves {
		dist := math.MaxInt32

		if nearestClaimedTile != nil {
			dist = GetManhattanDistance(tile, nearestClaimedTile)
		}

		if dir.Dx == player.CurrentDirection.Dx && dir.Dy == player.CurrentDirection.Dy {
			dist -= 2
		}

		if len(player.Tail.tailTiles) > 5 {
			distToCenter := GetManhattanDistance(tile, centerTile)
			currentDistToCenter := GetManhattanDistance(currentTile, centerTile)

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

func (s *DefaultStrategy) findNearestClaimedTile(start *Tile, playerColor *int, gm *GameManager) *Tile {
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

		if current.OwnerColor != nil && *current.OwnerColor == *playerColor {
			return current
		}

		for _, dirCoords := range Directions {
			dx, dy := dirCoords[1], dirCoords[0]
			nextRow, nextCol := current.Y+dy, current.X+dx

			if IsWall(nextRow, nextCol) {
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

func (s *DefaultStrategy) getSafestFleeDirection(player *Player, gm *GameManager, validMoves map[Direction]*Tile) Direction {
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
		distToOpponent := GetManhattanDistance(tile, nearestOpponentHead)

		if distToOpponent > maxOpponentDistance {
			maxOpponentDistance = distToOpponent
			bestFleeDir = dir
		} else if distToOpponent == maxOpponentDistance {
			if nearestClaimedTile != nil {
				distToBase := GetManhattanDistance(tile, nearestClaimedTile)
				if distToBase < minBaseDistance {
					minBaseDistance = distToBase
					bestFleeDir = dir
				}
			}
		}
	}

	return bestFleeDir
}

func (s *DefaultStrategy) getBestExpansionDirection(player *Player, gm *GameManager, validMoves map[Direction]*Tile) Direction {
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
			dist = GetManhattanDistance(tile, nearestClaimedTile)
		}

		if dir.Dx == player.CurrentDirection.Dx && dir.Dy == player.CurrentDirection.Dy {
			dist -= 2
		}

		if len(player.Tail.tailTiles) > 15 {
			distToCenter := GetManhattanDistance(tile, centerTile)
			currentDistToCenter := GetManhattanDistance(player.Location, centerTile)

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

func (s *DefaultStrategy) findNearestOpponentHead(player *Player, gm *GameManager) *Tile {
	minDist := math.MaxInt32
	var nearestHead *Tile

	gm.Players.Range(func(key, value interface{}) bool {
		if otherPlayer, ok := value.(*Player); ok && otherPlayer != nil {
			if key.(int) == *player.Color {
				return true
			}

			dist := GetManhattanDistance(player.Location, otherPlayer.Location)
			if dist < minDist {
				minDist = dist
				nearestHead = otherPlayer.Location
			}
		}
		return true
	})

	return nearestHead
}

func (s *DefaultStrategy) calculateThreatScore(player *Player, gm *GameManager) int {
	tailLength := len(player.Tail.tailTiles)
	if tailLength < 2 {
		return 0
	}

	totalThreat := 0
	gm.Players.Range(func(key, value interface{}) bool {
		if otherPlayer, ok := value.(*Player); ok && otherPlayer != nil {
			if key.(int) == *player.Color {
				return true
			}

			opponentHead := otherPlayer.Location

			minDistToTail := math.MaxInt32
			for _, tailTile := range player.Tail.tailTiles {
				dist := GetManhattanDistance(opponentHead, tailTile)
				if dist < minDistToTail {
					minDistToTail = dist
				}
			}

			if minDistToTail <= 3 {
				threatFactor := 4 - minDistToTail
				totalThreat += 500 * threatFactor
			}
		}
		return true
	})

	return totalThreat
}

func (s *DefaultStrategy) estimateTerritoryGain(player *Player) int {
	tailLength := len(player.Tail.tailTiles)
	if tailLength < 3 {
		return 0
	}
	return tailLength * 2
}

var AgresssorStrategy Strategy = &DefaultStrategy{}
