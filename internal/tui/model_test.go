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

func runCommand(m Model, value string) Model {
	m = press(m, runeKey(':'))
	m.command.SetValue(value)
	return press(m, tea.KeyMsg{Type: tea.KeyEnter})
}

func outputText(m Model) string {
	return strings.Join(m.output.Lines, "\n")
}

func assertViewFits(t *testing.T, m Model, width, height int) string {
	t.Helper()
	m = sized(m, width, height)
	view := m.View()
	if got := lipgloss.Width(view); got > width {
		t.Fatalf("view width = %d, want <= %d\n%s", got, width, view)
	}
	if got := lipgloss.Width(view); got != width {
		t.Fatalf("view width = %d, want exactly %d to avoid right-side slack\n%s", got, width, view)
	}
	if got := lipgloss.Height(view); got > height {
		t.Fatalf("view height = %d, want <= %d\n%s", got, height, view)
	}
	return view
}

func assertEveryLineWidth(t *testing.T, view string, width int) {
	t.Helper()
	for i, line := range strings.Split(view, "\n") {
		if got := lipgloss.Width(line); got != width {
			t.Fatalf("line %d width = %d, want %d: %q\n%s", i+1, got, width, line, view)
		}
	}
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

	evolvedID, err := m.service.Capture("evolved delta")
	if err != nil {
		t.Fatalf("capture evolved: %v", err)
	}
	if err := m.service.Evolve(evolvedID); err != nil {
		t.Fatalf("evolve: %v", err)
	}

	archivedID, err := m.service.Capture("archived epsilon")
	if err != nil {
		t.Fatalf("capture archived: %v", err)
	}
	if err := m.service.Archive(archivedID); err != nil {
		t.Fatalf("archive: %v", err)
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
	if m.filter != FilterAll || len(m.snapshot.Thoughts) != 4 {
		t.Fatalf("all filter mismatch: filter=%v thoughts=%+v", m.filter, m.snapshot.Thoughts)
	}
	for _, item := range m.snapshot.Thoughts {
		if item.Thought.ID == archivedID {
			t.Fatalf("archived thought leaked into Bloom all filter: %+v", m.snapshot.Thoughts)
		}
	}
	if strings.Contains(m.View(), "archived epsilon") {
		t.Fatalf("archived thought rendered in Bloom all view: %q", m.View())
	}
	foundEvolved := false
	for _, item := range m.snapshot.Thoughts {
		if item.Thought.ID == evolvedID {
			foundEvolved = true
		}
	}
	if !foundEvolved {
		t.Fatalf("evolved thought missing from Bloom all filter: %+v", m.snapshot.Thoughts)
	}

	m = press(m, runeKey('/'))
	m.search.SetValue("archived")
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
		layout := sized(m, size.width, size.height).layout()
		if layout.contextWidth != 0 {
			t.Fatalf("context output should not appear at %dx%d, got width %d", size.width, size.height, layout.contextWidth)
		}
	}
	wide := sized(m, 140, 36)
	wideLayout := wide.layout()
	if wideLayout.contextWidth != 0 {
		t.Fatalf("idle wide layout should stay two-column, got context width %d", wideLayout.contextWidth)
	}
	if strings.Contains(assertViewFits(t, m, 140, 36), "Output") {
		t.Fatal("idle wide view should not render contextual output")
	}
	view := assertViewFits(t, m, 50, 14)
	if !strings.Contains(view, "more room") {
		t.Fatalf("small view missing minimum-size message: %q", view)
	}
}

func TestPromptBarAlwaysPresentAndBounded(t *testing.T) {
	withSettleDuration(t, 0)
	m := newTestModel(t)
	if _, err := m.service.Capture("alpha"); err != nil {
		t.Fatalf("capture: %v", err)
	}
	m.reloadPreserving(0)
	m = sized(m, 120, 32)
	if got := m.layout().promptHeight; got != 5 {
		t.Fatalf("browse prompt height = %d, want 5", got)
	}
	if !strings.Contains(m.View(), "Bloom") || !strings.Contains(m.View(), "/ search") || !strings.Contains(m.View(), ": command") {
		t.Fatalf("browse prompt bar missing launcher text: %q", m.View())
	}

	m = press(m, runeKey('/'))
	layout := m.layout()
	if layout.promptHeight < 3 {
		t.Fatalf("search prompt height = %d, want at least 3", layout.promptHeight)
	}
	if layout.promptHeight > layout.height/5 {
		t.Fatalf("prompt height = %d, want <= 20%% of %d", layout.promptHeight, layout.height)
	}

	for _, key := range []tea.KeyMsg{runeKey(':'), runeKey('f'), runeKey('x')} {
		m = sized(newTestModel(t), 120, 32)
		if _, err := m.service.Capture("alpha"); err != nil {
			t.Fatalf("capture: %v", err)
		}
		m.reloadPreserving(0)
		m = press(m, key)
		if got := m.layout().promptHeight; got < 3 || got > m.layout().height/5 {
			t.Fatalf("prompt height for key %q = %d", key.String(), got)
		}
	}
}

