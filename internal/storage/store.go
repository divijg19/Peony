package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ri5hii/peony/internal/core"
)

// Store provides SQLite-backed persistence for thoughts and events.
type Store struct {
	db *sql.DB
}

const appStateKeyLastTendReadyCount = "last_tend_ready_count"

// New returns a Store bound to an existing database handle.
func New(db *sql.DB) (*Store, error) {
	if db == nil {
		return nil, fmt.Errorf("db is nil")
	}
	return &Store{db: db}, nil
}

// CreateThought inserts a new thought in captured state and returns its ID.
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

// AppendEvent appends an immutable event row for a thought.
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

// GetThought returns the thought snapshot and its ordered event history.
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
	err = row.Scan(&thought.ID, &thought.Content, &stateStr, &tendCounter, &createdAtStr, &updatedAtStr, &lastTendedAtStr, &eligibilityAtStr, &valence, &energy)
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
		var event core.Event
		var atStr string
		var previousStateStr sql.NullString
		var nextStateStr sql.NullString
		var noteStr sql.NullString

		err = rows.Scan(&event.ID, &event.ThoughtID, &event.Kind, &atStr, &previousStateStr, &nextStateStr, &noteStr)
		if err != nil {
			return core.Thought{}, nil, fmt.Errorf("get thought: scan event: %w", err)
		}

		event.At, err = time.Parse(time.RFC3339Nano, atStr)
		if err != nil {
			return core.Thought{}, nil, fmt.Errorf("get thought: parse event at: %w", err)
		}
		if previousStateStr.Valid {
			ps := core.State(previousStateStr.String)
			event.PreviousState = &ps
		}
		if nextStateStr.Valid {
			ns := core.State(nextStateStr.String)
			event.NextState = &ns
		}

		if noteStr.Valid {
			n := noteStr.String
			event.Note = &n
		}

		events = append(events, event)
	}

	err = rows.Err()
	if err != nil {
		return core.Thought{}, nil, fmt.Errorf("get thought: events rows: %w", err)
	}

	return thought, events, nil
}

// GetTendThought returns a thought and its events only if it is currently eligible for tending.
func (s *Store) GetTendThought(id int64) (core.Thought, []core.Event, error) {
	if s == nil {
		return core.Thought{}, nil, fmt.Errorf("get thought: store is nil")
	}
	if s.db == nil {
		return core.Thought{}, nil, fmt.Errorf("get thought: db is nil")
	}
	if id <= 0 {
		return core.Thought{}, nil, fmt.Errorf("get thought: invalid thought ID")
	}

	nowStr := time.Now().UTC().Format(time.RFC3339Nano)

	sqlThought := `SELECT id, content, current_state, tend_counter, created_at, updated_at, last_tended_at, eligibility_at, valence, energy
	               FROM thoughts
				   WHERE id = ? AND current_state IN (?, ?) AND eligibility_at <= ?
				  `

	var thought core.Thought
	var createdAtStr, updatedAtStr string
	var lastTendedAtStr sql.NullString
	var valence sql.NullInt64
	var energy sql.NullInt64
	var stateStr string
	var tendCounter int
	var eligibilityAtStr string

	var err error
	row := s.db.QueryRow(sqlThought, id, string(core.StateCaptured), string(core.StateResting), nowStr)
	err = row.Scan(&thought.ID, &thought.Content, &stateStr, &tendCounter, &createdAtStr, &updatedAtStr, &lastTendedAtStr, &eligibilityAtStr, &valence, &energy)
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

	sqlEvents := `SELECT id, thought_id, kind, at, previous_state, next_state, note
	              FROM events
	              WHERE thought_id = ?
	              ORDER BY at ASC, id ASC
				 `

	rows, err := s.db.Query(sqlEvents, id)
	if err != nil {
		return core.Thought{}, nil, fmt.Errorf("get thought: query events: %w", err)
	}
	defer rows.Close()

	events := make([]core.Event, 0)
	for rows.Next() {
		var event core.Event
		var atStr string
		var previousStateStr sql.NullString
		var nextStateStr sql.NullString
		var noteStr sql.NullString

		err = rows.Scan(&event.ID, &event.ThoughtID, &event.Kind, &atStr, &previousStateStr, &nextStateStr, &noteStr)
		if err != nil {
			return core.Thought{}, nil, fmt.Errorf("get thought: scan event: %w", err)
		}

		event.At, err = time.Parse(time.RFC3339Nano, atStr)
		if err != nil {
			return core.Thought{}, nil, fmt.Errorf("get thought: parse event at: %w", err)
		}

		if previousStateStr.Valid {
			ps := core.State(previousStateStr.String)
			event.PreviousState = &ps
		}

		if nextStateStr.Valid {
			ns := core.State(nextStateStr.String)
			event.NextState = &ns
		}

		if noteStr.Valid {
			n := noteStr.String
			event.Note = &n
		}

		events = append(events, event)
	}

	if err = rows.Err(); err != nil {
		return core.Thought{}, nil, fmt.Errorf("get thought: events rows: %w", err)
	}

	return thought, events, nil
}

