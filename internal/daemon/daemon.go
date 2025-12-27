package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/wellsgz/portmon/internal/ebpf"
	"github.com/wellsgz/portmon/internal/storage"
)

// Daemon orchestrates all daemon components.
type Daemon struct {
	config     *Config
	loader     *ebpf.Loader
	collector  *ebpf.Collector
	aggregator *Aggregator
	server     *Server
	db         *storage.DB
}

// New creates a new daemon instance.
func New(config *Config) *Daemon {
	return &Daemon{
		config: config,
	}
}

// Run starts the daemon and blocks until shutdown.
func (d *Daemon) Run() error {
	ctx, cancel := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Expand data dir
	dataDir := d.config.DataDir
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, ".portmon")
	} else if dataDir[0] == '~' {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, dataDir[1:])
	}
	d.config.DataDir = dataDir

	// Expand socket path
	socketPath := d.config.SocketPath
	if socketPath == "" {
		socketPath = filepath.Join(dataDir, "portmon.sock")
	}
	d.config.SocketPath = socketPath

	slog.Info("starting portmon daemon",
		"data_dir", dataDir,
		"socket", socketPath,
		"ports", d.config.Ports,
		"retention_days", d.config.RetentionDays)

	// Initialize database
	db, err := storage.Open(dataDir)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	d.db = db
	defer db.Close()

	// Load eBPF programs
	loader := ebpf.NewLoader()
	d.loader = loader

	if err := loader.Load(); err != nil {
		return fmt.Errorf("loading eBPF programs: %w", err)
	}
	defer loader.Close()

	// Attach kprobes
	if err := loader.Attach(); err != nil {
		return fmt.Errorf("attaching kprobes: %w", err)
	}

	// Add target ports
	for _, port := range d.config.Ports {
		if err := loader.AddPort(port); err != nil {
			slog.Warn("failed to add port", "port", port, "error", err)
		}
	}

	// Start stats collector
	collector := ebpf.NewCollector(loader, 100*time.Millisecond)
	d.collector = collector
	go collector.Run(ctx)

	// Start aggregator (persists to DB)
	aggregator := NewAggregator(collector, db, 60*time.Second)
	d.aggregator = aggregator
	go aggregator.Run(ctx)

	// Start retention cleanup job
	go d.runRetentionCleanup(ctx)

	// Start IPC server
	server := NewServer(socketPath, loader, collector, db, d.config)
	d.server = server

	go func() {
		if err := server.Serve(ctx); err != nil {
			slog.Error("IPC server error", "error", err)
		}
	}()

	slog.Info("daemon started successfully")

	// Wait for shutdown signal
	<-ctx.Done()
	slog.Info("shutting down daemon...")

	// Cleanup
	server.Close()

	return nil
}

// runRetentionCleanup runs daily cleanup of old data.
func (d *Daemon) runRetentionCleanup(ctx context.Context) {
	// Run once at startup
	d.db.DeleteOldData(d.config.RetentionDays)

	// Then run daily
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.db.DeleteOldData(d.config.RetentionDays)
		}
	}
}
