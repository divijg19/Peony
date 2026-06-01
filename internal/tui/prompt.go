package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) promptBarView(layout frameLayout) string {
	prompt, _ := m.promptContent(layout.contentWidth)
	return renderPromptBar(promptBarStyle, layout.contentWidth, layout.promptHeight, prompt)
}

func (m Model) footerView(layout frameLayout) string {
	_, hints := m.promptContent(layout.contentWidth)
	keys := m.keyLegend(hints, layout.contentWidth-footerStyle.GetHorizontalFrameSize())
	return renderRailRow(footerStyle, layout.contentWidth, keys)
}

func (m Model) promptContent(width int) (string, []keyHint) {
	switch m.mode {
	case ModeSearch:
		return m.searchPrompt(width), searchKeyHints
	case ModeCommand:
		return m.commandPrompt(width), commandKeyHints
	case ModeFilter:
		return m.filterPrompt(width), filterKeyHints
	case ModeReleaseConfirm:
		return m.releasePrompt(width), releaseKeyHints
	case ModeCapture:
		return m.capturePrompt(), captureKeyHints
	case ModeTend:
		return m.tendPrompt(), tendKeyHints
	case ModeHelp:
		return "Key guidance for this view.", helpKeyHints
	default:
		return m.idlePrompt(width), browseKeyHints
	}
}

func (m Model) idlePrompt(width int) string {
	lines := []string{promptLabelStyle.Render("Bloom prompt") + "  " + keyStyle.Render("/") + " " + keyDescStyle.Render("search") + "  " + keyStyle.Render(":") + " " + keyDescStyle.Render("command")}
	if status := strings.TrimSpace(m.status); status != "" {
		lines = append(lines, oneLine(status, maxInt(12, width-4)))
	} else if len(m.commandOutput) > 0 {
		lines = append(lines, oneLine(m.commandOutput[0], maxInt(12, width-4)))
	} else if m.query != "" {
		lines = append(lines, "Search active: "+oneLine(m.query, maxInt(12, width-17)))
	} else {
		lines = append(lines, m.browsePrompt())
	}
	lines = append(lines, m.showingLine(width))
	return strings.Join(lines, "\n")
}

func (m Model) browsePrompt() string {
	status := strings.TrimSpace(m.status)
	if status != "" {
		return status
	}
	item, ok := m.selectedItem()
	if !ok {
		return m.emptyText()
	}
	return fmt.Sprintf("Move gently. Open #%d when you want a closer look.", item.Thought.ID)
}

func (m Model) searchPrompt(width int) string {
	inputWidth := maxInt(12, width-lipgloss.Width("Search  ")-4)
	value := m.search.Value()
	if strings.TrimSpace(value) == "" {
		value = "search thoughts, states, notes, or ids"
	}
	lines := []string{promptLabelStyle.Render("Search") + "  " + oneLine(value+"_", inputWidth)}
	if status := strings.TrimSpace(m.status); status != "" {
		lines = append(lines, oneLine(status, maxInt(12, width-4)))
	}
	lines = append(lines, m.showingLine(width))
	return strings.Join(lines, "\n")
}

func (m Model) commandPrompt(width int) string {
	inputWidth := maxInt(12, width-lipgloss.Width("Command  ")-4)
	value := m.command.Value()
	if strings.TrimSpace(value) == "" {
		value = "type a Bloom command"
	}
	lines := []string{promptLabelStyle.Render("Command") + "  " + oneLine(value+"_", inputWidth)}
	if status := strings.TrimSpace(m.status); status != "" {
		lines = append(lines, oneLine(status, maxInt(12, width-4)))
	} else if len(m.commandOutput) > 0 {
		lines = append(lines, oneLine(m.commandOutput[0], maxInt(12, width-4)))
	} else {
		lines = append(lines, "A quiet command space is listening.")
	}
	lines = append(lines, m.showingLine(width))
	return strings.Join(lines, "\n")
}

func (m Model) filterPrompt(width int) string {
	parts := make([]string, 0, len(filterKinds))
	for i, kind := range filterKinds {
		label := kind.label()
		if i == m.filterIndex {
			label = "[" + label + "]"
		}
		parts = append(parts, label)
	}
	return strings.Join([]string{
		promptLabelStyle.Render("Showing") + "  " + strings.Join(parts, "  "),
		m.showingLine(width),
	}, "\n")
}

func (m Model) capturePrompt() string {
	if strings.TrimSpace(m.status) != "" {
		return m.status
	}
	return "Hold the thought in its original shape."
}

func (m Model) tendPrompt() string {
	if strings.TrimSpace(m.status) != "" {
		return m.status
	}
	if m.tendFocus == 1 {
		return "Add a note if one belongs with this tending."
	}
	return "Revise softly, then decide what comes next."
}

func (m Model) releasePrompt(width int) string {
	line := "Release this thought permanently? This deletes the thought, history, and reindexes local IDs."
	if m.pendingReleaseID != 0 {
		if item, err := m.service.Thought(m.pendingReleaseID); err == nil {
			preview := oneLine(item.Thought.Content, maxInt(10, width-96))
			line = fmt.Sprintf("Release #%d permanently? %s This deletes the thought, history, and reindexes local IDs.", item.Thought.ID, preview)
		}
	} else if item, ok := m.selectedItem(); ok {
		preview := oneLine(item.Thought.Content, maxInt(10, width-96))
		line = fmt.Sprintf("Release #%d permanently? %s This deletes the thought, history, and reindexes local IDs.", item.Thought.ID, preview)
	}
	return line
}

func (m Model) keyLegend(hints []keyHint, width int) string {
	if len(hints) == 0 {
		return ""
	}
	if len(hints) > 8 {
		return m.plainKeyLegend(hints, width)
	}
	parts := make([]string, 0, len(hints))
	for _, hint := range hints {
		parts = append(parts, keyStyle.Render(hint.Key)+" "+keyDescStyle.Render(hint.Label))
	}
	line := strings.Join(parts, "  ")
	if lipgloss.Width(line) <= width {
		return line
	}
	return m.plainKeyLegend(hints, width)
}

func (m Model) plainKeyLegend(hints []keyHint, width int) string {
	plain := make([]string, 0, len(hints))
	for _, hint := range hints {
		plain = append(plain, hint.Key+" "+hint.Label)
	}
	return oneLine(strings.Join(plain, "  "), width)
}

func (m Model) hasContextOutput(width, promptHeight int) bool {
	if width < contextOutputWidth {
		return false
	}
	contextWidth := minInt(34, maxInt(28, width*22/100))
	innerWidth := maxInt(12, contextWidth-outputStyle.GetHorizontalFrameSize())
	promptLines := maxInt(1, promptHeight-promptBarStyle.GetVerticalFrameSize())
	return len(m.contextOutputLines(innerWidth, promptLines)) > 0
}
