package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync"
	"time"

	"github.com/wellsgz/portmon/api"
	"github.com/wellsgz/portmon/internal/ebpf"
	"github.com/wellsgz/portmon/internal/storage"
)

// Server handles IPC requests from clients.
type Server struct {
	socketPath string
	listener   net.Listener
	loader     *ebpf.Loader
	collector  *ebpf.Collector
	db         *storage.DB
	config     *Config
	startTime  time.Time

	mu      sync.Mutex
	clients map[net.Conn]struct{}
}

// Config holds daemon configuration.
type Config struct {
	Ports         []uint16
	DataDir       string
	RetentionDays int
	SocketPath    string
	LogLevel      string
}

// NewServer creates a new IPC server.
func NewServer(socketPath string, loader *ebpf.Loader, collector *ebpf.Collector, db *storage.DB, config *Config) *Server {
	return &Server{
		socketPath: socketPath,
		loader:     loader,
		collector:  collector,
		db:         db,
		config:     config,
		startTime:  time.Now(),
		clients:    make(map[net.Conn]struct{}),
	}
}

// Serve starts the IPC server.
func (s *Server) Serve(ctx context.Context) error {
	// Remove existing socket file
	os.Remove(s.socketPath)

	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("creating unix socket: %w", err)
	}
	s.listener = listener

	// Make socket accessible to non-root users
	if err := os.Chmod(s.socketPath, 0666); err != nil {
		slog.Warn("failed to chmod socket", "error", err)
	}

	slog.Info("IPC server started", "socket", s.socketPath)

	go func() {
		<-ctx.Done()
		s.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			slog.Error("accept error", "error", err)
			continue
		}

		s.mu.Lock()
		s.clients[conn] = struct{}{}
		s.mu.Unlock()

		go s.handleClient(conn)
	}
}

// Close shuts down the server.
func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for conn := range s.clients {
		conn.Close()
	}
	s.clients = nil

	if s.listener != nil {
		s.listener.Close()
	}

	os.Remove(s.socketPath)
	slog.Info("IPC server stopped")
	return nil
}

func (s *Server) handleClient(conn net.Conn) {
	defer func() {
		conn.Close()
		s.mu.Lock()
		delete(s.clients, conn)
		s.mu.Unlock()
	}()

	scanner := bufio.NewScanner(conn)
	encoder := json.NewEncoder(conn)

	for scanner.Scan() {
		line := scanner.Bytes()

		var req api.Request
		if err := json.Unmarshal(line, &req); err != nil {
			s.sendError(encoder, 0, api.ErrCodeInvalidRequest, "invalid JSON")
			continue
		}

		resp := s.handleRequest(&req)
		if err := encoder.Encode(resp); err != nil {
			slog.Error("failed to send response", "error", err)
			return
		}
	}
}

func (s *Server) handleRequest(req *api.Request) *api.Response {
	switch req.Method {
	case api.MethodGetRealtimeStats:
		return s.handleGetRealtimeStats(req)
	case api.MethodGetHistoricalStats:
		return s.handleGetHistoricalStats(req)
	case api.MethodGetStatus:
		return s.handleGetStatus(req)
	case api.MethodAddPort:
		return s.handleAddPort(req)
	case api.MethodRemovePort:
		return s.handleRemovePort(req)
	case api.MethodListPorts:
		return s.handleListPorts(req)
	default:
		return s.errorResponse(req.ID, api.ErrCodeMethodNotFound, "method not found")
	}
}

func (s *Server) handleGetRealtimeStats(req *api.Request) *api.Response {
	var params api.PortParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return s.errorResponse(req.ID, api.ErrCodeInvalidParams, "invalid params")
	}

	stats := s.collector.GetStats(params.Port)

	result := api.RealtimeStatsResult{
		Port:        stats.Port,
		RxBytes:     stats.RxBytes,
		TxBytes:     stats.TxBytes,
		RxPackets:   stats.RxPackets,
		TxPackets:   stats.TxPackets,
		Connections: stats.Connections,
		RxRate:      stats.RxRate,
		TxRate:      stats.TxRate,
	}

	return s.successResponse(req.ID, result)
}

