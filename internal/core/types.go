package core

import (
	"time"
)

// State represents the lifecycle state of a thought.
type State string

const (
	StateCaptured State = "captured"
	StateResting  State = "resting"
	StateTended   State = "tended"
	StateEvolved  State = "evolved"
	StateReleased State = "released"
	StateArchived State = "archived"
)

// Thought represents the current snapshot of a cognitive unit.
type Thought struct {
	ID            int64      `db:"id"`
	Content       string     `db:"content"`
	CurrentState  State      `db:"current_state"`
	TendCounter   int        `db:"tend_counter"`
	CreatedAt     time.Time  `db:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at"`
	LastTendedAt  *time.Time `db:"last_tended_at"`
	EligibilityAt time.Time  `db:"eligibility_at"`
	Valence       *int       `db:"valence"`
	Energy        *int       `db:"energy"`
}

// Event represents a single append-only history record for a thought.
type Event struct {
	ID            int64     `db:"id"`
	ThoughtID     int64     `db:"thought_id"`
	Kind          string    `db:"kind"`
	At            time.Time `db:"at"`
	PreviousState *State    `db:"previous_state"`
	NextState     *State    `db:"next_state"`
	Note          *string   `db:"note"`
}
