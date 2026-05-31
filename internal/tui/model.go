package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/divijg19/peony/internal/app"
	"github.com/divijg19/peony/internal/core"
)

type Mode int

const (
	ModeBrowse Mode = iota
	ModeCapture
	ModeTend
	ModeSearch
	ModeFilter
	ModeHelp
	ModeReleaseConfirm
)

type PaneFocus int

const (
	FocusQueue PaneFocus = iota
	FocusDetail
	FocusPrompt
)

type FilterKind int

const (
	FilterReady FilterKind = iota
	FilterResting
	FilterAll
)

var filterKinds = []FilterKind{FilterReady, FilterResting, FilterAll}

// Run opens Bloom, Peony's full-screen terminal space.
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
		mode:    ModeBrowse,
		focus:   FocusQueue,
		filter:  FilterReady,
	}
	m.addBox = textarea.New()
	m.addBox.Placeholder = "What would you like to hold?"
	m.addBox.SetWidth(60)
	m.addBox.SetHeight(8)

	m.tendContent = textarea.New()
	m.tendContent.SetWidth(60)
	m.tendContent.SetHeight(8)

	m.tendNote = textarea.New()
	m.tendNote.Placeholder = "Optional note"
	m.tendNote.SetWidth(60)
	m.tendNote.SetHeight(4)

	m.search = textinput.New()
	m.search.Placeholder = "search thoughts, states, notes, or ids"
	m.search.CharLimit = 120
	m.search.Width = 44

	m.reloadPreserving(0)
	m.ensureUsableSelection()
	return m
}

// Model holds the state for Bloom.
type Model struct {
	service *app.Service

	mode   Mode
	width  int
	height int
	focus  PaneFocus

	filter      FilterKind
	filterIndex int
	query       string
	status      string

	selected     int
	queueOffset  int
	detailOffset int

	snapshot app.BloomSnapshot

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
		m.ensureQueueVisible()
		return m, nil
	case tea.KeyMsg:
		switch m.mode {
		case ModeCapture:
			return m.updateCapture(msg)
		case ModeSearch:
			return m.updateSearch(msg)
		case ModeTend:
			return m.updateTend(msg)
		case ModeFilter:
			return m.updateFilter(msg)
		case ModeHelp:
			return m.updateHelp(msg)
		case ModeReleaseConfirm:
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
		m.focus = FocusQueue
		m.status = ""
	case "?":
		m.mode = ModeHelp
		m.status = ""
	case "tab":
		if m.hasSelection() {
			m.toggleFocus()
		}
	case "enter":
		if m.hasSelection() {
			m.focus = FocusDetail
			m.status = "Detail focused. Tab returns to the queue."
		}
	case "down", "j":
		m.moveSelection(1)
	case "up", "k":
		m.moveSelection(-1)
	case "home":
		m.selectIndex(0)
	case "end":
		m.selectIndex(len(m.snapshot.Thoughts) - 1)
	case "ctrl+d":
		if m.focus == FocusDetail {
			m.scrollDetail(6)
		} else {
			m.moveSelection(5)
		}
	case "ctrl+u":
		if m.focus == FocusDetail {
			m.scrollDetail(-6)
		} else {
			m.moveSelection(-5)
		}
	case "right", "l":
		m.applyFilterIndex((m.filter.index() + 1) % len(filterKinds))
	case "left", "h":
		idx := m.filter.index() - 1
		if idx < 0 {
			idx = len(filterKinds) - 1
		}
		m.applyFilterIndex(idx)
	case "R":
		m.reloadPreserving(m.selectedID())
		m.status = "Bloom refreshed."
	case "a":
		m.mode = ModeCapture
		m.focus = FocusPrompt
		m.addBox.Reset()
		m.addBox.Focus()
		m.status = ""
	case "/":
		m.mode = ModeSearch
		m.focus = FocusPrompt
		m.search.SetValue(m.query)
		m.search.Focus()
		m.status = ""
	case "f":
		m.mode = ModeFilter
		m.focus = FocusPrompt
		m.filterIndex = m.filter.index()
		m.status = ""
	case "t":
		m.startTend()
	case "r":
		m.restSelected()
	case "e":
		m.evolveSelected()
	case "x":
		if m.hasSelection() {
			m.mode = ModeReleaseConfirm
			m.focus = FocusPrompt
			m.status = ""
		}
	case "A":
		m.archiveSelected()
	}
	return m, nil
}