// ListThoughtsByPagination returns a page of thoughts ordered by updated time and ID.
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
		var thought core.Thought
		var stateStr string
		var updatedAtStr string

		if err := rows.Scan(&thought.ID, &thought.Content, &stateStr, &thought.TendCounter, &updatedAtStr); err != nil {
			return nil, fmt.Errorf("list thoughts: scan: %w", err)
		}

		thought.CurrentState = core.State(stateStr)

		thought.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAtStr)
		if err != nil {
			return nil, fmt.Errorf("list thoughts: parse updated_at: %w", err)
		}

		thoughts = append(thoughts, thought)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list thoughts: rows: %w", err)
	}

	return thoughts, nil
}

// ListTendThoughtsByPagination returns a page of thoughts eligible for tending ordered by eligibility time and ID.
func (s *Store) ListTendThoughtsByPagination(limit, offset int) ([]core.Thought, error) {
	if s == nil {
		return nil, fmt.Errorf("list tend thoughts: store is nil")
	}
	if s.db == nil {
		return nil, fmt.Errorf("list tend thoughts: db is nil")
	}
	if limit <= 0 {
		return nil, fmt.Errorf("list tend thoughts: limit must be > 0")
	}
	if offset < 0 {
		return nil, fmt.Errorf("list tend thoughts: offset must be >= 0")
	}

	nowStr := time.Now().UTC().Format(time.RFC3339Nano)

	sqlList := `SELECT id, content, current_state, tend_counter, created_at, updated_at, last_tended_at, eligibility_at, valence, energy
	            FROM thoughts
	            WHERE current_state IN (?, ?)
	              AND eligibility_at <= ?
	            ORDER BY eligibility_at ASC, id ASC
	            LIMIT ? OFFSET ?`

	rows, err := s.db.Query(sqlList, string(core.StateCaptured), string(core.StateResting), nowStr, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list tend thoughts: query: %w", err)
	}
	defer rows.Close()

	thoughts := make([]core.Thought, 0, limit)
	for rows.Next() {
		var thought core.Thought

		var stateStr string
		var tendCounter int
		var createdAtStr, updatedAtStr string
		var lastTendedAtStr sql.NullString
		var eligibilityAtStr string
		var valence sql.NullInt64
		var energy sql.NullInt64

		err = rows.Scan(&thought.ID, &thought.Content, &stateStr, &tendCounter, &createdAtStr, &updatedAtStr, &lastTendedAtStr, &eligibilityAtStr, &valence, &energy)
		if err != nil {
			return nil, fmt.Errorf("list tend thoughts: scan: %w", err)
		}

		thought.CurrentState = core.State(stateStr)
		thought.TendCounter = tendCounter

		var err error
		thought.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAtStr)
		if err != nil {
			return nil, fmt.Errorf("list tend thoughts: parse created_at: %w", err)
		}

		thought.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAtStr)
		if err != nil {
			return nil, fmt.Errorf("list tend thoughts: parse updated_at: %w", err)
		}

		thought.EligibilityAt, err = time.Parse(time.RFC3339Nano, eligibilityAtStr)
		if err != nil {
			return nil, fmt.Errorf("list tend thoughts: parse eligibility_at: %w", err)
		}

		if lastTendedAtStr.Valid {
			t, err := time.Parse(time.RFC3339Nano, lastTendedAtStr.String)
			if err != nil {
				return nil, fmt.Errorf("list tend thoughts: parse last_tended_at: %w", err)
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

		thoughts = append(thoughts, thought)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list tend thoughts: rows: %w", err)
	}

	return thoughts, nil
}

