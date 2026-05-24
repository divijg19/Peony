package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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

func withSettleDuration(t *testing.T, duration time.Duration) {
	t.Helper()
	previous := core.SettleDuration
	core.SettleDuration = duration
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

func sized(m Model, width, height int) Model {
	next, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return next.(Model)
}

func assertViewFits(t *testing.T, m Model, width, height int) string {
	t.Helper()
	m = sized(m, width, height)
	view := m.View()
	if got := lipgloss.Width(view); got > width {
		t.Fatalf("view width = %d, want <= %d\n%s", got, width, view)
	}
	if got := lipgloss.Height(view); got > height {
		t.Fatalf("view height = %d, want <= %d\n%s", got, height, view)
	}
	return view
}

func TestModelCaptureSaveCancelAndEmptyValidation(t *testing.T) {
	withSettleDuration(t, 0)
	m := newTestModel(t)

	m = press(m, runeKey('a'))
	if m.mode != ModeCapture {
		t.Fatalf("mode = %v, want capture", m.mode)
	}
	m = press(m, tea.KeyMsg{Type: tea.KeyCtrlS})
	if m.mode != ModeCapture {
		t.Fatalf("empty save should stay in capture mode, got %v", m.mode)
	}
	if !strings.Contains(m.status, "empty") {
		t.Fatalf("empty save status = %q, want validation", m.status)
	}

	m.addBox.SetValue("a quiet thought")
	m = press(m, tea.KeyMsg{Type: tea.KeyCtrlS})
	if m.mode != ModeBrowse {
		t.Fatalf("mode after save = %v, want browse", m.mode)
	}
	if len(m.snapshot.Thoughts) != 1 || m.snapshot.Thoughts[0].Thought.Content != "a quiet thought" {
		t.Fatalf("unexpected queue after save: %+v", m.snapshot.Thoughts)
	}

	m = press(m, runeKey('a'))
	m.addBox.SetValue("cancel me")
	m = press(m, tea.KeyMsg{Type: tea.KeyEsc})
	if len(m.snapshot.Thoughts) != 1 {
		t.Fatalf("thought count after cancel = %d, want 1", len(m.snapshot.Thoughts))
	}
}

func TestModelTendSaveCancelAndRest(t *testing.T) {
	withSettleDuration(t, 0)
	m := newTestModel(t)
	id, err := m.service.Capture("rough edge")
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	m.reloadPreserving(id)

	m = press(m, runeKey('t'))
	if m.mode != ModeTend {
		t.Fatalf("mode = %v, want tend", m.mode)
	}
	m.tendContent.SetValue("rough edge, softened")
	m.tendNote.SetValue("left a note")
	m = press(m, tea.KeyMsg{Type: tea.KeyCtrlS})
	if m.mode != ModeBrowse {
		t.Fatalf("mode after tend = %v, want browse", m.mode)
	}
	item, ok := m.selectedItem()
	if !ok || item.Thought.CurrentState != core.StateTended {
		t.Fatalf("expected selected tended thought, got %+v", item.Thought)
	}

	m = press(m, runeKey('r'))
	item, ok = m.selectedItem()
	if !ok || item.Thought.CurrentState != core.StateResting {
		t.Fatalf("expected rested thought, got %+v", item.Thought)
	}

}

func TestFocusedQueueFilteringSearchAndOrdering(t *testing.T) {
	m := newTestModel(t)
	withSettleDuration(t, 0)
	readyID, err := m.service.Capture("ready alpha")
	if err != nil {
		t.Fatalf("capture ready: %v", err)
	}
	tendedID, err := m.service.Capture("tended beta")
	if err != nil {
		t.Fatalf("capture tended: %v", err)
	}
	if err := m.service.Tend(tendedID, "tended beta", nil); err != nil {
		t.Fatalf("tend beta: %v", err)
	}

	previous := core.SettleDuration
	core.SettleDuration = time.Hour
	restingID, err := m.service.Capture("settling gamma")
	core.SettleDuration = previous
	if err != nil {
		t.Fatalf("capture resting: %v", err)
	}

	memoryID, err := m.service.Capture("memory delta")
	if err != nil {
		t.Fatalf("capture memory: %v", err)
	}
	if err := m.service.Evolve(memoryID); err != nil {
		t.Fatalf("evolve memory: %v", err)
	}

	m.reloadPreserving(0)
	if m.filter != FilterReady {
		t.Fatalf("filter = %v, want ready", m.filter)
	}
	if len(m.snapshot.Thoughts) != 2 {
		t.Fatalf("ready queue count = %d, want 2", len(m.snapshot.Thoughts))
	}
	if m.snapshot.Thoughts[0].Thought.ID != readyID || m.snapshot.Thoughts[1].Thought.ID != tendedID {
		t.Fatalf("ready ordering = [%d %d], want ready then tended", m.snapshot.Thoughts[0].Thought.ID, m.snapshot.Thoughts[1].Thought.ID)
	}

	m = press(m, tea.KeyMsg{Type: tea.KeyRight})
	if m.filter != FilterResting || len(m.snapshot.Thoughts) != 1 || m.snapshot.Thoughts[0].Thought.ID != restingID {
		t.Fatalf("resting filter mismatch: filter=%v thoughts=%+v", m.filter, m.snapshot.Thoughts)
	}
	m = press(m, tea.KeyMsg{Type: tea.KeyRight})
	if m.filter != FilterMemory || len(m.snapshot.Thoughts) != 1 || m.snapshot.Thoughts[0].Thought.ID != memoryID {
		t.Fatalf("memory filter mismatch: filter=%v thoughts=%+v", m.filter, m.snapshot.Thoughts)
	}

	m = press(m, runeKey('/'))
	m.search.SetValue("gamma")
	m = press(m, tea.KeyMsg{Type: tea.KeyEnter})
	if !strings.Contains(m.status, "No matching thought") {
		t.Fatalf("search status = %q, want gentle empty", m.status)
	}
}

func TestLayoutFitsWideMediumCompactAndSmall(t *testing.T) {
	withSettleDuration(t, 0)
	m := newTestModel(t)
	for i := 0; i < 5; i++ {
		if _, err := m.service.Capture(fmt.Sprintf("thought %d", i)); err != nil {
			t.Fatalf("capture: %v", err)
		}
	}
	m.reloadPreserving(0)

	for _, size := range []struct{ width, height int }{{120, 36}, {100, 30}, {76, 24}, {54, 16}} {
		view := assertViewFits(t, m, size.width, size.height)
		if strings.Contains(view, "Garden") || strings.Contains(view, "terminal garden") {
			t.Fatalf("view should not include Garden copy: %q", view)
		}
	}
	view := assertViewFits(t, m, 50, 14)
	if !strings.Contains(view, "more room") {
		t.Fatalf("small view missing minimum-size message: %q", view)
	}
}

func TestQueueAndDetailScrolling(t *testing.T) {
	withSettleDuration(t, 0)
	m := newTestModel(t)
	for i := 0; i < 24; i++ {
		if _, err := m.service.Capture(fmt.Sprintf("scroll thought %02d", i)); err != nil {
			t.Fatalf("capture: %v", err)
		}
	}
	m.reloadPreserving(0)
	m = sized(m, 90, 24)
	for i := 0; i < 14; i++ {
		m = press(m, tea.KeyMsg{Type: tea.KeyDown})
	}
	if m.queueOffset == 0 {
		t.Fatal("queue offset should advance after moving beyond visible rows")
	}

	m = press(m, tea.KeyMsg{Type: tea.KeyEnter})
	m = press(m, tea.KeyMsg{Type: tea.KeyCtrlD})
	if m.detailOffset == 0 {
		t.Fatal("detail offset should advance when detail is focused and Ctrl+D is pressed")
	}
	m = press(m, tea.KeyMsg{Type: tea.KeyCtrlU})
	if m.detailOffset != 0 {
		t.Fatalf("detail offset = %d, want 0 after Ctrl+U", m.detailOffset)
	}
}

func TestBottomRailModes(t *testing.T) {
	withSettleDuration(t, 0)
	m := newTestModel(t)
	if _, err := m.service.Capture("alpha"); err != nil {
		t.Fatalf("capture: %v", err)
	}
	m.reloadPreserving(0)
	m = sized(m, 120, 32)

	browse := m.View()
	for _, want := range []string{"Move gently", "a capture", "x release"} {
		if !strings.Contains(browse, want) {
			t.Fatalf("browse rail missing %q: %q", want, browse)
		}
	}

	m = press(m, runeKey('/'))
	search := m.View()
	if !strings.Contains(search, "Search") || !strings.Contains(search, "Enter apply") {
		t.Fatalf("search rail incomplete: %q", search)
	}
	m = press(m, tea.KeyMsg{Type: tea.KeyEsc})

	m = press(m, runeKey('f'))
	filter := m.View()
	if !strings.Contains(filter, "Ready") || !strings.Contains(filter, "Memory") || !strings.Contains(filter, "h/l choose") {
		t.Fatalf("filter rail incomplete: %q", filter)
	}
	m = press(m, tea.KeyMsg{Type: tea.KeyEsc})

	m = press(m, runeKey('x'))
	release := m.View()
	if !strings.Contains(release, "Release #1 permanently") || !strings.Contains(release, "reindexes local IDs") {
		t.Fatalf("release rail incomplete: %q", release)
	}
}

func TestReleasePermanentConfirmation(t *testing.T) {
	withSettleDuration(t, 0)
	m := newTestModel(t)
	if _, err := m.service.Capture("first"); err != nil {
		t.Fatalf("capture first: %v", err)
	}
	if _, err := m.service.Capture("second"); err != nil {
		t.Fatalf("capture second: %v", err)
	}
	m.reloadPreserving(1)

	m = press(m, runeKey('x'))
	if m.mode != ModeReleaseConfirm {
		t.Fatalf("mode = %v, want release confirm", m.mode)
	}
	m = press(m, runeKey('n'))
	if len(m.snapshot.Thoughts) != 2 || m.mode != ModeBrowse {
		t.Fatalf("release cancel changed state: mode=%v count=%d", m.mode, len(m.snapshot.Thoughts))
	}

	m = press(m, runeKey('x'))
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
