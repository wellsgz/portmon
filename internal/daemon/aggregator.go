// Package daemon implements the portmond daemon logic.
package daemon

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/wellsgz/portmon/internal/ebpf"
	"github.com/wellsgz/portmon/internal/storage"
)

// Aggregator collects stats from eBPF and persists to database.
type Aggregator struct {
	collector       *ebpf.Collector
	db              *storage.DB
	persistInterval time.Duration

	mu          sync.RWMutex
	lastPersist map[uint16]*persistedStats
	peakRates   map[uint16]*peakRateTracker
}

type persistedStats struct {
	rxBytes     uint64
	txBytes     uint64
	rxPackets   uint64
	txPackets   uint64
	connections uint64
}

type peakRateTracker struct {
	date       string
	peakRxRate uint64
	peakTxRate uint64
}

// NewAggregator creates a new stats aggregator.
func NewAggregator(collector *ebpf.Collector, db *storage.DB, persistInterval time.Duration) *Aggregator {
	return &Aggregator{
		collector:       collector,
		db:              db,
		persistInterval: persistInterval,
		lastPersist:     make(map[uint16]*persistedStats),
		peakRates:       make(map[uint16]*peakRateTracker),
	}
}

// Run starts the aggregator loop.
func (a *Aggregator) Run(ctx context.Context) {
	ticker := time.NewTicker(a.persistInterval)
	defer ticker.Stop()

	slog.Info("aggregator started", "interval", a.persistInterval)

	for {
		select {
		case <-ctx.Done():
			// Final persist before shutdown
			a.persist()
			slog.Info("aggregator stopped")
			return
		case <-ticker.C:
			a.persist()
		}
	}
}

// Flush immediately persists current stats to database.
func (a *Aggregator) Flush() {
	a.persist()
	slog.Debug("aggregator flushed on demand")
}

// persist writes accumulated stats to the database.
func (a *Aggregator) persist() {
	allStats := a.collector.GetAllStats()
	now := time.Now()
	today := now.Format("2006-01-02")

	a.mu.Lock()
	defer a.mu.Unlock()

	for port, stats := range allStats {
		// Calculate deltas since last persist
		var deltaRx, deltaTx, deltaRxPkt, deltaTxPkt, deltaConn uint64

		if last, ok := a.lastPersist[port]; ok {
			if stats.RxBytes >= last.rxBytes {
				deltaRx = stats.RxBytes - last.rxBytes
			}
			if stats.TxBytes >= last.txBytes {
				deltaTx = stats.TxBytes - last.txBytes
			}
			if stats.RxPackets >= last.rxPackets {
				deltaRxPkt = stats.RxPackets - last.rxPackets
			}
			if stats.TxPackets >= last.txPackets {
				deltaTxPkt = stats.TxPackets - last.txPackets
			}
			if stats.Connections >= last.connections {
				deltaConn = stats.Connections - last.connections
			}
		} else {
			// First persist for this port
			deltaRx = stats.RxBytes
			deltaTx = stats.TxBytes
			deltaRxPkt = stats.RxPackets
			deltaTxPkt = stats.TxPackets
			deltaConn = stats.Connections
		}

		// Skip if no change
		if deltaRx == 0 && deltaTx == 0 {
			continue
		}

		// Update hourly stats
		if err := a.db.UpsertHourlyStats(port, now, deltaRx, deltaTx, deltaRxPkt, deltaTxPkt, deltaConn); err != nil {
			slog.Error("failed to upsert hourly stats", "port", port, "error", err)
		}

		// Track peak rates
		peak := a.peakRates[port]
		if peak == nil || peak.date != today {
			peak = &peakRateTracker{date: today}
			a.peakRates[port] = peak
		}

		// Update peak rates
		rxRate := uint64(stats.RxRate)
		txRate := uint64(stats.TxRate)
		if rxRate > peak.peakRxRate {
			peak.peakRxRate = rxRate
		}
		if txRate > peak.peakTxRate {
			peak.peakTxRate = txRate
		}

		// Update daily stats
		if err := a.db.UpsertDailyStats(port, today, deltaRx, deltaTx, deltaRxPkt, deltaTxPkt, deltaConn, peak.peakRxRate, peak.peakTxRate); err != nil {
			slog.Error("failed to upsert daily stats", "port", port, "error", err)
		}

		// Update last persisted values
		a.lastPersist[port] = &persistedStats{
			rxBytes:     stats.RxBytes,
			txBytes:     stats.TxBytes,
			rxPackets:   stats.RxPackets,
			txPackets:   stats.TxPackets,
			connections: stats.Connections,
		}

		slog.Debug("persisted stats", "port", port,
			"delta_rx", deltaRx, "delta_tx", deltaTx)
	}
}

// GetRealtimeStats returns current realtime stats for a port.
func (a *Aggregator) GetRealtimeStats(port uint16) *ebpf.Collector {
	return a.collector
}
