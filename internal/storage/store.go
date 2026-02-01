package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ri5hii/peony/internal/core"
)

// Store provides SQLite-backed persistence for thoughts and events.
type Store struct {
	db *sql.DB
}

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
	// created_at/updated_at are recorded in UTC; eligibility_at is computed from SettleDuration.
	nowTime := time.Now().UTC()
	now := nowTime.Format(time.RFC3339Nano)
	eligibilityAt := nowTime.Add(core.SettleDuration).Format(time.RFC3339Nano)
	state := core.StateCaptured
	// Insert the initial thought row.
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

	// Translate optional values into SQL NULLs.
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

	// Parse stored timestamps.
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

	// Load the append-only event history in chronological order.
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

// ReleaseThought permanently removes a thought and its events.
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

	// Delete dependent events first.
	_, err = tx.Exec(`DELETE FROM events WHERE thought_id = ?`, id)
	if err != nil {
		return fmt.Errorf("release thought: delete events: %w", err)
	}

	res, err := tx.Exec(`DELETE FROM thoughts WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("release thought: delete thought: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("release thought: rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("release thought: not found")
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("release thought: commit: %w", err)
	}
	return nil
}

// ReindexThoughtIDs renumbers thoughts.id into a dense 1..N sequence and updates events.thought_id to match.
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

	// Defer FK checks to commit so we can update parent/child IDs safely.
	_, _ = tx.Exec(`PRAGMA defer_foreign_keys = ON;`)

	rows, err := tx.Query(`SELECT id FROM thoughts ORDER BY id ASC`)
	if err != nil {
		return fmt.Errorf("reindex thought ids: list ids: %w", err)
	}
	defer rows.Close()

	type mapping struct {
		oldID int64
		newID int64
	}

	maps := make([]mapping, 0)
	var nextID int64 = 1
	for rows.Next() {
		var old int64
		if err := rows.Scan(&old); err != nil {
			return fmt.Errorf("reindex thought ids: scan id: %w", err)
		}
		maps = append(maps, mapping{oldID: old, newID: nextID})
		nextID++
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("reindex thought ids: ids rows: %w", err)
	}

	if len(maps) == 0 {
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("reindex thought ids: commit: %w", err)
		}
		return nil
	}

	//move all mapped IDs to temporary negative values to avoid collisions.
	for _, m := range maps {
		tmp := -m.newID
		if _, err := tx.Exec(`UPDATE thoughts SET id = ? WHERE id = ?`, tmp, m.oldID); err != nil {
			return fmt.Errorf("reindex thought ids: tmp update thoughts: %w", err)
		}
	}
	for _, m := range maps {
		tmp := -m.newID
		if _, err := tx.Exec(`UPDATE events SET thought_id = ? WHERE thought_id = ?`, tmp, m.oldID); err != nil {
			return fmt.Errorf("reindex thought ids: tmp update events: %w", err)
		}
	}

	//flip temporary negative IDs back to positive.
	if _, err := tx.Exec(`UPDATE thoughts SET id = -id WHERE id < 0`); err != nil {
		return fmt.Errorf("reindex thought ids: finalize thoughts: %w", err)
	}
	if _, err := tx.Exec(`UPDATE events SET thought_id = -thought_id WHERE thought_id < 0`); err != nil {
		return fmt.Errorf("reindex thought ids: finalize events: %w", err)
	}

	// Reset AUTOINCREMENT sequence so newly created thoughts continue from max(id).
	// sqlite_sequence exists once an AUTOINCREMENT table has been inserted into.
	_, _ = tx.Exec(`UPDATE sqlite_sequence
		SET seq = (SELECT COALESCE(MAX(id), 0) FROM thoughts)
		WHERE name = 'thoughts'`)
	_, _ = tx.Exec(`INSERT INTO sqlite_sequence(name, seq)
		SELECT 'thoughts', (SELECT COALESCE(MAX(id), 0) FROM thoughts)
		WHERE NOT EXISTS (SELECT 1 FROM sqlite_sequence WHERE name = 'thoughts')`)

	// Validate referential integrity before committing.
	checkRows, err := tx.Query(`PRAGMA foreign_key_check;`)
	if err != nil {
		return fmt.Errorf("reindex thought ids: foreign_key_check: %w", err)
	}
	defer checkRows.Close()
	if checkRows.Next() {
		return fmt.Errorf("reindex thought ids: foreign_key_check failed")
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("reindex thought ids: commit: %w", err)
	}
	return nil
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
				WHERE current_state != ?
                ORDER BY updated_at ASC, id ASC
                LIMIT ? OFFSET ?`

	rows, err := s.db.Query(sqlList, "archived", limit, offset)
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

// CountTendReady returns the number of thoughts currently eligible to be tended.
func (s *Store) CountTendReady() (int, error) {
	if s == nil {
		return 0, fmt.Errorf("count tend ready: store is nil")
	}
	if s.db == nil {
		return 0, fmt.Errorf("count tend ready: db is nil")
	}

	nowStr := time.Now().UTC().Format(time.RFC3339Nano)

	var n int
	sqlQuery := `SELECT COUNT(*)
         		 FROM thoughts
         		 WHERE current_state IN (?, ?)
           		 	AND eligibility_at <= ?`
	err := s.db.QueryRow(sqlQuery, string(core.StateCaptured), string(core.StateResting), nowStr).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count tend ready: scan: %w", err)
	}
	return n, nil
}

// DidCountTendChange returns true when n differs from the last value recorded.
//
// This is used to avoid printing the tend notice repeatedly across multiple
// `peony` invocations. The last value is persisted in a small temp file.
func (s *Store) DidCountTendChange(n int) bool {
	path := filepath.Join(os.TempDir(), "peony_last_tendReady_count")
	file, err := os.Open(path)
	if err != nil {
		// First time (or missing file): create it and treat as "changed".
		file, err = os.Create(path)
		if err != nil {
			return true
		}
		defer file.Close()

		_, _ = fmt.Fprintf(file, "%d\n", n)
		return true
	}
	defer file.Close()

	var prev int
	_, err = fmt.Fscan(file, &prev)
	if err != nil {
		// Corrupt/empty file: overwrite and treat as "changed".
		f2, err2 := os.Create(path)
		if err2 == nil {
			_, _ = fmt.Fprintf(f2, "%d\n", n)
			_ = f2.Close()
		}
		return true
	}

	if prev == n {
		return false
	}

	// Value changed: overwrite stored value.
	f2, err := os.Create(path)
	if err == nil {
		_, _ = fmt.Fprintf(f2, "%d\n", n)
		_ = f2.Close()
	}
	return true
}

// FilterViewByPagination returns a page of thoughts whose state matches filter.
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

	// Use a transaction so the thought update and event append happen atomically.
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