func TestCommandBarRunsReadableCommands(t *testing.T) {
	withSettleDuration(t, 0)
	m := newTestModel(t)
	id, err := m.service.Capture("visible alpha")
	if err != nil {
		t.Fatalf("capture visible: %v", err)
	}
	archivedID, err := m.service.Capture("archived omega")
	if err != nil {
		t.Fatalf("capture archived: %v", err)
	}
	if err := m.service.Archive(archivedID); err != nil {
		t.Fatalf("archive: %v", err)
	}
	m.reloadPreserving(id)

	for _, tc := range []struct {
		command string
		want    string
	}{
		{"help", "Peony commands"},
		{"help view", "peony view"},
		{"view", "Visible thoughts"},
		{fmt.Sprintf("view %d", id), "CONTENT"},
		{"view archived", "archived omega"},
		{"tend", "Ready to tend"},
		{"version", "Peony v0.4"},
		{"config", "Current configuration"},
		{"later", "Unknown command"},
		{"tui", "Bloom is already open"},
	} {
		m = runCommand(m, tc.command)
		if !strings.Contains(outputText(m), tc.want) {
			t.Fatalf("command %q output = %+v, want %q", tc.command, m.output.Lines, tc.want)
		}
	}
}

func TestCommandBarRunsMutatingAndTUIScreenCommands(t *testing.T) {
	withSettleDuration(t, 0)
	m := newTestModel(t)

	m = runCommand(m, "add")
	if m.mode != ModeCapture {
		t.Fatalf("bare add mode = %v, want capture", m.mode)
	}
	m = press(m, tea.KeyMsg{Type: tea.KeyEsc})

	m = runCommand(m, "add command-born thought")
	if len(m.snapshot.Thoughts) != 1 || m.snapshot.Thoughts[0].Thought.Content != "command-born thought" {
		t.Fatalf("add command snapshot = %+v", m.snapshot.Thoughts)
	}
	id := m.snapshot.Thoughts[0].Thought.ID

	m = runCommand(m, `add "quoted tender thought"`)
	foundQuoted := false
	for _, item := range m.snapshot.Thoughts {
		if item.Thought.Content == "quoted tender thought" {
			foundQuoted = true
		}
	}
	if len(m.snapshot.Thoughts) != 2 || !foundQuoted {
		t.Fatalf("quoted add snapshot = %+v", m.snapshot.Thoughts)
	}

	m = runCommand(m, fmt.Sprintf("tend %d", id))
	if m.mode != ModeTend || m.tendID != id {
		t.Fatalf("tend command mode=%v tendID=%d want mode tend id %d", m.mode, m.tendID, id)
	}
	m = press(m, tea.KeyMsg{Type: tea.KeyEsc})

	m = runCommand(m, fmt.Sprintf("evolve %d", id))
	if !strings.Contains(outputText(m), fmt.Sprintf("Evolved #%d", id)) {
		t.Fatalf("evolve output = %+v", m.output.Lines)
	}

	secondID, err := m.service.Capture("release candidate")
	if err != nil {
		t.Fatalf("capture release candidate: %v", err)
	}
	m.reloadPreserving(secondID)
	m = runCommand(m, fmt.Sprintf("release %d", secondID))
	if m.mode != ModeReleaseConfirm || m.pendingReleaseID != secondID {
		t.Fatalf("release command mode=%v pending=%d want confirm %d", m.mode, m.pendingReleaseID, secondID)
	}
	if _, err := m.service.Thought(secondID); err != nil {
		t.Fatalf("release command should not delete before confirmation: %v", err)
	}
}

