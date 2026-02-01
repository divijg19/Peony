package core

import (
	"time"
)

// SettleDuration defines how long a thought must rest before it becomes eligible to be tended.
// It can be overridden via configuration.
var SettleDuration = 18 * time.Hour

// EligibleToSurface reports whether a thought is eligible to be tended at the given time.
func EligibleToSurface(thought Thought, now time.Time) bool {
	// Only captured and resting thoughts can surface for tending.
	switch thought.CurrentState {
	case StateCaptured, StateResting:
		// eligible states
	case StateEvolved, StateReleased, StateArchived:
		// terminal states are never eligible
		return false
	default:
		// unknown states are treated as ineligible
		return false
	}

	// A zero eligibility timestamp is treated as not eligible.
	if thought.EligibilityAt.IsZero() {
		return false
	}

	// Eligibility is reached once now is at or after eligibility_at.
	return !now.Before(thought.EligibilityAt)
}
