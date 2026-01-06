package tests

import (
	"testing"
	"time"

	"github.com/ri5hii/peony/internal/core"
)

func TestEligibleToSurface_ClosedStates(t *testing.T) {
	now := time.Now().UTC()
	base := core.Thought{CreatedAt: now.Add(-10 * 24 * time.Hour)}

	closed := []core.State{core.StateReleased, core.StateArchived, core.StateEvolved}
	for _, st := range closed {
		th := base
		th.State = st
		if core.EligibleToSurface(th, now) {
			t.Fatalf("expected state %q to be ineligible", st)
		}
	}
}

func TestEligibleToSurface_SettleDuration(t *testing.T) {
	now := time.Now().UTC()

	tooFresh := core.Thought{
		State:     core.StateCaptured,
		CreatedAt: now.Add(-core.SettleDuration / 2),
	}
	if core.EligibleToSurface(tooFresh, now) {
		t.Fatalf("expected thought within settle duration to be ineligible")
	}

	oldEnough := core.Thought{
		State:     core.StateCaptured,
		CreatedAt: now.Add(-(core.SettleDuration + time.Minute)),
	}
	if !core.EligibleToSurface(oldEnough, now) {
		t.Fatalf("expected thought past settle duration to be eligible")
	}
}

func TestEligibleToSurface_RestUntil(t *testing.T) {
	now := time.Now().UTC()
	future := now.Add(2 * time.Hour)
	past := now.Add(-2 * time.Hour)

	th := core.Thought{State: core.StateResting, CreatedAt: now.Add(-10 * 24 * time.Hour), RestUntil: &future}
	if core.EligibleToSurface(th, now) {
		t.Fatalf("expected thought with future rest_until to be ineligible")
	}

	th.RestUntil = &past
	if !core.EligibleToSurface(th, now) {
		t.Fatalf("expected thought with past rest_until to be eligible")
	}
}

func TestEligibleToSurface_TendCooldown(t *testing.T) {
	now := time.Now().UTC()
	recent := now.Add(-core.DefaultTendCooldown / 2)
	old := now.Add(-(core.DefaultTendCooldown + time.Minute))

	th := core.Thought{State: core.StateTended, CreatedAt: now.Add(-10 * 24 * time.Hour), LastTendedAt: &recent}
	if core.EligibleToSurface(th, now) {
		t.Fatalf("expected thought within tend cooldown to be ineligible")
	}

	th.LastTendedAt = &old
	if !core.EligibleToSurface(th, now) {
		t.Fatalf("expected thought past tend cooldown to be eligible")
	}
}

func TestComputeRestUntil(t *testing.T) {
	now := time.Date(2026, 1, 6, 7, 0, 0, 0, time.UTC)

	cases := []struct {
		restCount int
		want      time.Time
	}{
		{0, now.Add(24 * time.Hour)},
		{1, now.Add(72 * time.Hour)},
		{2, now.Add(168 * time.Hour)},
		{3, now.Add(336 * time.Hour)},
		{10, now.Add(336 * time.Hour)},
	}

	for _, tc := range cases {
		got := core.ComputeRestUntil(now, tc.restCount)
		if !got.Equal(tc.want) {
			t.Fatalf("restCount=%d: got %s, want %s", tc.restCount, got.Format(time.RFC3339), tc.want.Format(time.RFC3339))
		}
	}
}
