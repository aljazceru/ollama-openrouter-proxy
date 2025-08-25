package main

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"os"
	"time"
)

type FailureStore struct {
	db *sql.DB
	defaultCooldown time.Duration
	rateLimitCooldown time.Duration
}

func NewFailureStore(path string) (*FailureStore, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	// Updated schema with failure type
	if _, err = db.Exec(`CREATE TABLE IF NOT EXISTS failures (
		model TEXT PRIMARY KEY, 
		failed_at INTEGER,
		failure_type TEXT DEFAULT 'general',
		failure_count INTEGER DEFAULT 1
	)`); err != nil {
		db.Close()
		return nil, err
	}
	
	// Set default cooldown periods
	defaultCooldown := 5 * time.Minute
	if cd := os.Getenv("FAILURE_COOLDOWN_MINUTES"); cd != "" {
		if minutes, err := time.ParseDuration(cd + "m"); err == nil {
			defaultCooldown = minutes
		}
	}
	
	rateLimitCooldown := 1 * time.Minute
	if cd := os.Getenv("RATELIMIT_COOLDOWN_MINUTES"); cd != "" {
		if minutes, err := time.ParseDuration(cd + "m"); err == nil {
			rateLimitCooldown = minutes
		}
	}
	
	return &FailureStore{
		db: db,
		defaultCooldown: defaultCooldown,
		rateLimitCooldown: rateLimitCooldown,
	}, nil
}

func (s *FailureStore) Close() error { return s.db.Close() }

func (s *FailureStore) MarkFailure(model string) error {
	return s.MarkFailureWithType(model, "general")
}

func (s *FailureStore) MarkFailureWithType(model string, failureType string) error {
	_, err := s.db.Exec(`
		INSERT INTO failures(model, failed_at, failure_type, failure_count) 
		VALUES(?, ?, ?, 1) 
		ON CONFLICT(model) DO UPDATE SET 
			failed_at=excluded.failed_at,
			failure_type=excluded.failure_type,
			failure_count=failure_count+1
	`, model, time.Now().Unix(), failureType)
	return err
}

func (s *FailureStore) ShouldSkip(model string) (bool, error) {
	var ts int64
	var failureType string
	var failureCount int
	err := s.db.QueryRow(`SELECT failed_at, failure_type, failure_count FROM failures WHERE model=?`, model).Scan(&ts, &failureType, &failureCount)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	
	// Use different cooldown periods based on failure type
	var cooldown time.Duration
	if failureType == "rate_limit" {
		cooldown = s.rateLimitCooldown
	} else {
		cooldown = s.defaultCooldown
		// Exponential backoff for repeated failures
		if failureCount > 1 {
			cooldown = cooldown * time.Duration(min(failureCount, 5))
		}
	}
	
	if time.Since(time.Unix(ts, 0)) < cooldown {
		return true, nil
	}
	return false, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ClearFailure removes a model from the failure store (for successful requests)
func (s *FailureStore) ClearFailure(model string) error {
	// Instead of deleting, reset the failure count but keep the record
	// This helps track patterns over time
	_, err := s.db.Exec(`UPDATE failures SET failure_count=0, failure_type='cleared' WHERE model=?`, model)
	if err == sql.ErrNoRows {
		return nil // No failure to clear, that's ok
	}
	return err
}

// ResetAllFailures clears all failure records (useful for testing or manual reset)
func (s *FailureStore) ResetAllFailures() error {
	_, err := s.db.Exec(`DELETE FROM failures`)
	return err
}
