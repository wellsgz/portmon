// portmond is the eBPF port traffic monitor daemon.
package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/wellsgz/portmon/internal/config"
	"github.com/wellsgz/portmon/internal/daemon"
)

var (
	configPath    string
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

	rootCmd.Flags().StringVarP(&configPath, "config", "c", "", "Config file path (default: /etc/portmon/portmon.yaml)")
	rootCmd.Flags().IntSliceVarP(&ports, "port", "p", nil, "Ports to monitor (can be specified multiple times)")
	rootCmd.Flags().StringVar(&dataDir, "data-dir", "", "Data directory (default: /var/lib/portmon)")
	rootCmd.Flags().IntVar(&retentionDays, "retention-days", 0, "Data retention in days (1-365)")
	rootCmd.Flags().StringVar(&socketPath, "socket", "", "Unix socket path (default: /run/portmon/portmon.sock)")
	rootCmd.Flags().StringVar(&logLevel, "log-level", "", "Log level (debug, info, warn, error)")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runDaemon(cmd *cobra.Command, args []string) error {
	// Start with defaults
	cfg := config.Defaults()

	// Try to load config file
	cfgPath := configPath
	if cfgPath == "" {
		cfgPath = config.DefaultConfigPath
	}

	if fileCfg, err := config.Load(cfgPath); err == nil {
		slog.Info("loaded config file", "path", cfgPath)
		cfg = fileCfg
	} else if configPath != "" {
		// Only error if user explicitly specified a config file
		return fmt.Errorf("loading config file %s: %w", configPath, err)
	}

	// CLI flags override config file
	if len(ports) > 0 {
		cfg.Ports = ports
	}
	if dataDir != "" {
		cfg.DataDir = dataDir
	}
	if retentionDays > 0 {
		cfg.RetentionDays = retentionDays
	}
	if socketPath != "" {
		cfg.Socket = socketPath
	}
	if logLevel != "" {
		cfg.LogLevel = logLevel
	}

	// Set default for log level if still empty
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}

	// Set default for retention days if still 0
	if cfg.RetentionDays == 0 {
		cfg.RetentionDays = 180
	}

	// Configure logging
	var level slog.Level
	switch cfg.LogLevel {
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
	if len(cfg.Ports) == 0 {
		return fmt.Errorf("at least one port must be specified (via --port or config file)")
	}

	// Convert and validate ports
	portList := make([]uint16, 0, len(cfg.Ports))
	for _, p := range cfg.Ports {
		if p < 1 || p > 65535 {
			return fmt.Errorf("invalid port %d: must be between 1 and 65535", p)
		}
		portList = append(portList, uint16(p))
	}

	if cfg.RetentionDays < 1 || cfg.RetentionDays > 365 {
		return fmt.Errorf("retention_days must be between 1 and 365")
	}

	// Check for root privileges (required for eBPF)
	if os.Geteuid() != 0 {
		slog.Warn("running without root privileges, eBPF loading may fail")
	}

	daemonCfg := &daemon.Config{
		Ports:         portList,
		DataDir:       cfg.DataDir,
		RetentionDays: cfg.RetentionDays,
		SocketPath:    cfg.Socket,
		LogLevel:      cfg.LogLevel,
	}

	d := daemon.New(daemonCfg)
	return d.Run()
}
