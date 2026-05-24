package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/divijg19/peony/internal/app"
	"github.com/divijg19/peony/internal/core"
)

func (m Model) headerView(layout frameLayout) string {
	line1 := alignRow(
		layout.contentWidth,
		titleStyle.Render("Bloom")+"  "+subtleStyle.Render("a soft place for unfinished thoughts"),
		metaStrongStyle.Render(fmt.Sprintf("Ready %d", m.snapshot.ReadyCount)),
	)
	line2 := subtleStyle.Render(m.showingLine(layout.contentWidth))
	return strings.Join([]string{line1, line2}, "\n")
}

func (m Model) showingLine(width int) string {
	focus := "queue"
	if m.focus == FocusDetail {
		focus = "detail"
	} else if m.focus == FocusPrompt {
		focus = "prompt"
	}
	parts := []string{fmt.Sprintf("Showing %s", m.filter.label()), fmt.Sprintf("%s focus", focus)}
	if item, ok := m.selectedItem(); ok {
		parts = append(parts, fmt.Sprintf("#%d %s", item.Thought.ID, m.stateLabel(item)))
	} else {
		parts = append(parts, "no thought selected")
	}
	if query := strings.TrimSpace(m.query); query != "" {
		parts = append(parts, fmt.Sprintf("search %q", oneLine(query, 24)))
	}
	return oneLine(strings.Join(parts, "  ·  "), width)
}

func (m Model) queueView(width, height int) string {
	innerWidth := maxInt(12, width-paneStyle.GetHorizontalFrameSize())
	innerHeight := maxInt(3, height-paneStyle.GetVerticalFrameSize())
	lines := []string{m.queueTitle(innerWidth)}
	rowsHeight := maxInt(1, innerHeight-len(lines))
	lines = append(lines, m.queueRows(innerWidth, rowsHeight)...)
	return renderBox(paneStyle, width, height, strings.Join(fitLines(lines, innerHeight), "\n"))
}

func (m Model) queueTitle(width int) string {
	count := len(m.snapshot.Thoughts)
	label := fmt.Sprintf("Queue  %s", countStyle.Render(fmt.Sprintf("%d", count)))
	if m.focus == FocusQueue && (m.mode == ModeBrowse || m.mode == ModeReleaseConfirm || m.mode == ModeSearch || m.mode == ModeFilter) {
		label = activeLabelStyle.Render("Queue") + " " + countStyle.Render(fmt.Sprintf("%d", count))
	}
	return oneLine(label, width)
}

func (m Model) queueRows(width, height int) []string {
	if height <= 0 {
		return nil
	}
	if len(m.snapshot.Thoughts) == 0 {
		return fitLines(wrapText(m.emptyText(), width, subtleStyle), height)
	}

	visibleItems := maxInt(1, height/2)
	start := m.queueOffset
	if start < 0 {
		start = 0
	}
	if start >= len(m.snapshot.Thoughts) {
		start = maxInt(0, len(m.snapshot.Thoughts)-1)
	}
	end := minInt(len(m.snapshot.Thoughts), start+visibleItems)
	rows := make([]string, 0, visibleItems*2+1)
	if start > 0 {
		rows = append(rows, subtleStyle.Render(fmt.Sprintf("%d above", start)))
	}
	for i := start; i < end; i++ {
		item := m.snapshot.Thoughts[i]
		selected := i == m.selected && m.focus == FocusQueue && m.mode == ModeBrowse
		rows = append(rows, m.queueRow(item, width, selected)...)
	}
	if end < len(m.snapshot.Thoughts) {
		rows = append(rows, subtleStyle.Render(fmt.Sprintf("%d more", len(m.snapshot.Thoughts)-end)))
	}
	return fitLines(rows, height)
}

func (m Model) queueRow(item app.BloomThought, width int, selected bool) []string {
	preview := fmt.Sprintf("#%d  %s", item.Thought.ID, oneLine(item.Thought.Content, maxInt(8, width-6)))
	meta := fmt.Sprintf("%s  |  tended %dx", m.readinessLabel(item), item.Thought.TendCounter)
	if selected {
		return []string{
			selectedStyle.Width(width).Render(oneLine(preview, width)),
			selectedMetaStyle.Width(width).Render(oneLine(meta, width)),
		}
	}
	return []string{
		rowTitleStyle.Width(width).Render(oneLine(preview, width)),
		subtleStyle.Width(width).Render(oneLine(meta, width)),
	}
}

