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
	modeFilter
	modeHelp
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

// Run opens Bloom, Peony's terminal garden.
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
	m.search.Placeholder = "search thoughts, states, notes, or ids"
	m.search.CharLimit = 80
	m.search.Width = 40

	m.reloadPreserving(0)
	m.ensureUsableSelection()
	return m
}

// Model holds the state for Bloom.
type Model struct {
	service *app.Service

	mode        mode
	width       int
	height      int
	zoneIndex   int
	selected    int
	filterIndex int
	filter      core.State
	query       string
	status      string
	detailFocus bool

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
		case modeFilter:
			return m.updateFilter(msg)
		case modeHelp:
			return m.updateHelp(msg)
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
		m.detailFocus = false
		m.status = ""
	case "?":
		m.mode = modeHelp
	case "tab":
		if m.compactLayout() && m.hasSelection() {
			m.detailFocus = !m.detailFocus
			if m.detailFocus {
				m.status = "Detail pane focused. Tab returns to the garden."
			} else {
				m.status = "Garden focused."
			}
		} else {
			m.nextZone()
		}
	case "shift+tab":
		m.previousZone()
	case "down", "j":
		if !m.detailFocus {
			m.move(1)
		}
	case "up", "k":
		if !m.detailFocus {
			m.move(-1)
		}
	case "right", "l":
		m.nextZone()
	case "left", "h":
		m.previousZone()
	case "enter":
		if m.hasSelection() {
			m.detailFocus = true
			m.status = "Inspecting thought. Esc returns to the garden."
		}
	case "R":
		m.reloadPreserving(m.selectedID())
		m.status = "Bloom refreshed."
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
		m.mode = modeFilter
		m.filterIndex = m.indexForFilter(m.filter)
		m.status = "Choose a lifecycle filter."
	case "t":
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

func (m Model) updateFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeBrowse
		m.status = "Filter unchanged."
	case "up", "k":
		m.filterIndex--
		if m.filterIndex < 0 {
			m.filterIndex = len(stateFilters) - 1
		}
	case "down", "j":
		m.filterIndex = (m.filterIndex + 1) % len(stateFilters)
	case "enter":
		m.filter = stateFilters[m.filterIndex]
		m.mode = modeBrowse
		m.reloadPreserving(0)
		if m.filter == "" {
			m.status = "Showing the whole garden."
		} else {
			m.status = "Filtering: " + string(m.filter)
		}
	}
	return m, nil
}

func (m Model) updateHelp(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "?", "q", "enter":
		m.mode = modeBrowse
		m.status = ""
	}
	return m, nil
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
		m.status = "Marked tended. Use r, e, x, or A when it feels resolved."
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
	if len(m.snapshot.Zones) == 0 {
		m.zoneIndex = 0
		m.selected = 0
		return
	}
	if id != 0 {
		for zi, zone := range m.snapshot.Zones {
			for ii, item := range zone.Thoughts {
				if item.Thought.ID == id {
					m.zoneIndex = zi
					m.selected = ii
					return
				}
			}
		}
	}
	m.ensureUsableSelection()
}

func (m *Model) ensureUsableSelection() {
	if len(m.snapshot.Zones) == 0 {
		m.zoneIndex = 0
		m.selected = 0
		return
	}
	if m.zoneIndex < 0 {
		m.zoneIndex = 0
	}
	if m.zoneIndex >= len(m.snapshot.Zones) {
		m.zoneIndex = len(m.snapshot.Zones) - 1
	}
	zone := m.snapshot.Zones[m.zoneIndex]
	if len(zone.Thoughts) == 0 {
		m.selected = 0
		return
	}
	if m.selected < 0 {
		m.selected = 0
	}
	if m.selected >= len(zone.Thoughts) {
		m.selected = len(zone.Thoughts) - 1
	}
}

func (m *Model) move(delta int) {
	zone := m.currentZone()
	if len(zone.Thoughts) == 0 {
		m.selected = 0
		return
	}
	m.selected += delta
	if m.selected < 0 {
		m.selected = 0
	}
	if m.selected >= len(zone.Thoughts) {
		m.selected = len(zone.Thoughts) - 1
	}
}

func (m *Model) nextZone() {
	if len(m.snapshot.Zones) == 0 {
		return
	}
	m.zoneIndex = (m.zoneIndex + 1) % len(m.snapshot.Zones)
	m.selected = 0
	m.detailFocus = false
}

func (m *Model) previousZone() {
	if len(m.snapshot.Zones) == 0 {
		return
	}
	m.zoneIndex--
	if m.zoneIndex < 0 {
		m.zoneIndex = len(m.snapshot.Zones) - 1
	}
	m.selected = 0
	m.detailFocus = false
}

func (m Model) currentZone() app.GardenZone {
	if len(m.snapshot.Zones) == 0 || m.zoneIndex < 0 || m.zoneIndex >= len(m.snapshot.Zones) {
		return app.GardenZone{Title: "Garden", Empty: "Nothing is here yet."}
	}
	return m.snapshot.Zones[m.zoneIndex]
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
	case modeFilter:
		return m.shell("Filter", m.filterView())
	case modeHelp:
		return m.shell("Help", m.helpView())
	case modeReleaseConfirm:
		return m.shell("Release", m.releaseView())
	default:
		return m.browseView()
	}
}

