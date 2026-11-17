package tui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	subtitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	okStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	warnStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	failStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	labelStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	valueStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)
