// Package client provides an IPC client for communicating with the daemon.
package client

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"

	"github.com/wellsgz/portmon/api"
)

// Client connects to the portmon daemon via Unix socket.
type Client struct {
	socketPath string
	conn       net.Conn
	encoder    *json.Encoder
	scanner    *bufio.Scanner
	mu         sync.Mutex
	reqID      atomic.Int32
}

// New creates a new client.
func New(socketPath string) *Client {
	if socketPath == "" {
		socketPath = "/run/portmon/portmon.sock"
	}
	return &Client{
		socketPath: socketPath,
	}
}

// Connect establishes connection to the daemon.
func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		return nil
	}

	conn, err := net.Dial("unix", c.socketPath)
	if err != nil {
		return fmt.Errorf("connecting to daemon: %w", err)
	}

	c.conn = conn
	c.encoder = json.NewEncoder(conn)
	c.scanner = bufio.NewScanner(conn)

	return nil
}

// Close closes the connection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		c.encoder = nil
		c.scanner = nil
		return err
	}
	return nil
}

// IsConnected returns true if connected to daemon.
func (c *Client) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn != nil
}

// call sends a request and waits for response.
func (c *Client) call(method string, params interface{}) (*api.Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return nil, errors.New("not connected")
	}

	id := int(c.reqID.Add(1))

	var paramsJSON json.RawMessage
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("marshaling params: %w", err)
		}
		paramsJSON = data
	}

	req := api.Request{
		Method: method,
		Params: paramsJSON,
		ID:     id,
	}

	if err := c.encoder.Encode(req); err != nil {
		c.conn.Close()
		c.conn = nil
		return nil, fmt.Errorf("sending request: %w", err)
	}

	if !c.scanner.Scan() {
		c.conn.Close()
		c.conn = nil
		if err := c.scanner.Err(); err != nil {
			return nil, fmt.Errorf("reading response: %w", err)
		}
		return nil, errors.New("connection closed")
	}

	var resp api.Response
	if err := json.Unmarshal(c.scanner.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("rpc error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	return &resp, nil
}

// GetRealtimeStats retrieves current stats for a port.
func (c *Client) GetRealtimeStats(port uint16) (*api.RealtimeStatsResult, error) {
	resp, err := c.call(api.MethodGetRealtimeStats, api.PortParams{Port: port})
	if err != nil {
		return nil, err
	}

	var result api.RealtimeStatsResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetHistoricalStats retrieves historical stats for a port and date range.
func (c *Client) GetHistoricalStats(port uint16, startDate, endDate string) (*api.HistoricalStatsResult, error) {
	resp, err := c.call(api.MethodGetHistoricalStats, api.HistoricalParams{
		Port:      port,
		StartDate: startDate,
		EndDate:   endDate,
	})
	if err != nil {
		return nil, err
	}

	var result api.HistoricalStatsResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetStatus retrieves daemon status.
func (c *Client) GetStatus() (*api.StatusResult, error) {
	resp, err := c.call(api.MethodGetStatus, nil)
	if err != nil {
		return nil, err
	}

	var result api.StatusResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// AddPort adds a port to monitoring.
func (c *Client) AddPort(port uint16) error {
	_, err := c.call(api.MethodAddPort, api.PortParams{Port: port})
	return err
}

// RemovePort removes a port from monitoring.
func (c *Client) RemovePort(port uint16) error {
	_, err := c.call(api.MethodRemovePort, api.PortParams{Port: port})
	return err
}

// ListPorts returns all monitored ports.
func (c *Client) ListPorts() ([]uint16, error) {
	resp, err := c.call(api.MethodListPorts, nil)
	if err != nil {
		return nil, err
	}

	var result api.ListPortsResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, err
	}
	return result.Ports, nil
}

// FlushStats triggers immediate persistence of stats to database.
func (c *Client) FlushStats() error {
	_, err := c.call(api.MethodFlushStats, nil)
	return err
}
