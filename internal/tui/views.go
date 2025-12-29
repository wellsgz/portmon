package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// viewDashboard renders the main dashboard
func (m Model) viewDashboard() string {
	var b strings.Builder

	// Header
	b.WriteString(m.renderHeader())
	b.WriteString("\n")

	// Main content
	if m.connected && m.port > 0 {
		// Period summary and realtime side by side
		summary := m.renderPeriodSummary()
		realtime := m.renderRealtimeStats()

		// Calculate widths
		leftWidth := m.width/2 - 2
		rightWidth := m.width - leftWidth - 4

		summaryPanel := PanelStyle.Width(leftWidth).Render(summary)
		realtimePanel := PanelStyle.Width(rightWidth).Render(realtime)

		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, summaryPanel, " ", realtimePanel))
		b.WriteString("\n")

		// Chart
		chart := m.renderChart()
		chartPanel := PanelStyle.Width(m.width - 2).Render(chart)
		b.WriteString(chartPanel)
	} else if !m.connected {
		errMsg := ErrorStyle.Render("⚠ Not connected to daemon")
		if m.lastError != "" {
			errMsg += "\n" + LabelStyle.Render(m.lastError)
		}
		b.WriteString(PanelStyle.Width(m.width - 2).Render(errMsg))
	} else {
		b.WriteString(PanelStyle.Width(m.width - 2).Render(LabelStyle.Render("No port selected. Use ←/→ to select a port.")))
	}

	b.WriteString("\n")

	// Port selector (at bottom)
	b.WriteString(m.renderPortSelector())
	b.WriteString("\n")

	// Help bar
	b.WriteString(m.renderHelpBar())

	return b.String()
}

// renderHeader renders the top header bar
func (m Model) renderHeader() string {
	var parts []string

	// Title
	parts = append(parts, TitleStyle.Render("portmon"))

	// Port
	if m.port > 0 {
		parts = append(parts, LabelStyle.Render("Port: ")+ValueStyle.Render(fmt.Sprintf("%d", m.port)))
	}

	// Connection status
	if m.connected {
		parts = append(parts, ConnectedStyle.Render(SymbolConn+" Connected"))
	} else {
		parts = append(parts, DisconnectedStyle.Render(SymbolDisconn+" Disconnected"))
	}

	// Uptime
	if m.daemonStatus != nil && m.daemonStatus.Uptime != "" {
		parts = append(parts, LabelStyle.Render("Uptime: ")+ValueStyle.Render(m.daemonStatus.Uptime))
	}

	return lipgloss.JoinHorizontal(lipgloss.Center, "  "+strings.Join(parts, "  │  "))
}

// renderPeriodSummary renders the period summary panel
func (m Model) renderPeriodSummary() string {
	var b strings.Builder

	// Title with date range
	title := PanelTitleStyle.Render("Period Summary")
	if m.historicalStats != nil {
		title += " " + LabelStyle.Render(fmt.Sprintf("(%s to %s)", m.historicalStats.StartDate, m.historicalStats.EndDate))
	}
	b.WriteString(title)
	b.WriteString("\n\n")

	if m.historicalStats == nil {
		b.WriteString(LabelStyle.Render("No data"))
		return b.String()
	}

	// Stats
	stats := m.historicalStats
	b.WriteString(fmt.Sprintf("  %s RX:    %s\n",
		RxStyle.Render(SymbolRx),
		ValueStyle.Render(FormatBytes(stats.TotalRx))))
	b.WriteString(fmt.Sprintf("  %s TX:    %s\n",
		TxStyle.Render(SymbolTx),
		ValueStyle.Render(FormatBytes(stats.TotalTx))))
	b.WriteString(fmt.Sprintf("  %s Total: %s\n",
		TotalStyle.Render(SymbolTotal),
		ValueStyle.Render(FormatBytes(stats.TotalBytes))))

	if stats.PeakRxRate > 0 || stats.PeakTxRate > 0 {
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("  Peak RX: %s\n", RxStyle.Render(FormatRate(float64(stats.PeakRxRate)))))
		b.WriteString(fmt.Sprintf("  Peak TX: %s\n", TxStyle.Render(FormatRate(float64(stats.PeakTxRate)))))
	}

	return b.String()
}

