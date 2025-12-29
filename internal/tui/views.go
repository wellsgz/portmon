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

		// Calculate widths and fixed height
		leftWidth := m.width/2 - 2
		rightWidth := m.width - leftWidth - 4
		panelHeight := 9 // Fixed height for alignment

		summaryPanel := PanelStyle.Width(leftWidth).Height(panelHeight).Render(summary)
		realtimePanel := PanelStyle.Width(rightWidth).Height(panelHeight).Render(realtime)

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
		b.WriteString(PanelStyle.Width(m.width - 2).Render(LabelStyle.Render("No port selected. Use ↑/↓ to select a port.")))
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

	// Default values
	var totalRx, totalTx, totalBytes, peakRx, peakTx uint64
	if m.historicalStats != nil {
		totalRx = m.historicalStats.TotalRx
		totalTx = m.historicalStats.TotalTx
		totalBytes = m.historicalStats.TotalBytes
		peakRx = m.historicalStats.PeakRxRate
		peakTx = m.historicalStats.PeakTxRate
	}

	// Stats (always show all lines)
	b.WriteString(fmt.Sprintf("  %s RX:    %s\n",
		RxStyle.Render(SymbolRx),
		ValueStyle.Render(FormatBytes(totalRx))))
	b.WriteString(fmt.Sprintf("  %s TX:    %s\n",
		TxStyle.Render(SymbolTx),
		ValueStyle.Render(FormatBytes(totalTx))))
	b.WriteString(fmt.Sprintf("  %s Total: %s\n",
		TotalStyle.Render(SymbolTotal),
		ValueStyle.Render(FormatBytes(totalBytes))))

	// Always show Peak lines for consistent height
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  Peak RX: %s\n", RxStyle.Render(FormatRate(float64(peakRx)))))
	b.WriteString(fmt.Sprintf("  Peak TX: %s\n", TxStyle.Render(FormatRate(float64(peakTx)))))

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

// renderChart renders the daily traffic bar chart with fixed height
func (m Model) renderChart() string {
	var b strings.Builder
	const maxDays = 5 // Fixed number of rows

	b.WriteString(PanelTitleStyle.Render("Daily Traffic"))
	b.WriteString("\n\n")

	// Chart width (leave room for date label, bars, and RX/TX values)
	chartWidth := m.width - 60
	if chartWidth < 20 {
		chartWidth = 20
	}

	// Get stats or create empty slice
	var stats []struct {
		Date    string
		RxBytes uint64
		TxBytes uint64
	}

	if m.historicalStats != nil && len(m.historicalStats.DailyStats) > 0 {
		for _, d := range m.historicalStats.DailyStats {
			stats = append(stats, struct {
				Date    string
				RxBytes uint64
				TxBytes uint64
			}{d.Date, d.RxBytes, d.TxBytes})
		}
		// Limit to last maxDays
		if len(stats) > maxDays {
			stats = stats[len(stats)-maxDays:]
		}
	}

	// Find max value for scaling
	var maxBytes uint64
	for _, d := range stats {
		total := d.RxBytes + d.TxBytes
		if total > maxBytes {
			maxBytes = total
		}
	}
	if maxBytes == 0 {
		maxBytes = 1 // Avoid division by zero
	}

	// Render rows (always show maxDays rows for consistent height)
	for i := 0; i < maxDays; i++ {
		if i < len(stats) {
			d := stats[i]
			// Date label
			dateLabel := ChartLabel.Render(fmt.Sprintf("%-6s", d.Date[5:])) // Just MM-DD

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

			// Build bar with padding
			rxBar := ChartBarRx.Render(strings.Repeat(SymbolBar, rxLen))
			txBar := ChartBarTx.Render(strings.Repeat(SymbolBar, txLen))
			padding := strings.Repeat(" ", chartWidth-rxLen-txLen)

			// RX/TX values
			rxVal := RxStyle.Render(fmt.Sprintf("RX:%-8s", FormatBytes(d.RxBytes)))
			txVal := TxStyle.Render(fmt.Sprintf("TX:%-8s", FormatBytes(d.TxBytes)))

			b.WriteString(fmt.Sprintf("  %s %s%s%s  %s %s\n", dateLabel, rxBar, txBar, padding, rxVal, txVal))
		} else {
			// Empty placeholder row
			b.WriteString(fmt.Sprintf("  %s %s\n",
				ChartLabel.Render("------"),
				LabelStyle.Render(strings.Repeat("─", chartWidth)+"  "+strings.Repeat(" ", 28))))
		}
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
		HelpKeyStyle.Render("↑/↓") + HelpStyle.Render(" port"),
		HelpKeyStyle.Render("r") + HelpStyle.Render(" refresh"),
		HelpKeyStyle.Render("?") + HelpStyle.Render(" help"),
	}
	return "  " + strings.Join(keys, "  ")
}

// renderPortSelector renders the vertical port table at the bottom
func (m Model) renderPortSelector() string {
	var b strings.Builder

	if len(m.ports) == 0 {
		return LabelStyle.Render("  No ports monitored")
	}

	// Table header
	b.WriteString(fmt.Sprintf("  %s  │  %s\n",
		PanelTitleStyle.Render(fmt.Sprintf("%-6s", "Port")),
		PanelTitleStyle.Render(fmt.Sprintf("%-32s", "Description"))))
	b.WriteString("  " + strings.Repeat("─", 44) + "\n")

	// Table rows (show max 5 ports around current selection)
	startIdx := 0
	endIdx := len(m.ports)
	if len(m.ports) > 5 {
		startIdx = m.portIndex - 2
		if startIdx < 0 {
			startIdx = 0
		}
		endIdx = startIdx + 5
		if endIdx > len(m.ports) {
			endIdx = len(m.ports)
			startIdx = endIdx - 5
		}
	}

	for i := startIdx; i < endIdx; i++ {
		port := m.ports[i]
		desc := m.getPortDescription(port)

		// Truncate description to 32 chars
		if len(desc) > 32 {
			desc = desc[:29] + "..."
		}

		portStr := fmt.Sprintf("%-6d", port)
		descStr := fmt.Sprintf("%-32s", desc)

		// Highlight selected port
		if i == m.portIndex {
			b.WriteString(fmt.Sprintf("  %s  │  %s\n",
				SelectedStyle.Render("▸"+portStr[:5]),
				SelectedStyle.Render(descStr)))
		} else {
			b.WriteString(fmt.Sprintf("   %s  │  %s\n",
				LabelStyle.Render(portStr),
				LabelStyle.Render(descStr)))
		}
	}

	return b.String()
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
