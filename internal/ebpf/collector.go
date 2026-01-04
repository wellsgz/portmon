package ebpf

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/wellsgz/portmon/internal/types"
)

// Collector polls eBPF maps and calculates traffic rates.
type Collector struct {
	loader       *Loader
	pollInterval time.Duration

	mu        sync.RWMutex
	lastStats map[uint16]*probePmPortStats
	lastTime  time.Time
	rates     map[uint16]*types.PortStats
}

// NewCollector creates a new stats collector.
func NewCollector(loader *Loader, pollInterval time.Duration) *Collector {
	return &Collector{
		loader:       loader,
		pollInterval: pollInterval,
		lastStats:    make(map[uint16]*probePmPortStats),
		rates:        make(map[uint16]*types.PortStats),
	}
}

// Run starts the collector loop. It polls BPF maps at the configured interval.
func (c *Collector) Run(ctx context.Context) {
	ticker := time.NewTicker(c.pollInterval)
	defer ticker.Stop()

	slog.Info("stats collector started", "interval", c.pollInterval)

	for {
		select {
		case <-ctx.Done():
			slog.Info("stats collector stopped")
			return
		case <-ticker.C:
			c.collect()
		}
	}
}

// collect reads stats from BPF maps and calculates rates.
func (c *Collector) collect() {
	stats, err := c.loader.GetAllPortStats()
	if err != nil {
		slog.Error("failed to get port stats", "error", err)
		return
	}

	// Get actual active connection counts
	activeConns, err := c.loader.CountActiveConnections()
	if err != nil {
		slog.Debug("failed to count active connections", "error", err)
		activeConns = make(map[uint16]uint64)
	}

	now := time.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Calculate time delta
	var elapsed float64
	if !c.lastTime.IsZero() {
		elapsed = now.Sub(c.lastTime).Seconds()
	}

	for port, current := range stats {
		portStats := &types.PortStats{
			Port:        port,
			RxBytes:     current.RxBytes,
			TxBytes:     current.TxBytes,
			RxPackets:   current.RxPackets,
			TxPackets:   current.TxPackets,
			Connections: activeConns[port], // Use actual active count
		}

		// Calculate rates if we have previous data
		if elapsed > 0 {
			if prev, ok := c.lastStats[port]; ok {
				rxDelta := current.RxBytes - prev.RxBytes
				txDelta := current.TxBytes - prev.TxBytes
				portStats.RxRate = float64(rxDelta) / elapsed
				portStats.TxRate = float64(txDelta) / elapsed
			}
		}

		c.rates[port] = portStats
		statsCopy := *current
		c.lastStats[port] = &statsCopy
	}

	c.lastTime = now
}

// GetStats returns the current stats and rates for a port.
func (c *Collector) GetStats(port uint16) *types.PortStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if stats, ok := c.rates[port]; ok {
		return stats
	}

	return &types.PortStats{Port: port}
}

// GetAllStats returns stats for all monitored ports.
func (c *Collector) GetAllStats() map[uint16]*types.PortStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[uint16]*types.PortStats, len(c.rates))
	for port, stats := range c.rates {
		statsCopy := *stats
		result[port] = &statsCopy
	}
	return result
}

// GetRawStats returns the raw eBPF stats for persistence.
func (c *Collector) GetRawStats() map[uint16]*probePmPortStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[uint16]*probePmPortStats, len(c.lastStats))
	for port, stats := range c.lastStats {
		statsCopy := *stats
		result[port] = &statsCopy
	}
	return result
}
