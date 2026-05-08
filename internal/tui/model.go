package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/divijg19/peony/internal/app"
	"github.com/divijg19/peony/internal/core"
)

type mode int

const (
	modeBrowse mode = iota
	modeAdd
	modeSearch
	modeTend
	modeReleaseConfirm
)

var stateFilters = []core.State{
	"",
	core.StateCaptured,
	core.StateResting,
	core.StateTended,
	core.StateEvolved,
	core.StateReleased,
	core.StateArchived,
}

// Run opens Peony's terminal garden.
func Run() int {
	service, closeFn, err := app.OpenDefault()
	if err != nil {
		fmt.Printf("tui: %v\n", err)
		return 1
	}
	defer closeFn()

	program := tea.NewProgram(NewModel(service), tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		fmt.Printf("tui: %v\n", err)
		return 1
	}
	return 0
}

// NewModel creates the Bubble Tea model. It is exported for tests and future embedding.
func NewModel(service *app.Service) Model {
	m := Model{
		service: service,
		mode:    modeBrowse,
	}
	m.addBox = textarea.New()
	m.addBox.Placeholder = "What would you like to hold?"
	m.addBox.SetWidth(60)
	m.addBox.SetHeight(6)

	m.tendContent = textarea.New()
	m.tendContent.SetWidth(60)
	m.tendContent.SetHeight(8)

	m.tendNote = textarea.New()
	m.tendNote.Placeholder = "Optional note"
	m.tendNote.SetWidth(60)
	m.tendNote.SetHeight(4)

	m.search = textinput.New()
	m.search.Placeholder = "search thoughts"
	m.search.CharLimit = 80
	m.search.Width = 40

	m.reloadPreserving(0)
	return m
}

// Model holds the state for the Peony TUI.
type Model struct {
	service *app.Service

	mode     mode
	width    int
	height   int
	selected int
	filter   core.State
	query    string
	status   string

	snapshot app.GardenSnapshot

	addBox      textarea.Model
	tendContent textarea.Model
	tendNote    textarea.Model
	tendFocus   int
	search      textinput.Model
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeInputs()
		return m, nil
	case tea.KeyMsg:
		switch m.mode {
		case modeAdd:
			return m.updateAdd(msg)
		case modeSearch:
			return m.updateSearch(msg)
		case modeTend:
			return m.updateTend(msg)
		case modeReleaseConfirm:
			return m.updateReleaseConfirm(msg)
		default:
			return m.updateBrowse(msg)
		}
	}
	return m, nil
}

func (m Model) updateBrowse(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.status = ""
	case "down", "j":
		m.move(1)
	case "up", "k":
		m.move(-1)
	case "R":
		m.reloadPreserving(m.selectedID())
		m.status = "Garden refreshed."
	case "a":
		m.mode = modeAdd
		m.addBox.Reset()
		m.addBox.Focus()
		m.status = "Capture a thought. Ctrl+S saves, Esc cancels."
	case "/":
		m.mode = modeSearch
		m.search.SetValue(m.query)
		m.search.Focus()
		m.status = "Search the garden. Enter applies."
	case "f":
		m.cycleFilter()
	case "enter", "t":
		m.startTend()
	case "r":
		m.restSelected()
	case "e":
		m.evolveSelected()
	case "x":
		if m.hasSelection() {
			m.mode = modeReleaseConfirm
			m.status = "Release permanently? y confirms, n cancels."
		}
	case "A":
		m.archiveSelected()
	}
	return m, nil
}

func (m Model) updateAdd(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.addBox.Blur()
		m.mode = modeBrowse
		m.status = "Capture cancelled."
		return m, nil
	case "ctrl+s":
		id, err := m.service.Capture(m.addBox.Value())
		if err != nil {
			m.status = err.Error()
			return m, nil
		}
		m.mode = modeBrowse
		m.addBox.Blur()
		m.reloadPreserving(id)
		m.status = fmt.Sprintf("Saved as #%d.", id)
		return m, nil
	}

	var cmd tea.Cmd
	m.addBox, cmd = m.addBox.Update(msg)
	return m, cmd
}

