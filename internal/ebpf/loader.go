// Package ebpf handles loading and managing eBPF programs for traffic monitoring.
package ebpf

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang -target amd64 -type pm_port_stats -type pm_conn_key -type pm_conn_stats probe ./bpf/probe.c -- -I./bpf -O2 -g -Wall

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
)

// Loader handles loading and attaching eBPF programs.
type Loader struct {
	objs  *probeObjects
	links []link.Link
	mu    sync.Mutex
}

// NewLoader creates a new eBPF loader.
func NewLoader() *Loader {
	return &Loader{}
}

// Load loads the eBPF programs and maps.
func (l *Loader) Load() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.objs != nil {
		return errors.New("eBPF programs already loaded")
	}

	objs := &probeObjects{}
	if err := loadProbeObjects(objs, nil); err != nil {
		return fmt.Errorf("loading eBPF objects: %w", err)
	}

	l.objs = objs
	slog.Info("eBPF programs loaded successfully")
	return nil
}

// Attach attaches the kprobes to the kernel.
func (l *Loader) Attach() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.objs == nil {
		return errors.New("eBPF programs not loaded")
	}

	// Attach kprobe for tcp_sendmsg
	sendmsgLink, err := link.Kprobe("tcp_sendmsg", l.objs.TraceTcpSendmsg, nil)
	if err != nil {
		return fmt.Errorf("attaching tcp_sendmsg kprobe: %w", err)
	}
	l.links = append(l.links, sendmsgLink)
	slog.Info("attached kprobe", "function", "tcp_sendmsg")

	// Attach kprobe for tcp_cleanup_rbuf
	cleanupLink, err := link.Kprobe("tcp_cleanup_rbuf", l.objs.TraceTcpCleanupRbuf, nil)
	if err != nil {
		return fmt.Errorf("attaching tcp_cleanup_rbuf kprobe: %w", err)
	}
	l.links = append(l.links, cleanupLink)
	slog.Info("attached kprobe", "function", "tcp_cleanup_rbuf")

	return nil
}

// AddPort adds a port to the monitoring list.
func (l *Loader) AddPort(port uint16) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.objs == nil {
		return errors.New("eBPF programs not loaded")
	}

	enabled := uint8(1)
	if err := l.objs.TargetPorts.Put(port, enabled); err != nil {
		return fmt.Errorf("adding port %d to target_ports map: %w", port, err)
	}

	slog.Info("added port to monitoring", "port", port)
	return nil
}

// RemovePort removes a port from the monitoring list.
func (l *Loader) RemovePort(port uint16) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.objs == nil {
		return errors.New("eBPF programs not loaded")
	}

	if err := l.objs.TargetPorts.Delete(port); err != nil {
		if errors.Is(err, ebpf.ErrKeyNotExist) {
			return nil // Port wasn't being monitored
		}
		return fmt.Errorf("removing port %d from target_ports map: %w", port, err)
	}

	slog.Info("removed port from monitoring", "port", port)
	return nil
}

// GetPortStats retrieves current statistics for a specific port.
func (l *Loader) GetPortStats(port uint16) (*probePmPortStats, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.objs == nil {
		return nil, errors.New("eBPF programs not loaded")
	}

	var stats probePmPortStats
	if err := l.objs.PortStatsMap.Lookup(port, &stats); err != nil {
		if errors.Is(err, ebpf.ErrKeyNotExist) {
			return &probePmPortStats{}, nil // No stats yet
		}
		return nil, fmt.Errorf("looking up port stats: %w", err)
	}

	return &stats, nil
}

// GetAllPortStats retrieves statistics for all monitored ports.
func (l *Loader) GetAllPortStats() (map[uint16]*probePmPortStats, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.objs == nil {
		return nil, errors.New("eBPF programs not loaded")
	}

	result := make(map[uint16]*probePmPortStats)

	var port uint16
	var stats probePmPortStats
	iter := l.objs.PortStatsMap.Iterate()
	for iter.Next(&port, &stats) {
		statsCopy := stats
		result[port] = &statsCopy
	}

	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("iterating port stats: %w", err)
	}

	return result, nil
}

// GetConnectionStats retrieves all per-connection statistics.
func (l *Loader) GetConnectionStats() (map[probePmConnKey]*probePmConnStats, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.objs == nil {
		return nil, errors.New("eBPF programs not loaded")
	}

	result := make(map[probePmConnKey]*probePmConnStats)

	var key probePmConnKey
	var stats probePmConnStats
	iter := l.objs.ConnStatsMap.Iterate()
	for iter.Next(&key, &stats) {
		statsCopy := stats
		result[key] = &statsCopy
	}

	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("iterating connection stats: %w", err)
	}

	return result, nil
}

// ClearPortStats resets statistics for a port.
func (l *Loader) ClearPortStats(port uint16) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.objs == nil {
		return errors.New("eBPF programs not loaded")
	}

	zeroStats := probePmPortStats{}
	if err := l.objs.PortStatsMap.Put(port, zeroStats); err != nil {
		return fmt.Errorf("clearing port stats: %w", err)
	}

	return nil
}

// CountActiveConnections counts actual entries in conn_stats_map per port.
// This gives accurate active connection counts instead of cumulative totals.
func (l *Loader) CountActiveConnections() (map[uint16]uint64, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.objs == nil {
		return nil, errors.New("eBPF programs not loaded")
	}

	counts := make(map[uint16]uint64)

	var key probePmConnKey
	var stats probePmConnStats
	iter := l.objs.ConnStatsMap.Iterate()
	for iter.Next(&key, &stats) {
		// Count connections where sport OR dport matches a monitored port
		// We check which port is monitored by looking at both
		var enabled uint8

		// Check if sport is monitored
		if err := l.objs.TargetPorts.Lookup(key.Sport, &enabled); err == nil && enabled == 1 {
			counts[key.Sport]++
		}
		// Check if dport is monitored
		if err := l.objs.TargetPorts.Lookup(key.Dport, &enabled); err == nil && enabled == 1 {
			counts[key.Dport]++
		}
	}

	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("iterating connection stats: %w", err)
	}

	return counts, nil
}

// Close releases all eBPF resources.
func (l *Loader) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	var errs []error

	// Detach kprobes
	for _, lnk := range l.links {
		if err := lnk.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	l.links = nil

	// Close eBPF objects
	if l.objs != nil {
		if err := l.objs.Close(); err != nil {
			errs = append(errs, err)
		}
		l.objs = nil
	}

	slog.Info("eBPF resources released")

	if len(errs) > 0 {
		return fmt.Errorf("errors closing eBPF resources: %v", errs)
	}
	return nil
}

// IsLoaded returns true if eBPF programs are loaded.
func (l *Loader) IsLoaded() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.objs != nil
}
