// Package storage provides SQLite-based persistence for traffic statistics.
package storage

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps SQLite database operations.
type DB struct {
	db   *sql.DB
	path string
	mu   sync.Mutex
}

// schema defines the database tables.
const schema = `
-- Hourly aggregated statistics
CREATE TABLE IF NOT EXISTS hourly_stats (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    port INTEGER NOT NULL,
    timestamp INTEGER NOT NULL,  -- Unix timestamp (hour granularity)
    rx_bytes INTEGER DEFAULT 0,
    tx_bytes INTEGER DEFAULT 0,
    rx_packets INTEGER DEFAULT 0,
    tx_packets INTEGER DEFAULT 0,
    connections INTEGER DEFAULT 0,
    created_at INTEGER DEFAULT (strftime('%s', 'now')),
    UNIQUE(port, timestamp)
);

-- Daily aggregated statistics
CREATE TABLE IF NOT EXISTS daily_stats (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    port INTEGER NOT NULL,
    date TEXT NOT NULL,  -- YYYY-MM-DD format
    rx_bytes INTEGER DEFAULT 0,
    tx_bytes INTEGER DEFAULT 0,
    rx_packets INTEGER DEFAULT 0,
    tx_packets INTEGER DEFAULT 0,
    connections INTEGER DEFAULT 0,
    peak_rx_rate INTEGER DEFAULT 0,
    peak_tx_rate INTEGER DEFAULT 0,
    created_at INTEGER DEFAULT (strftime('%s', 'now')),
    UNIQUE(port, date)
);

-- Active connections (ephemeral, cleared on restart)
CREATE TABLE IF NOT EXISTS active_connections (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    port INTEGER NOT NULL,
    remote_addr TEXT NOT NULL,
    remote_port INTEGER NOT NULL,
    state TEXT NOT NULL,
    rx_bytes INTEGER DEFAULT 0,
    tx_bytes INTEGER DEFAULT 0,
    started_at INTEGER NOT NULL,
    last_seen INTEGER NOT NULL
);

-- Metadata
CREATE TABLE IF NOT EXISTS metadata (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_hourly_port_ts ON hourly_stats(port, timestamp);
CREATE INDEX IF NOT EXISTS idx_daily_port_date ON daily_stats(port, date);
CREATE INDEX IF NOT EXISTS idx_active_port ON active_connections(port);
`

// Open opens or creates the SQLite database.
func Open(dataDir string) (*DB, error) {
	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("creating data directory: %w", err)
	}

	dbPath := filepath.Join(dataDir, "data.db")

	// Open database with WAL mode for better concurrency
	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Apply schema
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("applying schema: %w", err)
	}

	// Clear ephemeral connections table on startup
	if _, err := db.Exec("DELETE FROM active_connections"); err != nil {
		slog.Warn("failed to clear active connections", "error", err)
	}

	slog.Info("database opened", "path", dbPath)

	return &DB{
		db:   db,
		path: dbPath,
	}, nil
}

// Close closes the database.
func (d *DB) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

// Path returns the database file path.
func (d *DB) Path() string {
	return d.path
}

// UpsertHourlyStats inserts or updates hourly statistics.
func (d *DB) UpsertHourlyStats(port uint16, ts time.Time, rxBytes, txBytes, rxPackets, txPackets, connections uint64) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Truncate to hour
	hourTs := ts.Truncate(time.Hour).Unix()

	_, err := d.db.Exec(`
		INSERT INTO hourly_stats (port, timestamp, rx_bytes, tx_bytes, rx_packets, tx_packets, connections)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(port, timestamp) DO UPDATE SET
			rx_bytes = rx_bytes + excluded.rx_bytes,
			tx_bytes = tx_bytes + excluded.tx_bytes,
			rx_packets = rx_packets + excluded.rx_packets,
			tx_packets = tx_packets + excluded.tx_packets,
			connections = connections + excluded.connections
	`, port, hourTs, rxBytes, txBytes, rxPackets, txPackets, connections)

	return err
}

// UpsertDailyStats inserts or updates daily statistics.
func (d *DB) UpsertDailyStats(port uint16, date string, rxBytes, txBytes, rxPackets, txPackets, connections uint64, peakRx, peakTx uint64) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(`
		INSERT INTO daily_stats (port, date, rx_bytes, tx_bytes, rx_packets, tx_packets, connections, peak_rx_rate, peak_tx_rate)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(port, date) DO UPDATE SET
			rx_bytes = rx_bytes + excluded.rx_bytes,
			tx_bytes = tx_bytes + excluded.tx_bytes,
			rx_packets = rx_packets + excluded.rx_packets,
			tx_packets = tx_packets + excluded.tx_packets,
			connections = connections + excluded.connections,
			peak_rx_rate = MAX(peak_rx_rate, excluded.peak_rx_rate),
			peak_tx_rate = MAX(peak_tx_rate, excluded.peak_tx_rate)
	`, port, date, rxBytes, txBytes, rxPackets, txPackets, connections, peakRx, peakTx)

	return err
}

