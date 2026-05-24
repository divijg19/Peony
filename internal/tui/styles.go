package tui

import "github.com/charmbracelet/lipgloss"

var (
	rootStyle         = lipgloss.NewStyle().Padding(rootPadY, rootPadX).Background(lipgloss.Color("235"))
	titleStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("223"))
	activeLabelStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("223"))
	metaStrongStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("187"))
	metaStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("181"))
	subtleStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("248"))
	hintStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	bodyTextStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	labelStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("187"))
	promptLabelStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("223"))
	countStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Background(lipgloss.Color("239")).Padding(0, 1)
	rowTitleStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	selectedStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("235")).Background(lipgloss.Color("180"))
	selectedMetaStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("236")).Background(lipgloss.Color("180"))
	filterStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Padding(0, 1)
	filterActiveStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("235")).Background(lipgloss.Color("180")).Padding(0, 1)
	paneStyle         = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240")).Padding(1, 2)
	activePaneStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("180")).Padding(1, 2)
	sheetStyle        = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240")).Padding(1, 2)
	smallStyle        = lipgloss.NewStyle()
)