func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeBrowse
		m.search.Blur()
		m.status = "Search cancelled."
		return m, nil
	case "enter":
		m.query = strings.TrimSpace(m.search.Value())
		m.mode = modeBrowse
		m.search.Blur()
		m.reloadPreserving(0)
		if m.query == "" {
			m.status = "Search cleared."
		} else {
			m.status = "Search applied."
		}
		return m, nil
	case "ctrl+u":
		m.search.SetValue("")
	}

	var cmd tea.Cmd
	m.search, cmd = m.search.Update(msg)
	return m, cmd
}

func (m Model) updateTend(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeBrowse
		m.tendContent.Blur()
		m.tendNote.Blur()
		m.status = "Tend cancelled."
		return m, nil
	case "tab":
		m.tendFocus = (m.tendFocus + 1) % 2
		m.focusTendInput()
		return m, nil
	case "ctrl+s":
		item, ok := m.selectedItem()
		if !ok {
			m.mode = modeBrowse
			return m, nil
		}
		noteValue := strings.TrimSpace(m.tendNote.Value())
		var note *string
		if noteValue != "" {
			note = &noteValue
		}
		if err := m.service.Tend(item.Thought.ID, m.tendContent.Value(), note); err != nil {
			m.status = err.Error()
			return m, nil
		}
		m.mode = modeBrowse
		m.reloadPreserving(item.Thought.ID)
		m.status = "Marked tended. Use r, e, x, or A to resolve."
		return m, nil
	}

	var cmd tea.Cmd
	if m.tendFocus == 0 {
		m.tendContent, cmd = m.tendContent.Update(msg)
	} else {
		m.tendNote, cmd = m.tendNote.Update(msg)
	}
	return m, cmd
}

func (m Model) updateReleaseConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		id := m.selectedID()
		oldIndex := m.selected
		if id == 0 {
			m.mode = modeBrowse
			return m, nil
		}
		if err := m.service.ReleasePermanent(id); err != nil {
			m.status = err.Error()
			m.mode = modeBrowse
			return m, nil
		}
		m.mode = modeBrowse
		m.reloadPreserving(0)
		if oldIndex >= len(m.snapshot.Thoughts) {
			oldIndex = len(m.snapshot.Thoughts) - 1
		}
		if oldIndex < 0 {
			oldIndex = 0
		}
		m.selected = oldIndex
		m.status = fmt.Sprintf("Released #%d permanently.", id)
	case "n", "N", "esc":
		m.mode = modeBrowse
		m.status = "Release cancelled."
	}
	return m, nil
}

func (m *Model) startTend() {
	item, ok := m.selectedItem()
	if !ok {
		m.status = "No thought selected."
		return
	}
	if !item.Ready {
		m.status = "This thought is still resting."
		return
	}
	m.mode = modeTend
	m.tendFocus = 0
	m.tendContent.SetValue(item.Thought.Content)
	m.tendNote.Reset()
	m.focusTendInput()
	m.status = "Tend gently. Tab moves to note, Ctrl+S saves."
}

func (m *Model) restSelected() {
	item, ok := m.selectedItem()
	if !ok {
		return
	}
	if item.Thought.CurrentState != core.StateTended {
		m.status = "Only a tended thought can be rested."
		return
	}
	if err := m.service.Rest(item.Thought.ID, nil); err != nil {
		m.status = err.Error()
		return
	}
	m.reloadPreserving(item.Thought.ID)
	m.status = "Returned to rest."
}

func (m *Model) evolveSelected() {
	item, ok := m.selectedItem()
	if !ok {
		return
	}
	if err := m.service.Evolve(item.Thought.ID); err != nil {
		m.status = err.Error()
		return
	}
	m.reloadPreserving(item.Thought.ID)
	m.status = "Marked evolved."
}

func (m *Model) archiveSelected() {
	item, ok := m.selectedItem()
	if !ok {
		return
	}
	if err := m.service.Archive(item.Thought.ID); err != nil {
		m.status = err.Error()
		return
	}
	m.reloadPreserving(item.Thought.ID)
	m.status = "Archived."
}

func (m *Model) cycleFilter() {
	idx := 0
	for i, state := range stateFilters {
		if state == m.filter {
			idx = i
			break
		}
	}
	idx = (idx + 1) % len(stateFilters)
	m.filter = stateFilters[idx]
	m.reloadPreserving(0)
	if m.filter == "" {
		m.status = "Showing all thoughts."
	} else {
		m.status = "Filtering: " + string(m.filter)
	}
}

