package storage

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/ri5hii/peony/internal/core"
)

// Store provides persistence operations for thoughts and events.
type Store struct {
	db *sql.DB
}

// New constructs a Store backed by db.
func New(db *sql.DB) (*Store, error) {
	if db == nil {
		return nil, fmt.Errorf("db is nil")
	}
	return &Store{db: db}, nil
}

// CreateThought inserts a new thought in the captured state and returns its id.
func (s *Store) CreateThought(content string) (int64, error) {
	if s == nil {
		return -1, fmt.Errorf("create thought: store is nil")
	}
	if s.db == nil {
		return -1, fmt.Errorf("create thought: db is nil")
	}
	if content == "" {
		return -1, fmt.Errorf("create thought: content is empty")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	state := core.StateCaptured

	sqlString := `INSERT INTO thoughts (content, state, created_at, updated_at) VALUES (?, ?, ?, ?)`
	var err error
	var result sql.Result
	result, err = s.db.Exec(sqlString, content, string(state), now, now)
	if err != nil {
		return -1, fmt.Errorf("create thought: insert: %w", err)
	}
	var id int64
	id, err = result.LastInsertId()
	if err != nil {
		return -1, fmt.Errorf("create thought: last insert id: %w", err)
	}
	return id, nil
}

// AppendEvent records an event for a thought.
func (s *Store) AppendEvent(thoughtID int64, kind string, note *string) error {
	if s == nil {
		return fmt.Errorf("append event: store is nil")
	}
	if s.db == nil {
		return fmt.Errorf("append event: db is nil")
	}
	if thoughtID <= 0 {
		return fmt.Errorf("append event: invalid thought ID")
	}
	if kind == "" {
		return fmt.Errorf("append event: kind is empty")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)

	var noteValue any
	if note != nil {
		noteValue = *note
	} else {
		noteValue = nil
	}

	sqlString := `INSERT INTO events (thought_id, kind, at, note) VALUES (?, ?, ?, ?)`
	var err error
	_, err = s.db.Exec(sqlString, thoughtID, kind, now, noteValue)
	if err != nil {
		return fmt.Errorf("append event: insert: %w", err)
	}
	return nil
}
