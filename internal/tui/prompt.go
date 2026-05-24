package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) promptRailView(layout frameLayout) string {
	body := m.browseRail()
	switch m.mode {
	case ModeSearch:
		body = m.searchRail()
	case ModeFilter:
		body = m.filterRail()
	case ModeReleaseConfirm:
		body = m.releaseRail(layout.contentWidth)
	case ModeCapture:
		body = railLines("Hold the thought in its original shape.", "Ctrl+S save  Esc cancel")
	case ModeTend:
		body = railLines("Revise softly, then decide what comes next.", "Tab switch fields  Ctrl+S mark tended  Esc cancel")
	case ModeHelp:
		body = railLines("Key guidance for this view.", "Esc, Enter, ?, or q close help")
	}
	lines := strings.Split(clampRail(body, layout.contentWidth), "\n")
	for len(lines) < 2 {
		lines = append(lines, "")
	}
	separator := subtleStyle.Render(stringsRepeat("-", maxInt(1, layout.contentWidth-4)))
	return strings.Join([]string{separator, oneLine(lines[0], layout.contentWidth-4), hintStyle.Render(oneLine(lines[1], layout.contentWidth-4))}, "\n")
}

func (m Model) browseRail() string {
	status := strings.TrimSpace(m.status)
	if status == "" {
		status = "Move gently. Open a thought when you want a closer look."
	}
	return railLines(status, browseKeyHelp)
}

func (m Model) searchRail() string {
	line := promptLabelStyle.Render("Search") + "  " + m.search.View()
	return railLines(line, "Enter apply  Ctrl+U clear  Esc cancel")
}

func (m Model) filterRail() string {
	parts := make([]string, 0, len(filterKinds))
	for i, kind := range filterKinds {
		label := kind.label()
		if i == m.filterIndex {
			parts = append(parts, filterActiveStyle.Render(label))
		} else {
			parts = append(parts, filterStyle.Render(label))
		}
	}
	line := promptLabelStyle.Render("Filter") + "  " + strings.Join(parts, "  ")
	return railLines(line, "h/l choose  Enter apply  Esc cancel")
}

func (m Model) releaseRail(width int) string {
	line := "Release this thought permanently? This deletes the thought, history, and reindexes local IDs."
	if item, ok := m.selectedItem(); ok {
		preview := oneLine(item.Thought.Content, maxInt(12, width-78))
		line = fmt.Sprintf("Release #%d permanently? %s", item.Thought.ID, preview)
	}
	return railLines(promptLabelStyle.Render("Release")+"  "+line, "This deletes the thought, history, and reindexes local IDs.  y confirm  n cancel  Esc cancel")
}

func railLines(first, second string) string {
	return lipgloss.JoinVertical(lipgloss.Left, first, hintStyle.Render(second))
}

func clampRail(body string, width int) string {
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		lines[i] = oneLine(line, width)
	}
	return strings.Join(fitLines(lines, 2), "\n")
}