func (s *Store) FilterViewByPagination(limit, offset int, filter string) ([]core.Thought, error) {
	if s == nil {
		return nil, fmt.Errorf("list view thoughts: store is nil")
	}
	if s.db == nil {
		return nil, fmt.Errorf("list view thoughts: db is nil")
	}
	if limit <= 0 {
		return nil, fmt.Errorf("list view thoughts: limit must be > 0")
	}
	if offset < 0 {
		return nil, fmt.Errorf("list view thoughts: offset must be >= 0")
	}

	sqlList := `SELECT id, content, current_state, tend_counter, created_at, updated_at, last_tended_at, eligibility_at, valence, energy
	            FROM thoughts
	            WHERE current_state IN (?)
	            ORDER BY id ASC
	            LIMIT ? OFFSET ?`

	rows, err := s.db.Query(sqlList, filter, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list view thoughts: query: %w", err)
	}
	defer rows.Close()

	thoughts := make([]core.Thought, 0, limit)
	for rows.Next() {
		var thought core.Thought

		var stateStr string
		var tendCounter int
		var createdAtStr, updatedAtStr string
		var lastTendedAtStr sql.NullString
		var eligibilityAtStr string
		var valence sql.NullInt64
		var energy sql.NullInt64

		err = rows.Scan(&thought.ID, &thought.Content, &stateStr, &tendCounter, &createdAtStr, &updatedAtStr, &lastTendedAtStr, &eligibilityAtStr, &valence, &energy)
		if err != nil {
			return nil, fmt.Errorf("list view thoughts: scan: %w", err)
		}

		thought.CurrentState = core.State(stateStr)
		thought.TendCounter = tendCounter

		var err error
		thought.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAtStr)
		if err != nil {
			return nil, fmt.Errorf("list view thoughts: parse created_at: %w", err)
		}

		thought.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAtStr)
		if err != nil {
			return nil, fmt.Errorf("list view thoughts: parse updated_at: %w", err)
		}

		thought.EligibilityAt, err = time.Parse(time.RFC3339Nano, eligibilityAtStr)
		if err != nil {
			return nil, fmt.Errorf("list view thoughts: parse eligibility_at: %w", err)
		}

		if lastTendedAtStr.Valid {
			t, err := time.Parse(time.RFC3339Nano, lastTendedAtStr.String)
			if err != nil {
				return nil, fmt.Errorf("list view thoughts: parse last_tended_at: %w", err)
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

		thoughts = append(thoughts, thought)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list view thoughts: rows: %w", err)
	}

	return thoughts, nil
}

