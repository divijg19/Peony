package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/ri5hii/peony/internal/core"
)

// Store provides SQLite-backed persistence for thoughts and events.
type Store struct {
	db *sql.DB
}

// New creates a Store from an existing database handle.
func New(db *sql.DB) (*Store, error) {
	if db == nil {
		return nil, fmt.Errorf("db is nil")
	}
	return &Store{db: db}, nil
}

// CreateThought inserts a new thought in captured state and returns its id.
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
	nowTime := time.Now().UTC()
	now := nowTime.Format(time.RFC3339Nano)
	eligibilityAt := nowTime.Add(core.SettleDuration).Format(time.RFC3339Nano)
	state := core.StateCaptured
	sqlString := `INSERT INTO thoughts (content, current_state, tend_counter, created_at, updated_at, last_tended_at, eligibility_at, valence, energy)
	             VALUES (?, ?, 0, ?, ?, NULL, ?, NULL, NULL)`
	var err error
	var result sql.Result
	result, err = s.db.Exec(sqlString, content, string(state), now, now, eligibilityAt)
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

// AppendEvent records an append-only event for a thought.
func (s *Store) AppendEvent(thoughtID int64, kind string, previousState, nextState *core.State, note *string) error {
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

	var previousStateValue any
	if previousState != nil {
		previousStateValue = string(*previousState)
	} else {
		previousStateValue = nil
	}

	var nextStateValue any
	if nextState != nil {
		nextStateValue = string(*nextState)
	} else {
		nextStateValue = nil
	}

	var noteValue any
	if note != nil {
		noteValue = *note
	} else {
		noteValue = nil
	}

	sqlString := `INSERT INTO events (thought_id, kind, at, previous_state, next_state, note) VALUES (?, ?, ?, ?, ?, ?)`
	var err error
	_, err = s.db.Exec(sqlString, thoughtID, kind, now, previousStateValue, nextStateValue, noteValue)
	if err != nil {
		return fmt.Errorf("append event: insert: %w", err)
	}
	return nil
}

// GetThought returns the thought snapshot and its event history.
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

	sqlThought := `SELECT id, content, current_state, tend_counter, created_at, updated_at, last_tended_at, eligibility_at, valence, energy FROM thoughts WHERE id = ?`

	var thought core.Thought
	var createdAtStr, updatedAtStr string
	var lastTendedAtStr sql.NullString
	var valence sql.NullInt64
	var energy sql.NullInt64
	var stateStr string
	var tendCounter int
	var eligibilityAtStr string

	var err error
	row := s.db.QueryRow(sqlThought, id)
	err = row.Scan(
		&thought.ID,
		&thought.Content,
		&stateStr,
		&tendCounter,
		&createdAtStr,
		&updatedAtStr,
		&lastTendedAtStr,
		&eligibilityAtStr,
		&valence,
		&energy,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return core.Thought{}, nil, fmt.Errorf("get thought: not found")
		}
		return core.Thought{}, nil, fmt.Errorf("get thought: scan: %w", err)
	}

	thought.CurrentState = core.State(stateStr)
	thought.TendCounter = tendCounter

	thought.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAtStr)
	if err != nil {
		return core.Thought{}, nil, fmt.Errorf("get thought: parse created_at: %w", err)
	}
	thought.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAtStr)
	if err != nil {
		return core.Thought{}, nil, fmt.Errorf("get thought: parse updated_at: %w", err)
	}

	thought.EligibilityAt, err = time.Parse(time.RFC3339Nano, eligibilityAtStr)
	if err != nil {
		return core.Thought{}, nil, fmt.Errorf("get thought: parse eligibility_at: %w", err)
	}

	if lastTendedAtStr.Valid {
		var t time.Time
		t, err = time.Parse(time.RFC3339Nano, lastTendedAtStr.String)
		if err != nil {
			return core.Thought{}, nil, fmt.Errorf("get thought: parse last_tended_at: %w", err)
		}
		thought.LastTendedAt = &t
	}

	if valence.Valid {
		v := int(valence.Int64)
		thought.Valence = &v
	}

	if energy.Valid {
		e := int(energy.Int64)
		thought.Energy = &e
	}

	sqlEvents := `SELECT id, thought_id, kind, at, previous_state, next_state, note FROM events WHERE thought_id = ? ORDER BY at ASC, id ASC`
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
		var previousStateStr sql.NullString
		var nextStateStr sql.NullString
		var noteStr sql.NullString

		err = rows.Scan(&ev.ID, &ev.ThoughtID, &ev.Kind, &atStr, &previousStateStr, &nextStateStr, &noteStr)
		if err != nil {
			return core.Thought{}, nil, fmt.Errorf("get thought: scan event: %w", err)
		}

		ev.At, err = time.Parse(time.RFC3339Nano, atStr)
		if err != nil {
			return core.Thought{}, nil, fmt.Errorf("get thought: parse event at: %w", err)
		}
		if previousStateStr.Valid {
			ps := core.State(previousStateStr.String)
			ev.PreviousState = &ps
		}
		if nextStateStr.Valid {
			ns := core.State(nextStateStr.String)
			ev.NextState = &ns
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


func (s *Store) ListThoughtsByPagination(limit, offset int) ([]core.Thought, error) {
    if s == nil {
        return nil, fmt.Errorf("list thoughts: store is nil")
    }
    if s.db == nil {
        return nil, fmt.Errorf("list thoughts: db is nil")
    }
    if limit <= 0 {
        return nil, fmt.Errorf("list thoughts: limit must be > 0")
    }
    if offset < 0 {
        return nil, fmt.Errorf("list thoughts: offset must be >= 0")
    }

    sqlList := `SELECT id, content, current_state, tend_counter, updated_at
                FROM thoughts
                ORDER BY updated_at ASC, id ASC
                LIMIT ? OFFSET ?`

    rows, err := s.db.Query(sqlList, limit, offset)
    if err != nil {
        return nil, fmt.Errorf("list thoughts: query: %w", err)
    }
    defer rows.Close()

    thoughts := make([]core.Thought, 0, limit)
    for rows.Next() {
        var th core.Thought
        var stateStr string
        var updatedAtStr string

        if err := rows.Scan(&th.ID, &th.Content, &stateStr, &th.TendCounter, &updatedAtStr); err != nil {
            return nil, fmt.Errorf("list thoughts: scan: %w", err)
        }

        th.CurrentState = core.State(stateStr)

        th.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAtStr)
        if err != nil {
            return nil, fmt.Errorf("list thoughts: parse updated_at: %w", err)
        }

        thoughts = append(thoughts, th)
    }

    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("list thoughts: rows: %w", err)
    }

    return thoughts, nil
}