func (m Model) updateCapture(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.addBox.Blur()
		m.mode = ModeBrowse
		m.focus = FocusQueue
		m.status = "Capture cancelled."
		return m, nil
	case "ctrl+s":
		id, err := m.service.Capture(m.addBox.Value())
		if err != nil {
			m.status = err.Error()
			return m, nil
		}
		m.mode = ModeBrowse
		m.focus = FocusQueue
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
		m.mode = ModeBrowse
		m.focus = FocusQueue
		m.search.Blur()
		m.status = "Search cancelled."
		return m, nil
	case "enter":
		m.query = strings.TrimSpace(m.search.Value())
		m.mode = ModeBrowse
		m.focus = FocusQueue
		m.search.Blur()
		m.reloadPreserving(0)
		if m.query == "" {
			m.status = "Search cleared."
		} else if len(m.snapshot.Thoughts) == 0 {
			m.status = "No matching thought found. Nothing is wrong."
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
		m.mode = ModeBrowse
		m.focus = FocusQueue
		m.status = "Showing unchanged."
	case "right", "l", "down", "j":
		m.filterIndex = (m.filterIndex + 1) % len(filterKinds)
	case "left", "h", "up", "k":
		m.filterIndex--
		if m.filterIndex < 0 {
			m.filterIndex = len(filterKinds) - 1
		}
	case "enter":
		m.applyFilterIndex(m.filterIndex)
		m.mode = ModeBrowse
		m.focus = FocusQueue
	}
	return m, nil
}

func (m Model) updateHelp(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "?", "q", "enter":
		m.mode = ModeBrowse
		m.focus = FocusQueue
		m.status = ""
	}
	return m, nil
}

func (m Model) updateTend(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = ModeBrowse
		m.focus = FocusQueue
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
			m.mode = ModeBrowse
			m.focus = FocusQueue
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
		m.mode = ModeBrowse
		m.focus = FocusQueue
		m.tendContent.Blur()
		m.tendNote.Blur()
		m.reloadPreserving(item.Thought.ID)
		m.status = "Choose rest, evolve, archive, or release when it feels resolved."
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
			m.mode = ModeBrowse
			m.focus = FocusQueue
			return m, nil
		}
		oldIndex := m.selected
		if err := m.service.ReleasePermanent(id); err != nil {
			m.status = err.Error()
			m.mode = ModeBrowse
			m.focus = FocusQueue
			return m, nil
		}
		m.mode = ModeBrowse
		m.focus = FocusQueue
		m.reloadPreserving(0)
		m.selectIndex(oldIndex)
		m.status = fmt.Sprintf("Released #%d permanently.", id)
	case "n", "N", "esc":
		m.mode = ModeBrowse
		m.focus = FocusQueue
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
		m.status = "This thought is still settling."
		return
	}
	m.mode = ModeTend
	m.focus = FocusPrompt
	m.tendFocus = 0
	m.tendContent.SetValue(item.Thought.Content)
	m.tendNote.Reset()
	m.focusTendInput()
	m.status = ""
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
	m.status = "Remembered."
}

func (m *Model) reloadPreserving(id int64) {
	if id == 0 {
		id = m.selectedID()
	}
	snapshot, err := m.service.SnapshotBloom(m.filter.appFilter(), m.query)
	if err != nil {
		m.status = err.Error()
		return
	}
	m.snapshot = snapshot
	if id != 0 {
		for i, item := range m.snapshot.Thoughts {
			if item.Thought.ID == id {
				m.selected = i
				m.detailOffset = 0
				m.ensureQueueVisible()
				return
			}
		}
	}
	m.ensureUsableSelection()
}

func (m *Model) ensureUsableSelection() {
	if len(m.snapshot.Thoughts) == 0 {
		m.selected = 0
		m.queueOffset = 0
		m.detailOffset = 0
		return
	}
	if m.selected < 0 {
		m.selected = 0
	}
	if m.selected >= len(m.snapshot.Thoughts) {
		m.selected = len(m.snapshot.Thoughts) - 1
	}
	m.ensureQueueVisible()
}

