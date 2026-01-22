package storage

import (
	"database/sql"
	"errors"
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

func (s *Store) GetThought(id int64) (core.Thought, []core.Event, error) {
	if s == nil {
		return core.Thought{}, nil, fmt.Errorf("get thought: store is nil")
	}
	if s.db == nil {
		return core.Thought{}, nil, fmt.Errorf("get thought: db is nil")
	}
	if id <= 0 {
		return core.Thought{}, nil, fmt.Errorf("get thought: invalid thought ID")
	}

	sqlThought := `SELECT id, content, state, created_at, updated_at, last_tended_at, rest_until, valence, energy FROM thoughts WHERE id = ?`

	var thought core.Thought
	var createdAtStr, updatedAtStr string
	var lastTendedAtStr sql.NullString
	var restUntilStr sql.NullString
	var valence sql.NullInt64
	var energy sql.NullInt64
	var stateStr string

	var err error
	row := s.db.QueryRow(sqlThought, id)
	err = row.Scan(
		&thought.ID,
		&thought.Content,
		&stateStr,
		&createdAtStr,
		&updatedAtStr,
		&lastTendedAtStr,
		&restUntilStr,
		&valence,
		&energy,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return core.Thought{}, nil, fmt.Errorf("get thought: not found")
		}
		return core.Thought{}, nil, fmt.Errorf("get thought: scan: %w", err)
	}

	thought.State = core.State(stateStr)

	thought.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAtStr)
	if err != nil {
		return core.Thought{}, nil, fmt.Errorf("get thought: parse created_at: %w", err)
	}
	thought.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAtStr)
	if err != nil {
		return core.Thought{}, nil, fmt.Errorf("get thought: parse updated_at: %w", err)
	}

	if lastTendedAtStr.Valid {
		var t time.Time
		t, err = time.Parse(time.RFC3339Nano, lastTendedAtStr.String)
		if err != nil {
			return core.Thought{}, nil, fmt.Errorf("get thought: parse last_tended_at: %w", err)
		}
		thought.LastTendedAt = &t
	}

	if restUntilStr.Valid {
		var t time.Time
		t, err = time.Parse(time.RFC3339Nano, restUntilStr.String)
		if err != nil {
			return core.Thought{}, nil, fmt.Errorf("get thought: parse rest_until: %w", err)
		}
		thought.RestUntil = &t
	}

	if valence.Valid {
		v := int(valence.Int64)
		thought.Valence = &v
	}

	if energy.Valid {
		e := int(energy.Int64)
		thought.Energy = &e
	}

	sqlEvents := `SELECT id, thought_id, kind, at, note FROM events WHERE thought_id = ? ORDER BY at ASC, id ASC`
	var rows *sql.Rows
	rows, err = s.db.Query(sqlEvents, id)
	if err != nil {
		return core.Thought{}, nil, fmt.Errorf("get thought: query events: %w", err)
	}
	defer rows.Close()

	events := make([]core.Event, 0)
	for rows.Next() {
		var ev core.Event
		var atStr string
		var noteStr sql.NullString

		err = rows.Scan(&ev.ID, &ev.ThoughtID, &ev.Kind, &atStr, &noteStr)
		if err != nil {
			return core.Thought{}, nil, fmt.Errorf("get thought: scan event: %w", err)
		}

		ev.At, err = time.Parse(time.RFC3339Nano, atStr)
		if err != nil {
			return core.Thought{}, nil, fmt.Errorf("get thought: parse event at: %w", err)
		}

		if noteStr.Valid {
			n := noteStr.String
			ev.Note = &n
		}

		events = append(events, ev)
	}

	err = rows.Err()
	if err != nil {
		return core.Thought{}, nil, fmt.Errorf("get thought: events rows: %w", err)
	}

	return thought, events, nil
}