// renderRealtimeStats renders the realtime stats panel
func (m Model) renderRealtimeStats() string {
	var b strings.Builder

	b.WriteString(PanelTitleStyle.Render("Realtime"))
	b.WriteString("\n\n")

	if m.realtimeStats == nil {
		b.WriteString(LabelStyle.Render("No data"))
		return b.String()
	}

	stats := m.realtimeStats

	// Current rates
	b.WriteString(fmt.Sprintf("  Rate:  %s %s  %s %s\n",
		RxStyle.Render(SymbolRx), RxStyle.Render(FormatRate(stats.RxRate)),
		TxStyle.Render(SymbolTx), TxStyle.Render(FormatRate(stats.TxRate))))

	// Totals
	b.WriteString(fmt.Sprintf("  Total: %s %s  %s %s\n",
		RxStyle.Render(SymbolRx), ValueStyle.Render(FormatBytes(stats.RxBytes)),
		TxStyle.Render(SymbolTx), ValueStyle.Render(FormatBytes(stats.TxBytes))))

	// Connections
	b.WriteString(fmt.Sprintf("  Conns: %s active\n",
		ValueStyle.Render(fmt.Sprintf("%d", stats.Connections))))

	return b.String()
}

// renderChart renders the daily traffic bar chart
func (m Model) renderChart() string {
	var b strings.Builder

	b.WriteString(PanelTitleStyle.Render("Daily Traffic"))
	b.WriteString("\n\n")

	if m.historicalStats == nil || len(m.historicalStats.DailyStats) == 0 {
		b.WriteString(LabelStyle.Render("No historical data"))
		return b.String()
	}

	// Find max value for scaling
	var maxBytes uint64
	for _, d := range m.historicalStats.DailyStats {
		total := d.RxBytes + d.TxBytes
		if total > maxBytes {
			maxBytes = total
		}
	}

	if maxBytes == 0 {
		b.WriteString(LabelStyle.Render("No traffic recorded"))
		return b.String()
	}

	// Chart width (leave room for date label and value)
	chartWidth := m.width - 30
	if chartWidth < 20 {
		chartWidth = 20
	}

	// Limit to last 10 days if too many
	stats := m.historicalStats.DailyStats
	if len(stats) > 10 {
		stats = stats[len(stats)-10:]
	}

	// Render bars
	for _, d := range stats {
		// Date label
		dateLabel := ChartLabel.Render(fmt.Sprintf("%-10s", d.Date[5:])) // Just MM-DD

		// Calculate bar lengths
		rxRatio := float64(d.RxBytes) / float64(maxBytes)
		txRatio := float64(d.TxBytes) / float64(maxBytes)

		rxLen := int(rxRatio * float64(chartWidth))
		txLen := int(txRatio * float64(chartWidth))

		if rxLen == 0 && d.RxBytes > 0 {
			rxLen = 1
		}
		if txLen == 0 && d.TxBytes > 0 {
			txLen = 1
		}

		// Build bar
		rxBar := ChartBarRx.Render(strings.Repeat(SymbolBar, rxLen))
		txBar := ChartBarTx.Render(strings.Repeat(SymbolBar, txLen))

		// Value label
		total := d.RxBytes + d.TxBytes
		valueLabel := LabelStyle.Render(FormatBytes(total))

		b.WriteString(fmt.Sprintf("  %s %s%s %s\n", dateLabel, rxBar, txBar, valueLabel))
	}

	// Legend
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s RX  %s TX",
		ChartBarRx.Render(SymbolBar+SymbolBar),
		ChartBarTx.Render(SymbolBar+SymbolBar)))

	return b.String()
}

// renderHelpBar renders the bottom help bar
func (m Model) renderHelpBar() string {
	keys := []string{
		HelpKeyStyle.Render("q") + HelpStyle.Render(" quit"),
		HelpKeyStyle.Render("d") + HelpStyle.Render(" date"),
		HelpKeyStyle.Render("←/→") + HelpStyle.Render(" port"),
		HelpKeyStyle.Render("r") + HelpStyle.Render(" refresh"),
		HelpKeyStyle.Render("?") + HelpStyle.Render(" help"),
	}
	return "  " + strings.Join(keys, "  ")
}

// renderPortSelector renders the horizontal port selector at the bottom
func (m Model) renderPortSelector() string {
	if len(m.ports) == 0 {
		return LabelStyle.Render("  No ports monitored")
	}

	var parts []string
	for i, port := range m.ports {
		desc := m.getPortDescription(port)

		// Truncate description to 32 chars
		if len(desc) > 32 {
			desc = desc[:29] + "..."
		}

		// Format: "port desc" or just "port"
		label := fmt.Sprintf("%d", port)
		if desc != "" {
			label = fmt.Sprintf("%d %s", port, desc)
		}

		// Highlight selected port
		if i == m.portIndex {
			parts = append(parts, SelectedStyle.Render("▸ "+label))
		} else {
			parts = append(parts, LabelStyle.Render("  "+label))
		}
	}

	return "  " + strings.Join(parts, "  │  ")
}

// getPortDescription returns the description for a port, or empty string if none
func (m Model) getPortDescription(port uint16) string {
	for _, info := range m.portInfos {
		if info.Port == port {
			return info.Description
		}
	}
	return ""
}
