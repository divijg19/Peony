package core

import (
	"time"
)

// State is the lifecycle state of a thought.
type State string

const (
	StateCaptured State = "captured"
	StateResting  State = "resting"
	StateTended   State = "tended"
	StateEvolved  State = "evolved"
	StateReleased State = "released"
	StateArchived State = "archived"
)

// Thought is the core cognitive unit stored in Peony.
type Thought struct {
	ID           int64      `db:"id"`
	Content      string     `db:"content"`
	State        State      `db:"state"`
	CreatedAt    time.Time  `db:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at"`
	RestUntil    *time.Time `db:"rest_until"`
	LastTendedAt *time.Time `db:"last_tended_at"`
	Valence      *int       `db:"valence"`
	Energy       *int       `db:"energy"`
}

// Event records a gentle interaction in a thought's history.
type Event struct {
	ID        int64     `db:"id"`
	ThoughtID int64     `db:"thought_id"`
	Kind      string    `db:"kind"`
	At        time.Time `db:"at"`
	Note      *string   `db:"note"`
}
