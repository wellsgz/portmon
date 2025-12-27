// Package types defines shared data types used across the portmon application.
package types

import (
	"net"
	"time"
)

// PortStats holds real-time statistics for a monitored port.
type PortStats struct {
	Port        uint16 `json:"port"`
	RxBytes     uint64 `json:"rx_bytes"`
	TxBytes     uint64 `json:"tx_bytes"`
	RxPackets   uint64 `json:"rx_packets"`
	TxPackets   uint64 `json:"tx_packets"`
	Connections uint64 `json:"connections"`
	// Calculated rates (bytes/sec)
	RxRate float64 `json:"rx_rate"`
	TxRate float64 `json:"tx_rate"`
}

// HourlyStats represents hourly aggregated traffic data.
type HourlyStats struct {
	ID          int64     `json:"id,omitempty"`
	Port        uint16    `json:"port"`
	Timestamp   time.Time `json:"timestamp"` // Hour granularity (truncated)
	RxBytes     uint64    `json:"rx_bytes"`
	TxBytes     uint64    `json:"tx_bytes"`
	RxPackets   uint64    `json:"rx_packets"`
	TxPackets   uint64    `json:"tx_packets"`
	Connections uint64    `json:"connections"`
}

// DailyStats represents daily aggregated traffic data.
type DailyStats struct {
	ID          int64  `json:"id,omitempty"`
	Port        uint16 `json:"port"`
	Date        string `json:"date"` // YYYY-MM-DD format
	RxBytes     uint64 `json:"rx_bytes"`
	TxBytes     uint64 `json:"tx_bytes"`
	RxPackets   uint64 `json:"rx_packets"`
	TxPackets   uint64 `json:"tx_packets"`
	Connections uint64 `json:"connections"`
	PeakRxRate  uint64 `json:"peak_rx_rate"` // Peak bytes/sec
	PeakTxRate  uint64 `json:"peak_tx_rate"`
}

// ActiveConnection represents a currently active TCP connection.
type ActiveConnection struct {
	ID         int64     `json:"id,omitempty"`
	Port       uint16    `json:"port"`
	RemoteAddr net.IP    `json:"remote_addr"`
	RemotePort uint16    `json:"remote_port"`
	State      string    `json:"state"`
	RxBytes    uint64    `json:"rx_bytes"`
	TxBytes    uint64    `json:"tx_bytes"`
	StartedAt  time.Time `json:"started_at"`
	LastSeen   time.Time `json:"last_seen"`
}

// PeriodSummary holds aggregated stats for a time period.
type PeriodSummary struct {
	Port       uint16    `json:"port"`
	StartDate  time.Time `json:"start_date"`
	EndDate    time.Time `json:"end_date"`
	RxBytes    uint64    `json:"rx_bytes"`
	TxBytes    uint64    `json:"tx_bytes"`
	TotalBytes uint64    `json:"total_bytes"`
	RxPackets  uint64    `json:"rx_packets"`
	TxPackets  uint64    `json:"tx_packets"`
	AvgRxRate  float64   `json:"avg_rx_rate"` // bytes/sec
	AvgTxRate  float64   `json:"avg_tx_rate"`
	PeakRxRate float64   `json:"peak_rx_rate"`
	PeakTxRate float64   `json:"peak_tx_rate"`
}

// DaemonStatus holds daemon health and configuration info.
type DaemonStatus struct {
	Running        bool      `json:"running"`
	Uptime         string    `json:"uptime"`
	StartTime      time.Time `json:"start_time"`
	MonitoredPorts []uint16  `json:"monitored_ports"`
	DataDir        string    `json:"data_dir"`
	RetentionDays  int       `json:"retention_days"`
	SocketPath     string    `json:"socket_path"`
	Version        string    `json:"version"`
}

// ConnKey identifies a unique TCP connection.
type ConnKey struct {
	SrcAddr uint32
	DstAddr uint32
	SrcPort uint16
	DstPort uint16
}

// ConnStats holds per-connection statistics from eBPF.
type ConnStats struct {
	RxBytes      uint64
	TxBytes      uint64
	StartNs      uint64
	LastUpdateNs uint64
}
