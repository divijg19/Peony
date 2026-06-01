package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

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

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func fitLines(lines []string, maxLines int) []string {
	if maxLines <= 0 {
		return nil
	}
	if len(lines) <= maxLines {
		return lines
	}
	if maxLines == 1 {
		return []string{"..."}
	}
	fitted := append([]string{}, lines[:maxLines-1]...)
	fitted = append(fitted, subtleStyle.Render("..."))
	return fitted
}

func wrapText(text string, width int, style lipgloss.Style) []string {
	if width < 1 {
		width = 1
	}
	rendered := style.Width(width).Render(text)
	return strings.Split(rendered, "\n")
}

func widthLines(lines []string, width int) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = oneLine(line, width)
	}
	return out
}

func renderBox(style lipgloss.Style, width, height int, content string) string {
	contentWidth := maxInt(1, width-2)
	contentHeight := maxInt(1, height-2)
	return style.Width(contentWidth).Height(contentHeight).Render(content)
}

func renderPromptBar(style lipgloss.Style, width, height int, content string) string {
	contentWidth := maxInt(1, width-style.GetHorizontalFrameSize())
	contentHeight := maxInt(1, height-style.GetVerticalFrameSize())
	lines := strings.Split(content, "\n")
	lines = fitLines(widthLines(lines, maxInt(1, contentWidth-style.GetHorizontalFrameSize())), contentHeight)
	return exactWidth(style.Width(contentWidth).Height(contentHeight).Render(strings.Join(lines, "\n")), width)
}

func renderRailRow(style lipgloss.Style, width int, content string) string {
	contentWidth := maxInt(1, width-style.GetHorizontalFrameSize())
	return exactWidth(style.Width(contentWidth).Render(oneLine(content, maxInt(1, contentWidth-2))), width)
}

func alignRow(width int, left string, right string) string {
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	gap := width - leftWidth - rightWidth
	if gap < 1 {
		left = oneLine(left, maxInt(1, width-rightWidth-1))
		leftWidth = lipgloss.Width(left)
		gap = width - leftWidth - rightWidth
	}
	if gap < 1 {
		right = oneLine(right, maxInt(1, width-leftWidth-1))
		rightWidth = lipgloss.Width(right)
		gap = width - leftWidth - rightWidth
	}
	if gap < 1 {
		gap = 1
	}
	return left + stringsRepeat(" ", gap) + right
}

func exactWidth(rendered string, width int) string {
	if width <= 0 {
		return rendered
	}
	lines := strings.Split(rendered, "\n")
	for i, line := range lines {
		if gap := width - lipgloss.Width(line); gap > 0 {
			lines[i] = line + stringsRepeat(" ", gap)
		}
	}
	return strings.Join(lines, "\n")
}