func (m *Model) reloadPreserving(id int64) {
	if id == 0 {
		id = m.selectedID()
	}
	snapshot, err := m.service.Snapshot(m.filter, m.query)
	if err != nil {
		m.status = err.Error()
		return
	}
	m.snapshot = snapshot
	if len(m.snapshot.Thoughts) == 0 {
		m.selected = 0
		return
	}
	if id != 0 {
		for i, item := range m.snapshot.Thoughts {
			if item.Thought.ID == id {
				m.selected = i
				return
			}
		}
	}
	if m.selected >= len(m.snapshot.Thoughts) {
		m.selected = len(m.snapshot.Thoughts) - 1
	}
	if m.selected < 0 {
		m.selected = 0
	}
}

func (m *Model) move(delta int) {
	if len(m.snapshot.Thoughts) == 0 {
		m.selected = 0
		return
	}
	m.selected += delta
	if m.selected < 0 {
		m.selected = 0
	}
	if m.selected >= len(m.snapshot.Thoughts) {
		m.selected = len(m.snapshot.Thoughts) - 1
	}
}

func (m Model) View() string {
	if m.width > 0 && (m.width < 50 || m.height < 12) {
		return smallStyle.Render("Peony needs a little more room to bloom.")
	}

	switch m.mode {
	case modeAdd:
		return m.shell("Capture", m.addBox.View()+"\n\n"+hintStyle.Render("Ctrl+S save  Esc cancel"))
	case modeSearch:
		return m.shell("Search", m.search.View()+"\n\n"+hintStyle.Render("Enter apply  Ctrl+U clear  Esc cancel"))
	case modeTend:
		return m.shell("Tend", m.tendView())
	case modeReleaseConfirm:
		return m.shell("Release", m.releaseView())
	default:
		return m.browseView()
	}
}

func (m Model) browseView() string {
	list := m.listView()
	detail := m.detailView()
	if m.width >= 100 {
		leftWidth := maxInt(38, m.width/2-2)
		rightWidth := maxInt(40, m.width-leftWidth-4)
		list = lipgloss.NewStyle().Width(leftWidth).Render(list)
		detail = lipgloss.NewStyle().Width(rightWidth).Render(detail)
		return m.shell("Garden Inbox", lipgloss.JoinHorizontal(lipgloss.Top, list, "  ", detail))
	}
	return m.shell("Garden Inbox", list+"\n\n"+detail)
}

func (m Model) shell(title string, body string) string {
	filter := "all"
	if m.filter != "" {
		filter = string(m.filter)
	}
	query := m.query
	if query == "" {
		query = "none"
	}
	header := titleStyle.Render("Peony") + " " + subtleStyle.Render(title)
	meta := fmt.Sprintf("ready %d | filter %s | search %s", m.snapshot.ReadyCount, filter, query)
	footer := hintStyle.Render("j/k move  a add  t tend  r rest  e evolve  x release  A archive  / search  f filter  R reload  q quit")
	if strings.TrimSpace(m.status) != "" {
		footer += "\n" + statusStyle.Render(m.status)
	}
	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		subtleStyle.Render(meta),
		"",
		body,
		"",
		footer,
	)
}

func (m Model) listView() string {
	if len(m.snapshot.Thoughts) == 0 {
		return panelStyle.Render("No thoughts here yet.")
	}

	var b strings.Builder
	for i, item := range m.snapshot.Thoughts {
		marker := " "
		if i == m.selected {
			marker = ">"
		}
		state := string(item.Thought.CurrentState)
		if item.Ready {
			state = "ready"
		}
		line := fmt.Sprintf("%s #%d %-8s %s", marker, item.Thought.ID, state, oneLine(item.Thought.Content, 58))
		if i == m.selected {
			b.WriteString(selectedStyle.Render(line))
		} else {
			b.WriteString(line)
		}
		if i < len(m.snapshot.Thoughts)-1 {
			b.WriteString("\n")
		}
	}
	return panelStyle.Render(b.String())
}