func (s *Server) handleGetHistoricalStats(req *api.Request) *api.Response {
	var params api.HistoricalParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return s.errorResponse(req.ID, api.ErrCodeInvalidParams, "invalid params")
	}

	// Query daily stats
	dailyStats, err := s.db.QueryDailyStats(params.Port, params.StartDate, params.EndDate)
	if err != nil {
		return s.errorResponse(req.ID, api.ErrCodeInternal, err.Error())
	}

	// Aggregate totals
	result := api.HistoricalStatsResult{
		Port:      params.Port,
		StartDate: params.StartDate,
		EndDate:   params.EndDate,
	}

	for _, d := range dailyStats {
		result.TotalRx += d.RxBytes
		result.TotalTx += d.TxBytes
		if d.PeakRxRate > result.PeakRxRate {
			result.PeakRxRate = d.PeakRxRate
		}
		if d.PeakTxRate > result.PeakTxRate {
			result.PeakTxRate = d.PeakTxRate
		}

		result.DailyStats = append(result.DailyStats, api.DayStats{
			Date:        d.Date,
			RxBytes:     d.RxBytes,
			TxBytes:     d.TxBytes,
			RxPackets:   d.RxPackets,
			TxPackets:   d.TxPackets,
			Connections: d.Connections,
			PeakRxRate:  d.PeakRxRate,
			PeakTxRate:  d.PeakTxRate,
		})
	}

	result.TotalBytes = result.TotalRx + result.TotalTx

	return s.successResponse(req.ID, result)
}

func (s *Server) handleGetStatus(req *api.Request) *api.Response {
	uptime := time.Since(s.startTime)

	result := api.StatusResult{
		Running:        true,
		Uptime:         formatDuration(uptime),
		StartTime:      s.startTime.Format(time.RFC3339),
		MonitoredPorts: s.config.Ports,
		DataDir:        s.config.DataDir,
		RetentionDays:  s.config.RetentionDays,
		SocketPath:     s.config.SocketPath,
		Version:        "0.1.0",
	}

	return s.successResponse(req.ID, result)
}

func (s *Server) handleAddPort(req *api.Request) *api.Response {
	var params api.PortParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return s.errorResponse(req.ID, api.ErrCodeInvalidParams, "invalid params")
	}

	if err := s.loader.AddPort(params.Port); err != nil {
		return s.errorResponse(req.ID, api.ErrCodeInternal, err.Error())
	}

	// Add to config
	s.config.Ports = append(s.config.Ports, params.Port)

	return s.successResponse(req.ID, api.SuccessResult{
		Success: true,
		Message: fmt.Sprintf("port %d added", params.Port),
	})
}

func (s *Server) handleRemovePort(req *api.Request) *api.Response {
	var params api.PortParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return s.errorResponse(req.ID, api.ErrCodeInvalidParams, "invalid params")
	}

	if err := s.loader.RemovePort(params.Port); err != nil {
		return s.errorResponse(req.ID, api.ErrCodeInternal, err.Error())
	}

	// Remove from config
	newPorts := make([]uint16, 0, len(s.config.Ports))
	for _, p := range s.config.Ports {
		if p != params.Port {
			newPorts = append(newPorts, p)
		}
	}
	s.config.Ports = newPorts

	return s.successResponse(req.ID, api.SuccessResult{
		Success: true,
		Message: fmt.Sprintf("port %d removed", params.Port),
	})
}

func (s *Server) handleListPorts(req *api.Request) *api.Response {
	return s.successResponse(req.ID, api.ListPortsResult{
		Ports: s.config.Ports,
	})
}

func (s *Server) successResponse(id int, result interface{}) *api.Response {
	data, _ := json.Marshal(result)
	return &api.Response{
		Result: data,
		ID:     id,
	}
}

func (s *Server) errorResponse(id, code int, message string) *api.Response {
	return &api.Response{
		Error: &api.Error{
			Code:    code,
			Message: message,
		},
		ID: id,
	}
}

func (s *Server) sendError(encoder *json.Encoder, id, code int, message string) {
	encoder.Encode(s.errorResponse(id, code, message))
}

func formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	secs := int(d.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %02d:%02d:%02d", days, hours, mins, secs)
	}
	return fmt.Sprintf("%02d:%02d:%02d", hours, mins, secs)
}
