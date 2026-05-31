package tui

import "github.com/charmbracelet/lipgloss"

type layoutKind int

const (
	layoutSmall layoutKind = iota
	layoutWide
	layoutMedium
	layoutCompact
)

type frameLayout struct {
	kind          layoutKind
	width         int
	height        int
	contentWidth  int
	contentHeight int
	headerHeight  int
	actionHeight  int
	promptHeight  int
	footerHeight  int
	bodyWidth     int
	bodyHeight    int
	mainWidth     int
	queueWidth    int
	queueHeight   int
	detailWidth   int
	detailHeight  int
	outputWidth   int
	outputHeight  int
}

const (
	minWidth           = 56
	minHeight          = 16
	wideWidth          = 110
	outputDrawerWidth  = 132
	mediumWidth        = 80
	rootPadX           = 0
	rootPadY           = 0
	nameHeaderHeight   = 1
	actionHeaderHeight = 2
	footerHeight       = 1
	bodyGap            = 0
	paneGap            = 1
)

func (m Model) layout() frameLayout {
	width := m.width
	height := m.height
	if width <= 0 {
		width = 120
	}
	if height <= 0 {
		height = 36
	}
	layout := frameLayout{
		width:         width,
		height:        height,
		contentWidth:  maxInt(1, width-rootPadX*2),
		contentHeight: maxInt(1, height-rootPadY*2),
		headerHeight:  nameHeaderHeight + actionHeaderHeight,
		actionHeight:  actionHeaderHeight,
		footerHeight:  footerHeight,
	}
	if width < minWidth || height < minHeight {
		layout.kind = layoutSmall
		return layout
	}
	if m.hasPromptRow() {
		layout.promptHeight = minInt(3, maxInt(1, layout.contentHeight/5))
	}
	layout.bodyWidth = layout.contentWidth
	layout.bodyHeight = maxInt(1, layout.contentHeight-layout.headerHeight-layout.promptHeight-layout.footerHeight)
	layout.mainWidth = layout.bodyWidth

	switch {
	case width >= wideWidth:
		layout.kind = layoutWide
		if width >= outputDrawerWidth {
			layout.outputWidth = minInt(34, maxInt(28, layout.contentWidth*22/100))
			layout.outputHeight = layout.bodyHeight
			layout.mainWidth = maxInt(40, layout.contentWidth-layout.outputWidth-paneGap)
		}
		queueWidth := layout.mainWidth * 44 / 100
		queueWidth = minInt(60, maxInt(44, queueWidth))
		maxQueue := layout.mainWidth * 50 / 100
		if queueWidth > maxQueue {
			queueWidth = maxQueue
		}
		available := layout.mainWidth - paneGap
		layout.queueWidth = queueWidth
		layout.detailWidth = maxInt(24, available-queueWidth)
		layout.queueHeight = layout.bodyHeight
		layout.detailHeight = layout.bodyHeight
	case width >= mediumWidth:
		layout.kind = layoutMedium
		available := maxInt(2, layout.bodyHeight-paneGap)
		layout.queueWidth = layout.bodyWidth
		layout.detailWidth = layout.bodyWidth
		layout.queueHeight = maxInt(5, available*11/20)
		if layout.queueHeight > available-4 {
			layout.queueHeight = maxInt(3, available-4)
		}
		layout.detailHeight = maxInt(1, available-layout.queueHeight)
	default:
		layout.kind = layoutCompact
		layout.queueWidth = layout.bodyWidth
		layout.detailWidth = layout.bodyWidth
		layout.queueHeight = layout.bodyHeight
		layout.detailHeight = layout.bodyHeight
	}
	return layout
}

func (m Model) View() string {
	layout := m.layout()
	if layout.kind == layoutSmall {
		return smallStyle.Width(layout.width).Height(layout.height).Render(
			lipgloss.Place(layout.width, layout.height, lipgloss.Center, lipgloss.Center, "Peony needs a little more room to bloom."),
		)
	}

	header := m.headerView(layout)
	action := m.actionHeaderView(layout)
	body := m.bodyView(layout)
	parts := []string{header, action, body}
	if layout.promptHeight > 0 {
		parts = append(parts, m.promptBoxView(layout))
	}
	parts = append(parts, m.footerView(layout))
	content := lipgloss.JoinVertical(lipgloss.Left, parts...)
	content = lipgloss.Place(layout.contentWidth, layout.contentHeight, lipgloss.Left, lipgloss.Top, content)
	return rootStyle.Width(layout.contentWidth).Height(layout.contentHeight).Render(content)
}

func (m Model) bodyView(layout frameLayout) string {
	switch m.mode {
	case ModeCapture:
		return m.captureView(layout)
	case ModeTend:
		return m.tendView(layout)
	case ModeHelp:
		return m.helpView(layout)
	default:
		return m.browseView(layout)
	}
}

func (m Model) browseView(layout frameLayout) string {
	if layout.kind == layoutCompact {
		if m.focus == FocusDetail {
			return m.detailView(layout.detailWidth, layout.detailHeight)
		}
		return m.queueView(layout.queueWidth, layout.queueHeight)
	}
	if layout.kind == layoutMedium {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			m.queueView(layout.queueWidth, layout.queueHeight),
			stringsRepeat(" ", paneGap),
			m.detailView(layout.detailWidth, layout.detailHeight),
		)
	}
	main := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.queueView(layout.queueWidth, layout.queueHeight),
		stringsRepeat(" ", paneGap),
		m.detailView(layout.detailWidth, layout.detailHeight),
	)
	if layout.outputWidth == 0 {
		return main
	}
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		main,
		stringsRepeat(" ", paneGap),
		m.outputDrawerView(layout.outputWidth, layout.outputHeight),
	)
}

func (m Model) queueVisibleItems() int {
	layout := m.layout()
	height := layout.queueHeight - paneStyle.GetVerticalFrameSize() - 2
	if height < 1 {
		return 1
	}
	return maxInt(1, height/2)
}

func (m Model) detailVisibleLines() int {
	layout := m.layout()
	height := layout.detailHeight - paneStyle.GetVerticalFrameSize() - 4
	if height < 1 {
		return 1
	}
	return height
}

func stringsRepeat(s string, count int) string {
	out := ""
	for i := 0; i < count; i++ {
		out += s
	}
	return out
}