func TestContextOutputOnlyForWideOverflow(t *testing.T) {
	withSettleDuration(t, 0)
	m := newTestModel(t)
	if _, err := m.service.Capture("alpha"); err != nil {
		t.Fatalf("capture: %v", err)
	}
	m.reloadPreserving(0)

	m = sized(m, 140, 36)
	if got := m.layout().contextWidth; got != 0 {
		t.Fatalf("idle context width = %d, want 0", got)
	}

	m = runCommand(m, "version")
	m = sized(m, 140, 36)
	if got := m.layout().contextWidth; got != 0 {
		t.Fatalf("short output context width = %d, want 0", got)
	}
	m = press(m, tea.KeyMsg{Type: tea.KeyCtrlO})
	if m.focus == FocusOutput || m.output.Open {
		t.Fatalf("short output should stay in prompt, focus=%v open=%v", m.focus, m.output.Open)
	}

	longLines := []string{strings.Repeat("long command feedback ", 12)}
	for i := 0; i < 24; i++ {
		longLines = append(longLines, fmt.Sprintf("output line %02d", i))
	}
	m.setOutput("Output", longLines, OutputCommand, "test", true)
	if got := m.layout().contextWidth; got == 0 {
		t.Fatal("wide overflow should reserve contextual output")
	}
	if !strings.Contains(m.View(), "Output") {
		t.Fatalf("wide overflow view should render contextual output: %q", m.View())
	}
	m = press(m, tea.KeyMsg{Type: tea.KeyCtrlO})
	if m.focus != FocusOutput {
		t.Fatalf("ctrl+o focus = %v, want output", m.focus)
	}
	m = press(m, tea.KeyMsg{Type: tea.KeyDown})
	if m.output.ScrollOffset == 0 {
		t.Fatal("output should scroll independently when focused")
	}
	m = press(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.focus == FocusOutput || m.output.Open {
		t.Fatalf("esc should close focused output, focus=%v open=%v", m.focus, m.output.Open)
	}

	m.setOutput("Output", longLines, OutputCommand, "test", true)
	m = sized(m, 100, 30)
	if got := m.layout().contextWidth; got != 0 {
		t.Fatalf("medium overflow context width = %d, want collapsed output", got)
	}
	if !strings.Contains(m.View(), "long command feedback") {
		t.Fatalf("medium overflow should collapse into body output: %q", m.View())
	}
}

func TestChromeRowsFillEveryWindowLine(t *testing.T) {
	withSettleDuration(t, 0)
	m := newTestModel(t)
	if _, err := m.service.Capture("alpha"); err != nil {
		t.Fatalf("capture: %v", err)
	}
	m.reloadPreserving(0)

	for _, size := range []struct{ width, height int }{{120, 32}, {100, 30}, {76, 24}, {140, 36}} {
		view := assertViewFits(t, m, size.width, size.height)
		assertEveryLineWidth(t, view, size.width)
	}

	longLines := []string{strings.Repeat("structured output ", 12)}
	for i := 0; i < 12; i++ {
		longLines = append(longLines, fmt.Sprintf("line %02d", i))
	}
	m.setOutput("Output", longLines, OutputCommand, "test", true)
	for _, size := range []struct{ width, height int }{{140, 36}, {100, 30}, {76, 24}} {
		view := assertViewFits(t, m, size.width, size.height)
		assertEveryLineWidth(t, view, size.width)
	}
}

func TestPromptHistoryRecall(t *testing.T) {
	withSettleDuration(t, 0)
	m := newTestModel(t)
	if _, err := m.service.Capture("alpha"); err != nil {
		t.Fatalf("capture: %v", err)
	}
	m.reloadPreserving(0)

	m = press(m, runeKey('/'))
	m.search.SetValue("alpha")
	m = press(m, tea.KeyMsg{Type: tea.KeyEnter})
	m = press(m, runeKey('/'))
	m = press(m, tea.KeyMsg{Type: tea.KeyUp})
	if got := m.search.Value(); got != "alpha" {
		t.Fatalf("search history recall = %q, want alpha", got)
	}
	m = press(m, tea.KeyMsg{Type: tea.KeyEsc})

	m = runCommand(m, "version")
	m = press(m, runeKey(':'))
	m = press(m, tea.KeyMsg{Type: tea.KeyUp})
	if got := m.command.Value(); got != "version" {
		t.Fatalf("command history recall = %q, want version", got)
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

func TestPromptBarModes(t *testing.T) {
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
			t.Fatalf("browse chrome missing %q: %q", want, browse)
		}
	}
	lines := strings.Split(browse, "\n")
	if !strings.Contains(lines[len(lines)-1], "q quit") {
		t.Fatalf("footer keybinding row should sit on the bottom edge: %q", lines[len(lines)-1])
	}
	lines = strings.Split(browse, "\n")
	if strings.Contains(lines[1], "Move gently") || strings.Contains(lines[1], "Showing Ready") {
		t.Fatalf("top action/status bar should be gone: %q", lines[1])
	}

	m = press(m, runeKey('/'))
	search := m.View()
	if !strings.Contains(search, "Search") || !strings.Contains(search, "enter apply") {
		t.Fatalf("search prompt incomplete: %q", search)
	}
	m = press(m, tea.KeyMsg{Type: tea.KeyEsc})

	m = press(m, runeKey(':'))
	if m.mode != ModeCommand {
		t.Fatalf("mode = %v, want command", m.mode)
	}
	command := m.View()
	if !strings.Contains(command, "Command") || !strings.Contains(command, "enter run") {
		t.Fatalf("command prompt incomplete: %q", command)
	}
	m.command.SetValue("help")
	m = press(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != ModeBrowse || !strings.Contains(m.status, "Help opened") {
		t.Fatalf("command validation result: mode=%v status=%q", m.mode, m.status)
	}

	m = press(m, runeKey('f'))
	filter := m.View()
	if !strings.Contains(filter, "Ready") || !strings.Contains(filter, "Resting") || !strings.Contains(filter, "All") || !strings.Contains(filter, "h/l choose") {
		t.Fatalf("filter prompt incomplete: %q", filter)
	}
	if strings.Contains(filter, "Memory") {
		t.Fatalf("filter view should not expose Memory: %q", filter)
	}
	m = press(m, tea.KeyMsg{Type: tea.KeyEsc})

	m = press(m, runeKey('x'))
	release := m.View()
	if !strings.Contains(release, "Release #1 permanently") || !strings.Contains(release, "reindexes local IDs") {
		t.Fatalf("release prompt incomplete: %q", release)
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
