package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Preset labels
var presetLabels = []string{
	"Today",
	"Yesterday",
	"Last 7 days",
	"Last 30 days",
	"This month",
	"Last month",
	"Custom billing cycle",
}

// viewDatePicker renders the date range picker modal
func (m Model) viewDatePicker() string {
	var b strings.Builder

	b.WriteString(ModalTitleStyle.Render("Select Date Range"))
	b.WriteString("\n\n")

	for i, label := range presetLabels {
		cursor := "  "
		style := UnselectedStyle
		if i == m.presetCursor {
			cursor = "▶ "
			style = SelectedStyle
		}

		line := fmt.Sprintf("%s%s", cursor, style.Render(label))

		// Add cycle day info for custom cycle
		if i == int(PresetCustomCycle) && m.cycleDay > 0 {
			line += LabelStyle.Render(fmt.Sprintf(" (day %d)", m.cycleDay))
		}

		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(HelpStyle.Render("↑/↓ navigate  Enter select  Esc cancel"))

	content := b.String()

	// Center the modal
	modal := ModalStyle.Render(content)

	// Calculate centering
	modalWidth := lipgloss.Width(modal)
	modalHeight := lipgloss.Height(modal)

	padLeft := (m.width - modalWidth) / 2
	padTop := (m.height - modalHeight) / 2

	if padLeft < 0 {
		padLeft = 0
	}
	if padTop < 0 {
		padTop = 0
	}

	// Build centered view
	var out strings.Builder
	out.WriteString(strings.Repeat("\n", padTop))
	for _, line := range strings.Split(modal, "\n") {
		out.WriteString(strings.Repeat(" ", padLeft))
		out.WriteString(line)
		out.WriteString("\n")
	}

	return out.String()
}

// viewHelp renders the help modal
func (m Model) viewHelp() string {
	var b strings.Builder

	b.WriteString(ModalTitleStyle.Render("Keyboard Shortcuts"))
	b.WriteString("\n\n")

	helpItems := []struct{ key, desc string }{
		{"q", "Quit"},
		{"d", "Change date range"},
		{"n/N", "Next/Previous port"},
		{"r", "Refresh data"},
		{"?", "Toggle help"},
		{"Esc", "Close modal"},
	}

	for _, item := range helpItems {
		b.WriteString(fmt.Sprintf("  %s  %s\n",
			HelpKeyStyle.Render(fmt.Sprintf("%-6s", item.key)),
			HelpStyle.Render(item.desc)))
	}

	b.WriteString("\n")
	b.WriteString(HelpStyle.Render("Press any key to close"))

	content := b.String()
	modal := ModalStyle.Render(content)

	// Center
	modalWidth := lipgloss.Width(modal)
	modalHeight := lipgloss.Height(modal)

	padLeft := (m.width - modalWidth) / 2
	padTop := (m.height - modalHeight) / 2

	if padLeft < 0 {
		padLeft = 0
	}
	if padTop < 0 {
		padTop = 0
	}

	var out strings.Builder
	out.WriteString(strings.Repeat("\n", padTop))
	for _, line := range strings.Split(modal, "\n") {
		out.WriteString(strings.Repeat(" ", padLeft))
		out.WriteString(line)
		out.WriteString("\n")
	}

	return out.String()
}
