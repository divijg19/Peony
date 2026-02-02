package tests

import (
	"testing"
	"time"

	"github.com/ri5hii/peony/internal/core"
)

func TestEligibleToSurface_ClosedStates(t *testing.T) {
	now := time.Now().UTC()
	base := core.Thought{CreatedAt: now.Add(-10 * 24 * time.Hour), EligibilityAt: now.Add(-time.Hour)}

	closed := []core.State{core.StateReleased, core.StateArchived, core.StateEvolved}
	for _, st := range closed {
		th := base
		th.CurrentState = st
		if core.EligibleToSurface(th, now) {
			t.Fatalf("expected state %q to be ineligible", st)
		}
	}
}

func TestEligibleToSurface_Captured_UsesEligibilityAt(t *testing.T) {
	now := time.Now().UTC()
	th := core.Thought{CurrentState: core.StateCaptured, CreatedAt: now.Add(-10 * 24 * time.Hour), EligibilityAt: now.Add(time.Hour)}
	if core.EligibleToSurface(th, now) {
		t.Fatalf("expected captured thought with future eligibility_at to be ineligible")
	}

	th.EligibilityAt = now.Add(-time.Minute)
	if !core.EligibleToSurface(th, now) {
		t.Fatalf("expected captured thought with past eligibility_at to be eligible")
	}
}

func TestEligibleToSurface_Resting_UsesEligibilityAt(t *testing.T) {
	now := time.Now().UTC()
	th := core.Thought{CurrentState: core.StateResting, CreatedAt: now.Add(-10 * 24 * time.Hour), EligibilityAt: now.Add(-time.Minute)}
	if !core.EligibleToSurface(th, now) {
		t.Fatalf("expected resting thought with past eligibility_at to be eligible")
	}
}