func (m Model) detailView(width, height int) string {
	innerWidth := maxInt(12, width-paneStyle.GetHorizontalFrameSize())
	innerHeight := maxInt(3, height-paneStyle.GetVerticalFrameSize())
	lines := m.detailLines(innerWidth)
	if len(lines) == 0 {
		lines = []string{"Select a thought to see its shape."}
	}
	offset := clampInt(m.detailOffset, 0, maxInt(0, len(lines)-innerHeight))
	visible := fitLines(lines[offset:], innerHeight)
	if offset > 0 && len(visible) > 0 {
		visible[0] = subtleStyle.Render(fmt.Sprintf("%d above", offset))
	}
	if offset+innerHeight < len(lines) && len(visible) > 0 {
		visible[len(visible)-1] = subtleStyle.Render(fmt.Sprintf("%d more", len(lines)-offset-innerHeight))
	}
	content := strings.Join(widthLines(visible, innerWidth), "\n")
	style := paneStyle
	if m.focus == FocusDetail && (m.mode == ModeBrowse || m.mode == ModeReleaseConfirm) {
		style = activePaneStyle
	}
	return renderBox(style, width, height, content)
}

func (m Model) detailLines(width int) []string {
	item, ok := m.selectedItem()
	if !ok {
		return nil
	}
	t := item.Thought
	lines := []string{
		activeLabelStyle.Render("Selected thought"),
		fmt.Sprintf("#%d  %s", t.ID, m.stateLabel(item)),
		fmt.Sprintf("%s  |  tended %d times", m.readinessLabel(item), t.TendCounter),
		"",
	}
	lines = append(lines, wrapText(t.Content, width, bodyTextStyle)...)
	lines = append(lines,
		"",
		labelStyle.Render("When"),
		fmt.Sprintf("Created  %s", t.CreatedAt.UTC().Format("2006-01-02 15:04Z")),
		fmt.Sprintf("Updated  %s", t.UpdatedAt.UTC().Format("2006-01-02 15:04Z")),
	)
	if t.LastTendedAt != nil {
		lines = append(lines, fmt.Sprintf("Tended   %s", t.LastTendedAt.UTC().Format("2006-01-02 15:04Z")))
	}
	if len(item.Events) > 0 {
		lines = append(lines, "", labelStyle.Render("History"))
		for _, event := range item.Events {
			line := fmt.Sprintf("%s  %s", event.At.UTC().Format("2006-01-02"), event.Kind)
			if event.NextState != nil {
				line += fmt.Sprintf(" -> %s", *event.NextState)
			}
			lines = append(lines, line)
			if event.Note != nil && strings.TrimSpace(*event.Note) != "" {
				lines = append(lines, subtleStyle.Render("  "+oneLine(*event.Note, maxInt(8, width-2))))
			}
		}
	}
	return lines
}

func (m Model) captureView(layout frameLayout) string {
	body := lipgloss.JoinVertical(
		lipgloss.Left,
		activeLabelStyle.Render("Capture"),
		subtleStyle.Render("Leave a thought here exactly as it arrives."),
		"",
		m.addBox.View(),
	)
	return renderBox(sheetStyle, layout.bodyWidth, layout.bodyHeight, body)
}

func (m Model) tendView(layout frameLayout) string {
	body := strings.Join([]string{
		activeLabelStyle.Render("Tend"),
		subtleStyle.Render("Shape the thought gently, then decide what comes next."),
		"",
		labelStyle.Render("Content"),
		m.tendContent.View(),
		"",
		labelStyle.Render("Note"),
		m.tendNote.View(),
	}, "\n")
	return renderBox(sheetStyle, layout.bodyWidth, layout.bodyHeight, body)
}

func (m Model) helpView(layout frameLayout) string {
	lines := keyHelpLines(m.mode)
	return renderBox(sheetStyle, layout.bodyWidth, layout.bodyHeight, strings.Join(fitLines(lines, maxInt(3, layout.bodyHeight-sheetStyle.GetVerticalFrameSize())), "\n"))
}

func (m Model) emptyText() string {
	if strings.TrimSpace(m.query) != "" {
		return "No matching thought found. Nothing is wrong."
	}
	switch m.filter {
	case FilterReady:
		return "Nothing needs you right now."
	case FilterResting:
		return "Your thoughts are settling."
	case FilterMemory:
		return "Nothing has been placed in memory yet."
	default:
		return "Nothing is asking for your attention."
	}
}

func (m Model) stateLabel(item app.BloomThought) string {
	if item.Ready {
		return "ready"
	}
	switch item.Thought.CurrentState {
	case core.StateCaptured, core.StateResting:
		return "settling"
	case core.StateTended:
		return "needs resolution"
	case core.StateEvolved:
		return "evolved"
	case core.StateArchived:
		return "remembered"
	default:
		return string(item.Thought.CurrentState)
	}
}

func (m Model) readinessLabel(item app.BloomThought) string {
	now := time.Now().UTC()
	if item.Ready {
		return "ready now"
	}
	if item.Thought.CurrentState == core.StateTended {
		return "needs resolution"
	}
	if item.Thought.CurrentState == core.StateEvolved || item.Thought.CurrentState == core.StateArchived {
		return m.stateLabel(item)
	}
	return "eligible " + relativeTime(item.Thought.EligibilityAt, now)
}
