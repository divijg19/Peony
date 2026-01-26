package core

import (
	"time"
)

// SettleDuration is added to a newly captured thought to compute its eligibility time.
const SettleDuration = 18 * time.Hour

// EligibleToSurface reports whether a thought is eligible to be tended at time now.
func EligibleToSurface(thought Thought, now time.Time) bool {
	switch thought.CurrentState {
		case StateCaptured, StateResting:

		case StateEvolved, StateReleased, StateArchived:
			return false
		default:
			return false
	}

	if thought.EligibilityAt.IsZero() {
		return false
	}
	return !now.Before(thought.EligibilityAt)
}
