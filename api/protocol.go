// Package api defines the IPC protocol for daemon-client communication.
package api

import (
	"encoding/json"
	"time"
)

// Request is a JSON-RPC style request.
type Request struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
	ID     int             `json:"id"`
}

// Response is a JSON-RPC style response.
type Response struct {
	Result json.RawMessage `json:"result,omitempty"`
	Error  *Error          `json:"error,omitempty"`
	ID     int             `json:"id"`
}

// Error represents an RPC error.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Error codes
const (
	ErrCodeInvalidRequest = -32600
	ErrCodeMethodNotFound = -32601
	ErrCodeInvalidParams  = -32602
	ErrCodeInternal       = -32603
	ErrCodeNotFound       = -32001
)

// Method names
const (
	MethodGetRealtimeStats     = "get_realtime_stats"
	MethodGetHistoricalStats   = "get_historical_stats"
	MethodGetActiveConnections = "get_active_connections"
	MethodGetStatus            = "get_status"
	MethodAddPort              = "add_port"
	MethodRemovePort           = "remove_port"
	MethodListPorts            = "list_ports"
	MethodFlushStats           = "flush_stats"
)

// ========== Request Parameters ==========

// PortParams is used for single-port operations.
type PortParams struct {
	Port uint16 `json:"port"`
}

// HistoricalParams is used for historical data queries.
type HistoricalParams struct {
	Port      uint16 `json:"port"`
	StartDate string `json:"start_date"` // YYYY-MM-DD
	EndDate   string `json:"end_date"`   // YYYY-MM-DD
}

// ========== Response Types ==========

// RealtimeStatsResult contains current stats and rates.
type RealtimeStatsResult struct {
	Port        uint16  `json:"port"`
	RxBytes     uint64  `json:"rx_bytes"`
	TxBytes     uint64  `json:"tx_bytes"`
	RxPackets   uint64  `json:"rx_packets"`
	TxPackets   uint64  `json:"tx_packets"`
	Connections uint64  `json:"connections"`
	RxRate      float64 `json:"rx_rate"` // bytes/sec
	TxRate      float64 `json:"tx_rate"`
}

// HistoricalStatsResult contains aggregated historical data.
type HistoricalStatsResult struct {
	Port       uint16     `json:"port"`
	StartDate  string     `json:"start_date"`
	EndDate    string     `json:"end_date"`
	TotalRx    uint64     `json:"total_rx"`
	TotalTx    uint64     `json:"total_tx"`
	TotalBytes uint64     `json:"total_bytes"`
	PeakRxRate uint64     `json:"peak_rx_rate"`
	PeakTxRate uint64     `json:"peak_tx_rate"`
	DailyStats []DayStats `json:"daily_stats,omitempty"`
}

// DayStats represents a single day's statistics.
type DayStats struct {
	Date        string `json:"date"`
	RxBytes     uint64 `json:"rx_bytes"`
	TxBytes     uint64 `json:"tx_bytes"`
	RxPackets   uint64 `json:"rx_packets"`
	TxPackets   uint64 `json:"tx_packets"`
	Connections uint64 `json:"connections"`
	PeakRxRate  uint64 `json:"peak_rx_rate"`
	PeakTxRate  uint64 `json:"peak_tx_rate"`
}

// ConnectionInfo represents an active connection.
type ConnectionInfo struct {
	RemoteAddr string    `json:"remote_addr"`
	RemotePort uint16    `json:"remote_port"`
	RxBytes    uint64    `json:"rx_bytes"`
	TxBytes    uint64    `json:"tx_bytes"`
	StartedAt  time.Time `json:"started_at"`
	Duration   string    `json:"duration"`
}

// ActiveConnectionsResult contains the list of active connections.
type ActiveConnectionsResult struct {
	Port        uint16           `json:"port"`
	Connections []ConnectionInfo `json:"connections"`
	Count       int              `json:"count"`
}

// PortInfo contains port number and description.
type PortInfo struct {
	Port        uint16 `json:"port"`
	Description string `json:"description"`
}

// StatusResult contains daemon status information.
type StatusResult struct {
	Running        bool       `json:"running"`
	Uptime         string     `json:"uptime"`
	StartTime      string     `json:"start_time"`
	MonitoredPorts []uint16   `json:"monitored_ports"`
	PortInfos      []PortInfo `json:"port_infos"`
	DataDir        string     `json:"data_dir"`
	RetentionDays  int        `json:"retention_days"`
	SocketPath     string     `json:"socket_path"`
	Version        string     `json:"version"`
}

// ListPortsResult contains the list of monitored ports.
type ListPortsResult struct {
	Ports []uint16 `json:"ports"`
}

// SuccessResult indicates a successful operation.
type SuccessResult struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}
