// portmon is the CLI/TUI client for the port traffic monitor.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/wellsgz/portmon/internal/client"
	"github.com/wellsgz/portmon/internal/storage"
	"github.com/wellsgz/portmon/internal/tui"
)

var (
	socketPath string
	port       uint16
	outputJSON bool
	fromDate   string
	toDate     string
	cycleDay   int
	today      bool
	thisMonth  bool
	last7Days  bool
	last30Days bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "portmon",
		Short: "Port traffic monitor client",
		Long:  `portmon is a client for querying traffic statistics from the portmond daemon.`,
	}

	rootCmd.PersistentFlags().StringVar(&socketPath, "socket", "", "Unix socket path (default: /run/portmon/portmon.sock)")

	// TUI command
	tuiCmd := &cobra.Command{
		Use:   "tui",
		Short: "Launch interactive terminal UI",
		RunE:  runTUI,
	}
	tuiCmd.Flags().Uint16VarP(&port, "port", "p", 0, "Initial port to display (optional)")

	// Stats command
	statsCmd := &cobra.Command{
		Use:   "stats",
		Short: "Show traffic statistics",
		RunE:  runStats,
	}
	statsCmd.Flags().Uint16VarP(&port, "port", "p", 0, "Port to query (required)")
	statsCmd.Flags().BoolVar(&outputJSON, "json", false, "Output in JSON format")
	statsCmd.Flags().StringVar(&fromDate, "from", "", "Start date (YYYY-MM-DD)")
	statsCmd.Flags().StringVar(&toDate, "to", "", "End date (YYYY-MM-DD)")
	statsCmd.Flags().IntVar(&cycleDay, "cycle-day", 0, "Billing cycle day (1-28)")
	statsCmd.Flags().BoolVar(&today, "today", false, "Show today's stats")
	statsCmd.Flags().BoolVar(&thisMonth, "this-month", false, "Show this month's stats")
	statsCmd.Flags().BoolVar(&last7Days, "last-7-days", false, "Show last 7 days")
	statsCmd.Flags().BoolVar(&last30Days, "last-30-days", false, "Show last 30 days")
	statsCmd.MarkFlagRequired("port")

	// Status command
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show daemon status",
		RunE:  runStatus,
	}

	// List ports command
	listPortsCmd := &cobra.Command{
		Use:   "list-ports",
		Short: "List monitored ports",
		RunE:  runListPorts,
	}

	// Add port command
	addPortCmd := &cobra.Command{
		Use:   "add-port PORT",
		Short: "Add a port to monitor",
		Args:  cobra.ExactArgs(1),
		RunE:  runAddPort,
	}

	// Remove port command
	removePortCmd := &cobra.Command{
		Use:   "remove-port PORT",
		Short: "Remove a port from monitoring",
		Args:  cobra.ExactArgs(1),
		RunE:  runRemovePort,
	}

	rootCmd.AddCommand(tuiCmd, statsCmd, statusCmd, listPortsCmd, addPortCmd, removePortCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runTUI(cmd *cobra.Command, args []string) error {
	model := tui.New(socketPath, port)
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func getClient() (*client.Client, error) {
	c := client.New(socketPath)
	if err := c.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to daemon: %w\nIs portmond running?", err)
	}
	return c, nil
}

func runStats(cmd *cobra.Command, args []string) error {
	c, err := getClient()
	if err != nil {
		return err
	}
	defer c.Close()

	now := time.Now()

	// Determine date range
	var startDate, endDate string

	switch {
	case today:
		start, end := storage.GetTodayDates(now)
		startDate, endDate = storage.FormatDateRange(start, end)
	case thisMonth:
		start, end := storage.GetCurrentMonthDates(now)
		startDate, endDate = storage.FormatDateRange(start, end)
	case last7Days:
		start, end := storage.GetLastNDays(7, now)
		startDate, endDate = storage.FormatDateRange(start, end)
	case last30Days:
		start, end := storage.GetLastNDays(30, now)
		startDate, endDate = storage.FormatDateRange(start, end)
	case cycleDay > 0:
		start, end := storage.GetBillingCycleDates(cycleDay, now)
		startDate, endDate = storage.FormatDateRange(start, end)
	case fromDate != "" && toDate != "":
		startDate, endDate = fromDate, toDate
	default:
		// Default: show realtime stats
		stats, err := c.GetRealtimeStats(port)
		if err != nil {
			return err
		}

		if outputJSON {
			return json.NewEncoder(os.Stdout).Encode(stats)
		}

		fmt.Printf("Port %d - Realtime Statistics\n", port)
		fmt.Printf("════════════════════════════════════════\n")
		fmt.Printf("  RX Bytes:    %s (%s/s)\n", formatBytes(stats.RxBytes), formatBytes(uint64(stats.RxRate)))
		fmt.Printf("  TX Bytes:    %s (%s/s)\n", formatBytes(stats.TxBytes), formatBytes(uint64(stats.TxRate)))
		fmt.Printf("  Total:       %s\n", formatBytes(stats.RxBytes+stats.TxBytes))
		fmt.Printf("  RX Packets:  %d\n", stats.RxPackets)
		fmt.Printf("  TX Packets:  %d\n", stats.TxPackets)
		fmt.Printf("  Connections: %d\n", stats.Connections)
		return nil
	}

	// Query historical stats
	stats, err := c.GetHistoricalStats(port, startDate, endDate)
	if err != nil {
		return err
	}

	if outputJSON {
		return json.NewEncoder(os.Stdout).Encode(stats)
	}

	fmt.Printf("Port %d - Historical Statistics\n", port)
	fmt.Printf("Period: %s to %s\n", startDate, endDate)
	fmt.Printf("════════════════════════════════════════\n")
	fmt.Printf("  Total RX:    %s\n", formatBytes(stats.TotalRx))
	fmt.Printf("  Total TX:    %s\n", formatBytes(stats.TotalTx))
	fmt.Printf("  Total:       %s\n", formatBytes(stats.TotalBytes))
	fmt.Printf("  Peak RX:     %s/s\n", formatBytes(stats.PeakRxRate))
	fmt.Printf("  Peak TX:     %s/s\n", formatBytes(stats.PeakTxRate))

	if len(stats.DailyStats) > 0 {
		fmt.Printf("\nDaily Breakdown:\n")
		fmt.Printf("  %-12s  %12s  %12s  %12s\n", "Date", "RX", "TX", "Total")
		fmt.Printf("  %-12s  %12s  %12s  %12s\n", "────────────", "────────────", "────────────", "────────────")
		for _, d := range stats.DailyStats {
			fmt.Printf("  %-12s  %12s  %12s  %12s\n",
				d.Date,
				formatBytes(d.RxBytes),
				formatBytes(d.TxBytes),
				formatBytes(d.RxBytes+d.TxBytes))
		}
	}

	return nil
}

func runStatus(cmd *cobra.Command, args []string) error {
	c, err := getClient()
	if err != nil {
		return err
	}
	defer c.Close()

	status, err := c.GetStatus()
	if err != nil {
		return err
	}

	if outputJSON {
		return json.NewEncoder(os.Stdout).Encode(status)
	}

	fmt.Printf("Daemon Status\n")
	fmt.Printf("════════════════════════════════════════\n")
	fmt.Printf("  Running:    %v\n", status.Running)
	fmt.Printf("  Uptime:     %s\n", status.Uptime)
	fmt.Printf("  Start Time: %s\n", status.StartTime)
	fmt.Printf("  Version:    %s\n", status.Version)
	fmt.Printf("  Data Dir:   %s\n", status.DataDir)
	fmt.Printf("  Retention:  %d days\n", status.RetentionDays)
	fmt.Printf("  Socket:     %s\n", status.SocketPath)
	fmt.Printf("  Ports:      %v\n", status.MonitoredPorts)

	return nil
}

func runListPorts(cmd *cobra.Command, args []string) error {
	c, err := getClient()
	if err != nil {
		return err
	}
	defer c.Close()

	ports, err := c.ListPorts()
	if err != nil {
		return err
	}

	if outputJSON {
		return json.NewEncoder(os.Stdout).Encode(map[string][]uint16{"ports": ports})
	}

	if len(ports) == 0 {
		fmt.Println("No ports being monitored")
		return nil
	}

	fmt.Println("Monitored ports:")
	for _, p := range ports {
		fmt.Printf("  - %d\n", p)
	}

	return nil
}

func runAddPort(cmd *cobra.Command, args []string) error {
	var portNum uint16
	if _, err := fmt.Sscanf(args[0], "%d", &portNum); err != nil {
		return fmt.Errorf("invalid port: %s", args[0])
	}

	c, err := getClient()
	if err != nil {
		return err
	}
	defer c.Close()

	if err := c.AddPort(portNum); err != nil {
		return err
	}

	fmt.Printf("Port %d added to monitoring\n", portNum)
	return nil
}

func runRemovePort(cmd *cobra.Command, args []string) error {
	var portNum uint16
	if _, err := fmt.Sscanf(args[0], "%d", &portNum); err != nil {
		return fmt.Errorf("invalid port: %s", args[0])
	}

	c, err := getClient()
	if err != nil {
		return err
	}
	defer c.Close()

	if err := c.RemovePort(portNum); err != nil {
		return err
	}

	fmt.Printf("Port %d removed from monitoring\n", portNum)
	return nil
}

func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
