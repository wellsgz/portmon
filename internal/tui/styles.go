// Package tui provides the terminal user interface for portmon.
package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Colors
var (
	primaryColor   = lipgloss.Color("#7C3AED") // Purple
	secondaryColor = lipgloss.Color("#10B981") // Green
	accentColor    = lipgloss.Color("#F59E0B") // Amber
	errorColor     = lipgloss.Color("#EF4444") // Red
	mutedColor     = lipgloss.Color("#6B7280") // Gray
	textColor      = lipgloss.Color("#F3F4F6") // Light gray
	bgColor        = lipgloss.Color("#1F2937") // Dark gray
)

// Styles
var (
	// Base styles
	BaseStyle = lipgloss.NewStyle().
			Foreground(textColor)

	// Title bar
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			Background(bgColor).
			Padding(0, 1)

	// Status indicators
	ConnectedStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true)

	DisconnectedStyle = lipgloss.NewStyle().
				Foreground(errorColor).
				Bold(true)

	// Panel styles
	PanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(mutedColor).
			Padding(0, 1)

	PanelTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor)

	// Stats display
	LabelStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	ValueStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(textColor)

	RxStyle = lipgloss.NewStyle().
		Foreground(secondaryColor).
		Bold(true)

	TxStyle = lipgloss.NewStyle().
		Foreground(accentColor).
		Bold(true)

	TotalStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	// Chart styles
	ChartBarRx = lipgloss.NewStyle().
			Foreground(secondaryColor)

	ChartBarTx = lipgloss.NewStyle().
			Foreground(accentColor)

	ChartLabel = lipgloss.NewStyle().
			Foreground(mutedColor)

	// Help bar
	HelpStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	HelpKeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor)

	// Modal styles
	ModalStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(primaryColor).
			Padding(1, 2).
			Background(bgColor)

	ModalTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			MarginBottom(1)

	SelectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(textColor).
			Background(primaryColor).
			Padding(0, 1)

	UnselectedStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Padding(0, 1)

	// Error display
	ErrorStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)
)

// Symbols
const (
	SymbolRx      = "▼"
	SymbolTx      = "▲"
	SymbolTotal   = "⇅"
	SymbolConn    = "●"
	SymbolDisconn = "○"
	SymbolBar     = "█"
	SymbolBarHalf = "▌"
	SymbolBarBg   = "░"
)