// HourlyStatsRow represents a row from hourly_stats table.
type HourlyStatsRow struct {
	Port        uint16
	Timestamp   int64
	RxBytes     uint64
	TxBytes     uint64
	RxPackets   uint64
	TxPackets   uint64
	Connections uint64
}

// DailyStatsRow represents a row from daily_stats table.
type DailyStatsRow struct {
	Port        uint16
	Date        string
	RxBytes     uint64
	TxBytes     uint64
	RxPackets   uint64
	TxPackets   uint64
	Connections uint64
	PeakRxRate  uint64
	PeakTxRate  uint64
}

// QueryHourlyStats queries hourly stats for a port within a time range.
func (d *DB) QueryHourlyStats(port uint16, start, end time.Time) ([]HourlyStatsRow, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	rows, err := d.db.Query(`
		SELECT port, timestamp, rx_bytes, tx_bytes, rx_packets, tx_packets, connections
		FROM hourly_stats
		WHERE port = ? AND timestamp >= ? AND timestamp <= ?
		ORDER BY timestamp
	`, port, start.Unix(), end.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []HourlyStatsRow
	for rows.Next() {
		var r HourlyStatsRow
		if err := rows.Scan(&r.Port, &r.Timestamp, &r.RxBytes, &r.TxBytes, &r.RxPackets, &r.TxPackets, &r.Connections); err != nil {
			return nil, err
		}
		result = append(result, r)
	}

	return result, rows.Err()
}

// QueryDailyStats queries daily stats for a port within a date range.
func (d *DB) QueryDailyStats(port uint16, startDate, endDate string) ([]DailyStatsRow, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	rows, err := d.db.Query(`
		SELECT port, date, rx_bytes, tx_bytes, rx_packets, tx_packets, connections, peak_rx_rate, peak_tx_rate
		FROM daily_stats
		WHERE port = ? AND date >= ? AND date <= ?
		ORDER BY date
	`, port, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []DailyStatsRow
	for rows.Next() {
		var r DailyStatsRow
		if err := rows.Scan(&r.Port, &r.Date, &r.RxBytes, &r.TxBytes, &r.RxPackets, &r.TxPackets, &r.Connections, &r.PeakRxRate, &r.PeakTxRate); err != nil {
			return nil, err
		}
		result = append(result, r)
	}

	return result, rows.Err()
}

// GetPeriodSummary returns aggregated stats for a port over a date range.
func (d *DB) GetPeriodSummary(port uint16, startDate, endDate string) (*DailyStatsRow, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	var r DailyStatsRow
	r.Port = port

	err := d.db.QueryRow(`
		SELECT 
			COALESCE(SUM(rx_bytes), 0),
			COALESCE(SUM(tx_bytes), 0),
			COALESCE(SUM(rx_packets), 0),
			COALESCE(SUM(tx_packets), 0),
			COALESCE(SUM(connections), 0),
			COALESCE(MAX(peak_rx_rate), 0),
			COALESCE(MAX(peak_tx_rate), 0)
		FROM daily_stats
		WHERE port = ? AND date >= ? AND date <= ?
	`, port, startDate, endDate).Scan(
		&r.RxBytes, &r.TxBytes, &r.RxPackets, &r.TxPackets,
		&r.Connections, &r.PeakRxRate, &r.PeakTxRate,
	)
	if err != nil {
		return nil, err
	}

	r.Date = startDate + " to " + endDate
	return &r, nil
}

// DeleteOldData removes data older than the specified number of days.
func (d *DB) DeleteOldData(retentionDays int) (int64, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	cutoffTs := cutoff.Unix()
	cutoffDate := cutoff.Format("2006-01-02")

	var totalDeleted int64

	// Delete from hourly_stats
	result, err := d.db.Exec("DELETE FROM hourly_stats WHERE timestamp < ?", cutoffTs)
	if err != nil {
		return 0, fmt.Errorf("deleting old hourly stats: %w", err)
	}
	n, _ := result.RowsAffected()
	totalDeleted += n

	// Delete from daily_stats
	result, err = d.db.Exec("DELETE FROM daily_stats WHERE date < ?", cutoffDate)
	if err != nil {
		return 0, fmt.Errorf("deleting old daily stats: %w", err)
	}
	n, _ = result.RowsAffected()
	totalDeleted += n

	if totalDeleted > 0 {
		slog.Info("cleaned up old data", "deleted_rows", totalDeleted, "retention_days", retentionDays)
	}

	return totalDeleted, nil
}

// GetMetadata retrieves a metadata value.
func (d *DB) GetMetadata(key string) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	var value string
	err := d.db.QueryRow("SELECT value FROM metadata WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// SetMetadata sets a metadata value.
func (d *DB) SetMetadata(key, value string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(`
		INSERT INTO metadata (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, key, value)
	return err
}
