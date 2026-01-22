package core

import (
	"time"
)

// SettleDuration prevents freshly-captured thoughts from resurfacing immediately.
const SettleDuration = 18 * time.Hour

// DefaultTendCooldown throttles how soon a recently tended thought can resurface.
const DefaultTendCooldown = 36 * time.Hour

// EligibleToSurface reports whether a thought is eligible to be surfaced by `tend`.
func EligibleToSurface(thought Thought, now time.Time) (bool) {
	if thought.State == StateReleased || thought.State == StateArchived || thought.State == StateEvolved {
		return false
	}

	if !thought.CreatedAt.IsZero() && now.Sub(thought.CreatedAt) < SettleDuration {
		return false
	}

	if thought.RestUntil != nil && now.Before(*thought.RestUntil) {
		return false
	}

	if thought.LastTendedAt != nil && now.Sub(*thought.LastTendedAt) < DefaultTendCooldown {
		return false
	}

	return true
}

// ComputeRestUntil returns the next rest boundary given a rest count.
func ComputeRestUntil(now time.Time, restCount int) (time.Time) {
	var d time.Duration
	switch restCount {
	case 0:
		d = 24 * time.Hour
	case 1:
		d = 72 * time.Hour
	case 2:
		d = 168 * time.Hour
	default:
		d = 336 * time.Hour
	}
	return now.Add(d)
}