// UpdateThoughtContent updates a thought's content and refreshed updated_at.
func (s *Store) UpdateThoughtContent(id int64, content string) error {
	if s == nil {
		return fmt.Errorf("update thought content: store is nil")
	}
	if s.db == nil {
		return fmt.Errorf("update thought content: db is nil")
	}
	if id <= 0 {
		return fmt.Errorf("update thought content: invalid thought ID")
	}
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("update thought content: content is empty")
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := s.db.Exec(
		`UPDATE thoughts SET content = ?, updated_at = ? WHERE id = ?`,
		content,
		now,
		id,
	)
	if err != nil {
		return fmt.Errorf("update thought content: update: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update thought content: rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("update thought content: no rows updated (id=%d)", id)
	}

	return nil
}

// MarkThoughtTended transitions a thought to tended, increments tend_counter, and appends a state-change event.
func (s *Store) MarkThoughtTended(id int64, note *string) error {
	if s == nil {
		return fmt.Errorf("mark thought tended: store is nil")
	}
	if s.db == nil {
		return fmt.Errorf("mark thought tended: db is nil")
	}
	if id <= 0 {
		return fmt.Errorf("mark thought tended: invalid thought ID")
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("mark thought tended: begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var prevStateStr string
	row := tx.QueryRow(`SELECT current_state FROM thoughts WHERE id = ?`, id)
	if err := row.Scan(&prevStateStr); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("mark thought tended: not found")
		}
		return fmt.Errorf("mark thought tended: read current_state: %w", err)
	}

	prev := core.State(prevStateStr)
	if prev == core.StateEvolved || prev == core.StateReleased || prev == core.StateArchived {
		return fmt.Errorf("mark thought tended: thought is in terminal state (%s)", prev)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	next := core.StateTended

	_, err = tx.Exec(
		`UPDATE thoughts
		 SET current_state = ?,
		     tend_counter = tend_counter + 1,
		     last_tended_at = ?,
		     updated_at = ?
		 WHERE id = ?`,
		string(next),
		now,
		now,
		id,
	)
	if err != nil {
		return fmt.Errorf("mark thought tended: update thoughts: %w", err)
	}

	var noteValue any
	if note != nil && strings.TrimSpace(*note) != "" {
		noteValue = *note
	} else {
		noteValue = nil
	}

	_, err = tx.Exec(
		`INSERT INTO events (thought_id, kind, at, previous_state, next_state, note)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		id,
		"state_change",
		now,
		string(prev),
		string(next),
		noteValue,
	)
	if err != nil {
		return fmt.Errorf("mark thought tended: insert event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("mark thought tended: commit: %w", err)
	}
	return nil
}


// TransitionPostTendResolutionStrict transitions a tended thought into resting or a terminal state and appends exactly one event.
func (s *Store) TransitionPostTendResolutionStrict(id int64, next core.State, note *string) error {
	if s == nil {
		return fmt.Errorf("post-tend transition: store is nil")
	}
	if s.db == nil {
		return fmt.Errorf("post-tend transition: db is nil")
	}
	if id <= 0 {
		return fmt.Errorf("post-tend transition: invalid thought ID")
	}

	if next != core.StateResting && next != core.StateEvolved && next != core.StateReleased && next != core.StateArchived {
		return fmt.Errorf("post-tend transition: invalid next state %q", next)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("post-tend transition: begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var prevStateStr string
	row := tx.QueryRow(`SELECT current_state FROM thoughts WHERE id = ?`, id)
	if err := row.Scan(&prevStateStr); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("post-tend transition: not found")
		}
		return fmt.Errorf("post-tend transition: read current_state: %w", err)
	}

	prev := core.State(prevStateStr)
	if prev != core.StateTended {
		return fmt.Errorf("post-tend transition: thought is not in tended state (currently %s)", prev)
	}

	nowTime := time.Now().UTC()
	now := nowTime.Format(time.RFC3339Nano)

	var noteValue any
	if note != nil && strings.TrimSpace(*note) != "" {
		noteValue = *note
	} else {
		noteValue = nil
	}

	if next == core.StateResting {
		eligibilityAt := nowTime.Add(core.SettleDuration).Format(time.RFC3339Nano)
		_, err = tx.Exec(
			`UPDATE thoughts
			 SET current_state = ?,
			     updated_at = ?,
			     eligibility_at = ?
			 WHERE id = ?`,
			string(next),
			now,
			eligibilityAt,
			id,
		)
		if err != nil {
			return fmt.Errorf("post-tend transition: update thoughts (rest): %w", err)
		}
	} else {
		_, err = tx.Exec(
			`UPDATE thoughts
			 SET current_state = ?,
			     updated_at = ?
			 WHERE id = ?`,
			string(next),
			now,
			id,
		)
		if err != nil {
			return fmt.Errorf("post-tend transition: update thoughts: %w", err)
		}
	}

	_, err = tx.Exec(
		`INSERT INTO events (thought_id, kind, at, previous_state, next_state, note)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		id,
		"state_change",
		now,
		string(prev),
		string(next),
		noteValue,
	)
	if err != nil {
		return fmt.Errorf("post-tend transition: insert event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("post-tend transition: commit: %w", err)
	}

	return nil
}

func (s *Store) ToEvolve(id int64) error {
	if s == nil {
		return fmt.Errorf("post-tend transition: store is nil")
	}
	if s.db == nil {
		return fmt.Errorf("post-tend transition: db is nil")
	}
	if id <= 0 {
		return fmt.Errorf("post-tend transition: invalid thought ID")
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("to evolve: begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var prevStateStr string
	row := tx.QueryRow(`SELECT current_state FROM thoughts WHERE id = ?`, id)
	if err := row.Scan(&prevStateStr); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("to evolve: not found")
		}
		return fmt.Errorf("to evolve: read current_state: %w", err)
	}

	prev := core.State(prevStateStr)
	if prev == core.StateEvolved {
		return fmt.Errorf("to evolve: thought is in terminal state (%s)", prev)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	state := core.StateEvolved

	sqlQuery := `UPDATE thoughts
		 		 SET current_state = ?,
		     		updated_at = ?
		 		 WHERE id = ?`
	res, err := tx.Exec(sqlQuery, state, now, id)

	if err != nil {
		return fmt.Errorf("to evolve: update thoughts: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("to evolve: rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("to evolve: not found")
	}

	_, err = tx.Exec(`INSERT INTO events (thought_id, kind, at, previous_state, next_state, note) VALUES (?, ?, ?, ?, ?, ?)`, id, "state_change", now, string(prev), string(state), nil)
	if err != nil {
		return fmt.Errorf("to evolve: insert event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("to evolve: commit: %w", err)
	}
	return nil
}

// ReleaseThought permanently deletes a thought and its associated events.
func (s *Store) ReleaseThought(id int64) error {
	if s == nil {
		return fmt.Errorf("release thought: store is nil")
	}
	if s.db == nil {
		return fmt.Errorf("release thought: db is nil")
	}
	if id <= 0 {
		return fmt.Errorf("release thought: invalid thought ID")
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("release thought: begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	_, err = tx.Exec(`DELETE FROM events WHERE thought_id = ?`, id)
	if err != nil {
		return fmt.Errorf("release thought: delete events: %w", err)
	}

	res, err := tx.Exec(`DELETE FROM thoughts WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("release thought: delete thought: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("release thought: rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("release thought: not found")
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("release thought: commit: %w", err)
	}
	return nil
}

// CountTendReady returns the number of thoughts currently eligible for tending.
func (s *Store) CountTendReady() (int, error) {
	if s == nil {
		return 0, fmt.Errorf("count tend ready: store is nil")
	}
	if s.db == nil {
		return 0, fmt.Errorf("count tend ready: db is nil")
	}

	nowStr := time.Now().UTC().Format(time.RFC3339Nano)
	var n int
	err := s.db.QueryRow(
		`SELECT COUNT(*)
		 FROM thoughts
		 WHERE current_state IN (?, ?)
		   AND eligibility_at <= ?`,
		string(core.StateCaptured),
		string(core.StateResting),
		nowStr,
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count tend ready: query: %w", err)
	}
	return n, nil
}

// DidCountTendChange returns true when the supplied count differs from the last persisted value.
// It updates the persisted value on change.
func (s *Store) DidCountTendChange(newCount int) bool {
	if s == nil || s.db == nil {
		return false
	}
	if newCount < 0 {
		return false
	}

	_ = s.ensureAppStateTable()

	var oldValue string
	err := s.db.QueryRow(`SELECT value FROM app_state WHERE key = ?`, appStateKeyLastTendReadyCount).Scan(&oldValue)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			_ = s.setAppStateInt(appStateKeyLastTendReadyCount, newCount)
			return true
		}
		return false
	}

	oldCount, err := strconv.Atoi(strings.TrimSpace(oldValue))
	if err != nil {
		_ = s.setAppStateInt(appStateKeyLastTendReadyCount, newCount)
		return true
	}

	if oldCount == newCount {
		return false
	}

	_ = s.setAppStateInt(appStateKeyLastTendReadyCount, newCount)
	return true
}

func (s *Store) ensureAppStateTable() error {
	if s == nil || s.db == nil {
		return fmt.Errorf("ensure app_state: store/db is nil")
	}
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS app_state (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);
	`)
	if err != nil {
		return fmt.Errorf("ensure app_state: %w", err)
	}
	return nil
}

func (s *Store) setAppStateInt(key string, value int) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("set app_state: store/db is nil")
	}
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("set app_state: empty key")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.Exec(
		`INSERT INTO app_state(key, value, updated_at)
		 VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key,
		strconv.Itoa(value),
		now,
	)
	if err != nil {
		return fmt.Errorf("set app_state: upsert: %w", err)
	}
	return nil
}

// ReindexThoughtIDs renumbers thought IDs to be contiguous (1..N) and rewrites event foreign keys.
// This is a UX nicety for a local-only CLI and is intended to be called after deletions.
func (s *Store) ReindexThoughtIDs() error {
	if s == nil {
		return fmt.Errorf("reindex thought ids: store is nil")
	}
	if s.db == nil {
		return fmt.Errorf("reindex thought ids: db is nil")
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("reindex thought ids: begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Map old IDs to new contiguous IDs.
	_, err = tx.Exec(`CREATE TEMP TABLE IF NOT EXISTS thought_id_map (old_id INTEGER PRIMARY KEY, new_id INTEGER NOT NULL);`)
	if err != nil {
		return fmt.Errorf("reindex thought ids: create map: %w", err)
	}
	_, err = tx.Exec(`DELETE FROM thought_id_map;`)
	if err != nil {
		return fmt.Errorf("reindex thought ids: clear map: %w", err)
	}
	_, err = tx.Exec(`
		INSERT INTO thought_id_map(old_id, new_id)
		SELECT id, ROW_NUMBER() OVER (ORDER BY id)
		FROM thoughts
		ORDER BY id;
	`)
	if err != nil {
		return fmt.Errorf("reindex thought ids: populate map: %w", err)
	}

	// Create new tables.
	_, err = tx.Exec(`
		CREATE TABLE thoughts_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			content TEXT NOT NULL,
			current_state TEXT NOT NULL,
			tend_counter INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			last_tended_at TEXT NULL,
			eligibility_at TEXT NOT NULL,
			valence INTEGER NULL,
			energy INTEGER NULL
		);
	`)
	if err != nil {
		return fmt.Errorf("reindex thought ids: create thoughts_new: %w", err)
	}

	_, err = tx.Exec(`
		CREATE TABLE events_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			thought_id INTEGER NOT NULL,
			kind TEXT NOT NULL,
			at TEXT NOT NULL,
			previous_state TEXT NULL,
			next_state TEXT NULL,
			note TEXT NULL,
			FOREIGN KEY(thought_id) REFERENCES thoughts_new(id)
		);
	`)
	if err != nil {
		return fmt.Errorf("reindex thought ids: create events_new: %w", err)
	}

	// Copy data with remapped IDs.
	_, err = tx.Exec(`
		INSERT INTO thoughts_new (id, content, current_state, tend_counter, created_at, updated_at, last_tended_at, eligibility_at, valence, energy)
		SELECT m.new_id, t.content, t.current_state, t.tend_counter, t.created_at, t.updated_at, t.last_tended_at, t.eligibility_at, t.valence, t.energy
		FROM thoughts t
		JOIN thought_id_map m ON m.old_id = t.id
		ORDER BY m.new_id;
	`)
	if err != nil {
		return fmt.Errorf("reindex thought ids: copy thoughts: %w", err)
	}

	_, err = tx.Exec(`
		INSERT INTO events_new (id, thought_id, kind, at, previous_state, next_state, note)
		SELECT e.id, m.new_id, e.kind, e.at, e.previous_state, e.next_state, e.note
		FROM events e
		JOIN thought_id_map m ON m.old_id = e.thought_id
		ORDER BY e.id;
	`)
	if err != nil {
		return fmt.Errorf("reindex thought ids: copy events: %w", err)
	}

	// Drop old tables and swap in the new ones.
	_, err = tx.Exec(`DROP TABLE events;`)
	if err != nil {
		return fmt.Errorf("reindex thought ids: drop events: %w", err)
	}
	_, err = tx.Exec(`DROP TABLE thoughts;`)
	if err != nil {
		return fmt.Errorf("reindex thought ids: drop thoughts: %w", err)
	}

	_, err = tx.Exec(`ALTER TABLE thoughts_new RENAME TO thoughts;`)
	if err != nil {
		return fmt.Errorf("reindex thought ids: rename thoughts: %w", err)
	}
	_, err = tx.Exec(`ALTER TABLE events_new RENAME TO events;`)
	if err != nil {
		return fmt.Errorf("reindex thought ids: rename events: %w", err)
	}

	// Recreate indexes.
	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_thoughts_state_eligibility ON thoughts(current_state, eligibility_at);`)
	if err != nil {
		return fmt.Errorf("reindex thought ids: create idx_thoughts_state_eligibility: %w", err)
	}
	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_events_thought_id_at ON events(thought_id, at);`)
	if err != nil {
		return fmt.Errorf("reindex thought ids: create idx_events_thought_id_at: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("reindex thought ids: commit: %w", err)
	}
	return nil
}