func (m *Model) selectIndex(index int) {
	if len(m.snapshot.Thoughts) == 0 {
		m.selected = 0
		m.queueOffset = 0
		m.detailOffset = 0
		return
	}
	if index < 0 {
		index = 0
	}
	if index >= len(m.snapshot.Thoughts) {
		index = len(m.snapshot.Thoughts) - 1
	}
	if m.selected != index {
		m.detailOffset = 0
	}
	m.selected = index
	m.ensureQueueVisible()
}

func (m *Model) moveSelection(delta int) {
	m.selectIndex(m.selected + delta)
}

func (m *Model) ensureQueueVisible() {
	visible := m.queueVisibleItems()
	if visible <= 0 || len(m.snapshot.Thoughts) == 0 {
		m.queueOffset = 0
		return
	}
	if m.selected < m.queueOffset {
		m.queueOffset = m.selected
	}
	if m.selected >= m.queueOffset+visible {
		m.queueOffset = m.selected - visible + 1
	}
	maxOffset := len(m.snapshot.Thoughts) - visible
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.queueOffset > maxOffset {
		m.queueOffset = maxOffset
	}
	if m.queueOffset < 0 {
		m.queueOffset = 0
	}
}

func (m *Model) scrollDetail(delta int) {
	layout := m.layout()
	innerWidth := maxInt(12, layout.detailWidth-paneStyle.GetHorizontalFrameSize())
	lines := m.detailLines(innerWidth)
	visible := m.detailVisibleLines()
	maxOffset := len(lines) - visible
	if maxOffset < 0 {
		maxOffset = 0
	}
	m.detailOffset += delta
	if m.detailOffset < 0 {
		m.detailOffset = 0
	}
	if m.detailOffset > maxOffset {
		m.detailOffset = maxOffset
	}
}

func (m *Model) toggleFocus() {
	if m.focus == FocusDetail {
		m.focus = FocusQueue
		m.status = "Queue focused."
		return
	}
	m.focus = FocusDetail
	m.status = "Detail focused. Use Ctrl+D and Ctrl+U to scroll."
}

func (m *Model) applyFilterIndex(index int) {
	if index < 0 || index >= len(filterKinds) {
		index = 0
	}
	m.filterIndex = index
	m.filter = filterKinds[index]
	m.selected = 0
	m.queueOffset = 0
	m.detailOffset = 0
	m.reloadPreserving(0)
	m.status = "Showing " + strings.ToLower(m.filter.label()) + "."
	if len(m.snapshot.Thoughts) == 0 && m.query != "" {
		m.status = "No matching thought found. Nothing is wrong."
	}
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
	layout := m.layout()
	inputWidth := layout.bodyWidth - sheetStyle.GetHorizontalFrameSize()
	if inputWidth < 30 {
		inputWidth = 30
	}
	if inputWidth > 96 {
		inputWidth = 96
	}
	m.addBox.SetWidth(inputWidth)
	m.tendContent.SetWidth(inputWidth)
	m.tendNote.SetWidth(inputWidth)
	m.search.Width = minInt(maxInt(24, layout.contentWidth-16), 72)

	editorHeight := maxInt(6, minInt(12, layout.bodyHeight/2))
	noteHeight := maxInt(4, minInt(7, layout.bodyHeight/4))
	m.addBox.SetHeight(editorHeight)
	m.tendContent.SetHeight(editorHeight)
	m.tendNote.SetHeight(noteHeight)
}

func (m Model) selectedItem() (app.BloomThought, bool) {
	if m.selected < 0 || m.selected >= len(m.snapshot.Thoughts) {
		return app.BloomThought{}, false
	}
	return m.snapshot.Thoughts[m.selected], true
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

func (f FilterKind) label() string {
	switch f {
	case FilterReady:
		return "Ready"
	case FilterResting:
		return "Resting"
	default:
		return "All"
	}
}

func (f FilterKind) appFilter() app.BloomFilterKind {
	switch f {
	case FilterReady:
		return app.BloomFilterReady
	case FilterResting:
		return app.BloomFilterResting
	default:
		return app.BloomFilterAll
	}
}

func (f FilterKind) index() int {
	for i, kind := range filterKinds {
		if kind == f {
			return i
		}
	}
	return 0
}

var _ tea.Model = Model{}
