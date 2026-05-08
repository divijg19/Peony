package tui

import (
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/divijg19/peony/internal/app"
	"github.com/divijg19/peony/internal/core"
	"github.com/divijg19/peony/internal/storage"
)

func newTestModel(t *testing.T) Model {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "peony.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	service, err := app.NewForDB(db)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	return NewModel(service)
}

func withReadyThoughts(t *testing.T) {
	t.Helper()
	previous := core.SettleDuration
	core.SettleDuration = 0
	t.Cleanup(func() {
		core.SettleDuration = previous
	})
}

func press(m Model, key tea.KeyMsg) Model {
	next, _ := m.Update(key)
	return next.(Model)
}

func runeKey(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func TestModelAddSaveAndCancel(t *testing.T) {
	withReadyThoughts(t)
	m := newTestModel(t)

	m = press(m, runeKey('a'))
	if m.mode != modeAdd {
		t.Fatalf("mode = %v, want add", m.mode)
	}
	m.addBox.SetValue("a quiet thought")
	m = press(m, tea.KeyMsg{Type: tea.KeyCtrlS})
	if m.mode != modeBrowse {
		t.Fatalf("mode after save = %v, want browse", m.mode)
	}
	if len(m.snapshot.Thoughts) != 1 {
		t.Fatalf("thought count = %d, want 1", len(m.snapshot.Thoughts))
	}
	if got := m.snapshot.Thoughts[0].Thought.Content; got != "a quiet thought" {
		t.Fatalf("content = %q", got)
	}

	m = press(m, runeKey('a'))
	m.addBox.SetValue("cancel me")
	m = press(m, tea.KeyMsg{Type: tea.KeyEsc})
	if len(m.snapshot.Thoughts) != 1 {
		t.Fatalf("thought count after cancel = %d, want 1", len(m.snapshot.Thoughts))
	}
}

func TestModelTendThenRest(t *testing.T) {
	withReadyThoughts(t)
	m := newTestModel(t)
	id, err := m.service.Capture("rough edge")
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	m.reloadPreserving(id)

	m = press(m, runeKey('t'))
	if m.mode != modeTend {
		t.Fatalf("mode = %v, want tend", m.mode)
	}
	m.tendContent.SetValue("rough edge, softened")
	m.tendNote.SetValue("left a note")
	m = press(m, tea.KeyMsg{Type: tea.KeyCtrlS})
	if m.mode != modeBrowse {
		t.Fatalf("mode after tend = %v, want browse", m.mode)
	}
	if got := m.snapshot.Thoughts[m.selected].Thought.CurrentState; got != core.StateTended {
		t.Fatalf("state after tend = %s, want tended", got)
	}

	m = press(m, runeKey('r'))
	if got := m.snapshot.Thoughts[m.selected].Thought.CurrentState; got != core.StateResting {
		t.Fatalf("state after rest = %s, want resting", got)
	}
	if !m.snapshot.Thoughts[m.selected].Thought.EligibilityAt.After(time.Now().UTC().Add(-time.Second)) {
		t.Fatal("rest should refresh eligibility time")
	}
}

func TestModelFilteringSearchAndNavigation(t *testing.T) {
	withReadyThoughts(t)
	m := newTestModel(t)
	firstID, err := m.service.Capture("alpha seed")
	if err != nil {
		t.Fatalf("capture first: %v", err)
	}
	if _, err := m.service.Capture("beta seed"); err != nil {
		t.Fatalf("capture second: %v", err)
	}
	if err := m.service.Evolve(firstID); err != nil {
		t.Fatalf("evolve first: %v", err)
	}
	m.reloadPreserving(0)

	m = press(m, tea.KeyMsg{Type: tea.KeyDown})
	if m.selected != 1 {
		t.Fatalf("selected = %d, want 1", m.selected)
	}

	m = press(m, runeKey('/'))
	m.search.SetValue("beta")
	m = press(m, tea.KeyMsg{Type: tea.KeyEnter})
	if len(m.snapshot.Thoughts) != 1 || m.snapshot.Thoughts[0].Thought.Content != "beta seed" {
		t.Fatalf("unexpected search results: %+v", m.snapshot.Thoughts)
	}

	m = press(m, runeKey('f'))
	if m.filter != core.StateCaptured {
		t.Fatalf("filter = %q, want captured", m.filter)
	}
	if len(m.snapshot.Thoughts) != 1 {
		t.Fatalf("captured filter count = %d, want 1", len(m.snapshot.Thoughts))
	}
}

func TestModelEvolveArchiveAndReleaseCancel(t *testing.T) {
	withReadyThoughts(t)
	m := newTestModel(t)
	firstID, err := m.service.Capture("first")
	if err != nil {
		t.Fatalf("capture first: %v", err)
	}
	secondID, err := m.service.Capture("second")
	if err != nil {
		t.Fatalf("capture second: %v", err)
	}
	m.reloadPreserving(firstID)

	m = press(m, runeKey('e'))
	if got := m.snapshot.Thoughts[m.selected].Thought.CurrentState; got != core.StateEvolved {
		t.Fatalf("state after evolve = %s, want evolved", got)
	}

	m.reloadPreserving(secondID)
	m = press(m, runeKey('A'))
	if got := m.snapshot.Thoughts[m.selected].Thought.CurrentState; got != core.StateArchived {
		t.Fatalf("state after archive = %s, want archived", got)
	}

	m = press(m, runeKey('x'))
	if m.mode != modeReleaseConfirm {
		t.Fatalf("mode = %v, want release confirm", m.mode)
	}
	m = press(m, runeKey('n'))
	if m.mode != modeBrowse {
		t.Fatalf("mode after cancel = %v, want browse", m.mode)
	}
	if len(m.snapshot.Thoughts) != 2 {
		t.Fatalf("thought count after cancel = %d, want 2", len(m.snapshot.Thoughts))
	}
}

func TestModelReleasePermanentConfirmation(t *testing.T) {
	withReadyThoughts(t)
	m := newTestModel(t)
	if _, err := m.service.Capture("first"); err != nil {
		t.Fatalf("capture first: %v", err)
	}
	if _, err := m.service.Capture("second"); err != nil {
		t.Fatalf("capture second: %v", err)
	}
	m.reloadPreserving(1)

	m = press(m, runeKey('x'))
	if m.mode != modeReleaseConfirm {
		t.Fatalf("mode = %v, want release confirm", m.mode)
	}
	m = press(m, runeKey('y'))
	if len(m.snapshot.Thoughts) != 1 {
		t.Fatalf("thought count after release = %d, want 1", len(m.snapshot.Thoughts))
	}
	if got := m.snapshot.Thoughts[0].Thought.ID; got != 1 {
		t.Fatalf("remaining id = %d, want reindexed 1", got)
	}
	if got := m.snapshot.Thoughts[0].Thought.Content; got != "second" {
		t.Fatalf("remaining content = %q, want second", got)
	}
}