func (m Model) browseView() string {
	zones := m.zonesView()
	detail := m.detailView()
	if m.compactLayout() {
		if m.detailFocus {
			return m.shell("Garden", detail+"\n\n"+hintStyle.Render("Tab returns to zones  Esc back"))
		}
		return m.shell("Garden", zones+"\n\n"+hintStyle.Render("Tab switches pane  h/l switches zones"))
	}
	if m.width >= 110 {
		leftWidth := maxInt(44, m.width/2-4)
		rightWidth := maxInt(44, m.width-leftWidth-4)
		zones = lipgloss.NewStyle().Width(leftWidth).Render(zones)
		detail = lipgloss.NewStyle().Width(rightWidth).Render(detail)
		return m.shell("Garden", lipgloss.JoinHorizontal(lipgloss.Top, zones, "  ", detail))
	}
	return m.shell("Garden", zones+"\n\n"+detail)
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
	zone := m.currentZone().Title
	header := titleStyle.Render("Bloom") + " " + subtleStyle.Render(title)
	meta := fmt.Sprintf("ready %d | zone %s | filter %s | search %s", m.snapshot.ReadyCount, strings.ToLower(zone), filter, query)
	footer := hintStyle.Render("j/k move  tab zone/pane  a capture  t tend  r rest  e evolve  x release  A archive  / search  f filter  ? help  R reload  q quit")
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

func (m Model) zonesView() string {
	if len(m.snapshot.Zones) == 0 {
		return panelStyle.Render("Nothing has been planted yet.")
	}
	blocks := make([]string, 0, len(m.snapshot.Zones))
	for zi, zone := range m.snapshot.Zones {
		var b strings.Builder
		title := fmt.Sprintf("%s (%d)", zone.Title, len(zone.Thoughts))
		if zi == m.zoneIndex {
			b.WriteString(activeZoneStyle.Render(title))
		} else {
			b.WriteString(zoneStyle.Render(title))
		}
		b.WriteString("\n")
		if len(zone.Thoughts) == 0 {
			b.WriteString(subtleStyle.Render(zone.Empty))
		} else {
			for ii, item := range zone.Thoughts {
				marker := " "
				if zi == m.zoneIndex && ii == m.selected && !m.detailFocus {
					marker = ">"
				}
				state := string(item.Thought.CurrentState)
				if item.Ready {
					state = "ready"
				}
				line := fmt.Sprintf("%s #%d %-8s %s", marker, item.Thought.ID, state, oneLine(item.Thought.Content, 56))
				if zi == m.zoneIndex && ii == m.selected && !m.detailFocus {
					b.WriteString(selectedStyle.Render(line))
				} else {
					b.WriteString(line)
				}
				if ii < len(zone.Thoughts)-1 {
					b.WriteString("\n")
				}
			}
		}
		blocks = append(blocks, panelStyle.Render(b.String()))
	}
	return strings.Join(blocks, "\n")
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
	} else if t.CurrentState == core.StateTended {
		b.WriteString("Needs a resolution: rest, evolve, archive, or release\n")
	} else {
		fmt.Fprintf(&b, "Eligible %s\n", relativeTime(t.EligibilityAt, now))
	}
	b.WriteString("\n")
	b.WriteString(t.Content)
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "Created  %s\n", t.CreatedAt.UTC().Format("2006-01-02 15:04Z"))
	fmt.Fprintf(&b, "Updated  %s\n", t.UpdatedAt.UTC().Format("2006-01-02 15:04Z"))
	if t.LastTendedAt != nil {
		fmt.Fprintf(&b, "Tended   %s\n", t.LastTendedAt.UTC().Format("2006-01-02 15:04Z"))
	}
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

func (m Model) filterView() string {
	labels := []string{"All", "Captured", "Resting", "Tended", "Evolved", "Released", "Archived"}
	var b strings.Builder
	for i, label := range labels {
		marker := " "
		if i == m.filterIndex {
			marker = ">"
		}
		line := marker + " " + label
		if i == m.filterIndex {
			b.WriteString(selectedStyle.Render(line))
		} else {
			b.WriteString(line)
		}
		if i < len(labels)-1 {
			b.WriteString("\n")
		}
	}
	return panelStyle.Render(b.String()) + "\n" + hintStyle.Render("j/k choose  Enter apply  Esc cancel")
}

func (m Model) helpView() string {
	return panelStyle.Render(strings.Join([]string{
		"Bloom is Peony's terminal garden.",
		"",
		"Move: j/k, arrows, h/l, Tab",
		"Capture: a",
		"Tend: t, then Ctrl+S",
		"Resolve: r rest, e evolve, A archive, x release",
		"Find: / search, f filter",
		"Refresh: R",
		"Leave: q from garden, Esc from overlays",
	}, "\n"))
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
	zone := m.currentZone()
	if m.selected < 0 || m.selected >= len(zone.Thoughts) {
		return app.GardenThought{}, false
	}
	return zone.Thoughts[m.selected], true
}

func (m Model) hasSelection() bool {
	_, ok := m.selectedItem()
	return ok
}

func (m Model) selectedID() int64 {
	item, ok := m.selectedItem()
	if !ok {
		return 0
	}
	return item.Thought.ID
}

func (m Model) compactLayout() bool {
	return m.width > 0 && m.width < 82
}

func (m Model) indexForFilter(filter core.State) int {
	for i, state := range stateFilters {
		if state == filter {
			return i
		}
	}
	return 0
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
	if limit <= 3 {
		return s[:limit]
	}
	return s[:limit-3] + "..."
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
	titleStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("170"))
	subtleStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	hintStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("246"))
	statusStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("114"))
	selectedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("95"))
	labelStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("109"))
	zoneStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("109"))
	activeZoneStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("95")).Padding(0, 1)
	panelStyle      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("238")).Padding(1, 2)
	smallStyle      = lipgloss.NewStyle().Padding(1, 2)
)

var _ tea.Model = Model{}