func (m Model) detailView() string {
	item, ok := m.selectedItem()
	if !ok {
		return panelStyle.Render("Select a thought to see its shape.")
	}

	now := time.Now().UTC()
	t := item.Thought
	var b strings.Builder
	fmt.Fprintf(&b, "#%d  %s  tends:%d\n", t.ID, t.CurrentState, t.TendCounter)
	if item.Ready {
		b.WriteString("Ready to tend\n")
	} else {
		fmt.Fprintf(&b, "Eligible %s\n", relativeTime(t.EligibilityAt, now))
	}
	b.WriteString("\n")
	b.WriteString(t.Content)
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "Created  %s\n", t.CreatedAt.UTC().Format("2006-01-02 15:04Z"))
	fmt.Fprintf(&b, "Updated  %s\n", t.UpdatedAt.UTC().Format("2006-01-02 15:04Z"))
	if len(item.Events) > 0 {
		b.WriteString("\nEvents\n")
		for _, event := range item.Events {
			fmt.Fprintf(&b, "- %s %s", event.At.UTC().Format("2006-01-02"), event.Kind)
			if event.NextState != nil {
				fmt.Fprintf(&b, " -> %s", *event.NextState)
			}
			if event.Note != nil && strings.TrimSpace(*event.Note) != "" {
				fmt.Fprintf(&b, " (%s)", oneLine(*event.Note, 42))
			}
			b.WriteString("\n")
		}
	}
	return panelStyle.Render(b.String())
}

func (m Model) tendView() string {
	return strings.Join([]string{
		labelStyle.Render("Content"),
		m.tendContent.View(),
		labelStyle.Render("Note"),
		m.tendNote.View(),
		hintStyle.Render("Tab switch fields  Ctrl+S mark tended  Esc cancel"),
	}, "\n")
}

func (m Model) releaseView() string {
	item, ok := m.selectedItem()
	if !ok {
		return "No thought selected."
	}
	return panelStyle.Render(fmt.Sprintf(
		"Release #%d permanently?\n\n%s\n\nThis deletes the thought and its event history, then reindexes local IDs.\n\ny confirm  n cancel",
		item.Thought.ID,
		oneLine(item.Thought.Content, 80),
	))
}

func (m Model) selectedItem() (app.GardenThought, bool) {
	if !m.hasSelection() {
		return app.GardenThought{}, false
	}
	return m.snapshot.Thoughts[m.selected], true
}

func (m Model) hasSelection() bool {
	return m.selected >= 0 && m.selected < len(m.snapshot.Thoughts)
}

func (m Model) selectedID() int64 {
	item, ok := m.selectedItem()
	if !ok {
		return 0
	}
	return item.Thought.ID
}

func (m *Model) focusTendInput() {
	if m.tendFocus == 0 {
		m.tendContent.Focus()
		m.tendNote.Blur()
		return
	}
	m.tendContent.Blur()
	m.tendNote.Focus()
}

func (m *Model) resizeInputs() {
	width := m.width - 8
	if width < 30 {
		width = 30
	}
	if width > 86 {
		width = 86
	}
	m.addBox.SetWidth(width)
	m.tendContent.SetWidth(width)
	m.tendNote.SetWidth(width)
	m.search.Width = minInt(width, 60)
}

func oneLine(s string, limit int) string {
	s = strings.Join(strings.Fields(s), " ")
	if limit <= 0 || len(s) <= limit {
		return s
	}
	if limit <= 1 {
		return s[:limit]
	}
	return s[:limit-1] + "..."
}

func relativeTime(t time.Time, now time.Time) string {
	if t.IsZero() {
		return "later"
	}
	d := t.Sub(now)
	prefix := "in "
	if d < 0 {
		d = -d
		prefix = ""
	}
	var value string
	switch {
	case d < time.Minute:
		value = "now"
	case d < time.Hour:
		value = fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		value = fmt.Sprintf("%dh", int(d.Hours()))
	default:
		value = fmt.Sprintf("%dd", int(d.Hours()/24))
	}
	if prefix == "" && value != "now" {
		return value + " ago"
	}
	return prefix + value
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("170"))
	subtleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	hintStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("246"))
	statusStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("114"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("95"))
	labelStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("109"))
	panelStyle    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("238")).Padding(1, 2)
	smallStyle    = lipgloss.NewStyle().Padding(1, 2)
)

var _ tea.Model = Model{}
