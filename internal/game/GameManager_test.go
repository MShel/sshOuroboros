package game

// --- Mocking Environment Setup ---

// Define small map constants for testing (must be accessible to the game package for player movement)
const (
	TestMapRowCount = 20
	TestMapColCount = 20
	TestSpawnY      = 10
	TestSpawnX      = 10
)

// The game package variables (MapRowCount, MapColCount) are used by Player.GetNextTile()
// To enable testing, we set the package global variables here:
func init() {
	MapRowCount = TestMapRowCount
	MapColCount = TestMapColCount
}
