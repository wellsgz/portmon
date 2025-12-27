// portmond is the eBPF port traffic monitor daemon.
package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/wellsgz/portmon/internal/daemon"
	"github.com/spf13/cobra"
)

var (
	ports         []int
	dataDir       string
	retentionDays int
	socketPath    string
	logLevel      string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "portmond",
		Short: "eBPF port traffic monitor daemon",
		Long: `portmond is a daemon that uses eBPF kprobes to monitor TCP traffic
on specified ports. It collects statistics, persists them to SQLite,
and exposes an IPC interface for clients.`,
		RunE: runDaemon,
	}

	rootCmd.Flags().IntSliceVarP(&ports, "port", "p", nil, "Ports to monitor (can be specified multiple times)")
	rootCmd.Flags().StringVar(&dataDir, "data-dir", "", "Data directory (default: ~/.portmon)")
	rootCmd.Flags().IntVar(&retentionDays, "retention-days", 180, "Data retention in days (1-365)")
	rootCmd.Flags().StringVar(&socketPath, "socket", "", "Unix socket path (default: ~/.portmon/portmon.sock)")
	rootCmd.Flags().StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error)")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runDaemon(cmd *cobra.Command, args []string) error {
	// Configure logging
	var level slog.Level
	switch logLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(handler))

	// Validate inputs
	if len(ports) == 0 {
		return fmt.Errorf("at least one --port must be specified")
	}

	// Convert and validate ports
	portList := make([]uint16, 0, len(ports))
	for _, p := range ports {
		if p < 1 || p > 65535 {
			return fmt.Errorf("invalid port %d: must be between 1 and 65535", p)
		}
		portList = append(portList, uint16(p))
	}

	if retentionDays < 1 || retentionDays > 365 {
		return fmt.Errorf("retention-days must be between 1 and 365")
	}

	// Check for root privileges (required for eBPF)
	if os.Geteuid() != 0 {
		slog.Warn("running without root privileges, eBPF loading may fail")
	}

	config := &daemon.Config{
		Ports:         portList,
		DataDir:       dataDir,
		RetentionDays: retentionDays,
		SocketPath:    socketPath,
		LogLevel:      logLevel,
	}

	d := daemon.New(config)
	return d.Run()
}