// ListTendThought returns thoughts that are eligible to be tended at the provided time.
func (s *Store) ListTendThought(now time.Time) ([]core.Thought, error) {
	if s == nil {
		return nil, fmt.Errorf("list tend thought: store is nil")
	}
	if s.db == nil {
		return nil, fmt.Errorf("list tend thought: db is nil")
	}

	thoughts := make([]core.Thought, 0)

	nowStr := now.UTC().Format(time.RFC3339Nano)
	sqlList := `SELECT id, content, current_state, tend_counter, created_at, updated_at, last_tended_at, eligibility_at, valence, energy
			FROM thoughts
			WHERE current_state IN (?, ?)
			AND eligibility_at <= ?
			ORDER BY eligibility_at ASC, id ASC`

	var err error
	var rows *sql.Rows
	rows, err = s.db.Query(sqlList, string(core.StateCaptured), string(core.StateResting), nowStr)
	if err != nil {
		return nil, fmt.Errorf("list tend thought: query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var th core.Thought

		var createdAtStr, updatedAtStr string
		var lastTendedAtStr sql.NullString
		var valence sql.NullInt64
		var energy sql.NullInt64
		var stateStr string
		var tendCounter int
		var eligibilityAtStr string

		err = rows.Scan(
			&th.ID,
			&th.Content,
			&stateStr,
			&tendCounter,
			&createdAtStr,
			&updatedAtStr,
			&lastTendedAtStr,
			&eligibilityAtStr,
			&valence,
			&energy,
		)
		if err != nil {
			return nil, fmt.Errorf("list tend thought: scan: %w", err)
		}

		th.CurrentState = core.State(stateStr)
		th.TendCounter = tendCounter

		th.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAtStr)
		if err != nil {
			return nil, fmt.Errorf("list tend thought: parse created_at: %w", err)
		}

		th.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAtStr)
		if err != nil {
			return nil, fmt.Errorf("list tend thought: parse updated_at: %w", err)
		}

		th.EligibilityAt, err = time.Parse(time.RFC3339Nano, eligibilityAtStr)
		if err != nil {
			return nil, fmt.Errorf("list tend thought: parse eligibility_at: %w", err)
		}

		if lastTendedAtStr.Valid {
			var t time.Time
			t, err = time.Parse(time.RFC3339Nano, lastTendedAtStr.String)
			if err != nil {
				return nil, fmt.Errorf("list tend thought: parse last_tended_at: %w", err)
			}
			th.LastTendedAt = &t
		}

		if valence.Valid {
			v := int(valence.Int64)
			th.Valence = &v
		}

		if energy.Valid {
			e := int(energy.Int64)
			th.Energy = &e
		}

		if core.EligibleToSurface(th, now) {
			thoughts = append(thoughts, th)
		}
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("list tend thought: rows: %w", err)
	}
	return thoughts, nil
}
