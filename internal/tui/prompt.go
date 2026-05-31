package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) hasPromptRow() bool {
	switch m.mode {
	case ModeSearch, ModeFilter, ModeReleaseConfirm, ModeCapture, ModeTend:
		return true
	default:
		return false
	}
}

func (m Model) promptBoxView(layout frameLayout) string {
	prompt, _ := m.promptContent(layout.contentWidth)
	return renderPromptBox(promptBoxStyle, layout.contentWidth, layout.promptHeight, prompt)
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
	case ModeFilter:
		return m.filterPrompt(), filterKeyHints
	case ModeReleaseConfirm:
		return m.releasePrompt(width), releaseKeyHints
	case ModeCapture:
		return m.capturePrompt(), captureKeyHints
	case ModeTend:
		return m.tendPrompt(), tendKeyHints
	case ModeHelp:
		return "Key guidance for this view.", helpKeyHints
	default:
		return m.browsePrompt(), browseKeyHints
	}
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
	return "Search  " + oneLine(value+"_", inputWidth)
}

func (m Model) filterPrompt() string {
	parts := make([]string, 0, len(filterKinds))
	for i, kind := range filterKinds {
		label := kind.label()
		if i == m.filterIndex {
			label = "[" + label + "]"
		}
		parts = append(parts, label)
	}
	return "Showing  " + strings.Join(parts, "  ")
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
	if item, ok := m.selectedItem(); ok {
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
