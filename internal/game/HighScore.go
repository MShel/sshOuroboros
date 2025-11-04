package game

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type HighScoreService struct {
	db *sql.DB
}

const dbPath = "highscores.db"
const tableName = "high_scores"

type Score struct {
	ID          int
	PlayerName  string
	ClaimedLand float64 // Updated type to match the REAL type in the database schema
	Kills       int
	CreatedAt   time.Time
}

func NewHighScoreService() *HighScoreService {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Error opening database: %v", err)
	}

	service := &HighScoreService{db: db}
	if err := service.createTable(); err != nil {
		log.Fatalf("Error creating high scores table: %v", err)
	}

	return service
}

// createTable creates the high_scores table if it does not exist.
func (serviceImpl *HighScoreService) createTable() error {
	const createTableSQL = `
	CREATE TABLE IF NOT EXISTS ` + tableName + ` (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		player_name TEXT NOT NULL,
		player_color INT NOT NULL,
		claimed_land REAL NOT NULL,
		kills INTEGER NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	_, err := serviceImpl.db.Exec(createTableSQL)
	if err != nil {
		return fmt.Errorf("failed to execute CREATE TABLE: %w", err)
	}
	log.Println("High scores table ensured.")
	return nil
}

func (serviceImpl *HighScoreService) SavePlayersHighScore(playerName string,
	playerColor int,
	claimedLand float64,
	kills int) error {
	const insertSQL = `
	INSERT INTO ` + tableName + ` (player_name, player_color, claimed_land, kills) 
	VALUES (?, ?, ?, ?);`

	_, err := serviceImpl.db.Exec(insertSQL, playerName, playerColor, claimedLand, kills)
	if err != nil {
		return fmt.Errorf("failed to insert high score for %s: %w", playerName, err)
	}

	return nil
}

// GetHighScores retrieves a paginated list of scores, ordered by claimed land and kills.
func (serviceImpl *HighScoreService) GetHighScores(limit, offset int) ([]Score, error) {
	const selectSQL = `
	SELECT id, player_name, claimed_land, kills, created_at
	FROM ` + tableName + `
	ORDER BY claimed_land DESC, kills DESC 
	LIMIT ? OFFSET ?;`

	rows, err := serviceImpl.db.Query(selectSQL, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query high scores: %w", err)
	}
	defer rows.Close()

	var scores []Score

	for rows.Next() {
		var score Score
		var createdAt string // Read as string from DB
		err := rows.Scan(&score.ID, &score.PlayerName, &score.ClaimedLand, &score.Kills, &createdAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		dateTimeCreatedAt, err := time.Parse(time.RFC3339, createdAt)
		if err == nil {
			score.CreatedAt = dateTimeCreatedAt
		} else {
			// Log the error to help debug if the format is still incorrect
			log.Printf("Time parsing error for score (ID %d, Name %s): %v, raw string: %s", score.ID, score.PlayerName, err, createdAt)
		}
		scores = append(scores, score)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error after iterating rows: %w", err)
	}

	return scores, nil
}

func (serviceImpl *HighScoreService) GetTotalScoreCount() (int, error) {
	const countSQL = `SELECT COUNT(*) FROM ` + tableName + `;`
	var count int
	err := serviceImpl.db.QueryRow(countSQL).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get total score count: %w", err)
	}
	return count, nil
}
