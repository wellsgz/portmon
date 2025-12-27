// Package tui provides the terminal user interface for portmon.
package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/wellsgz/portmon/api"
	"github.com/wellsgz/portmon/internal/client"
	"github.com/wellsgz/portmon/internal/storage"
)

// View represents the current view state
type View int

const (
	ViewDashboard View = iota
	ViewDatePicker
	ViewHelp
)

// DateRangePreset represents a date range option
type DateRangePreset int

const (
	PresetToday DateRangePreset = iota
	PresetYesterday
	PresetLast7Days
	PresetLast30Days
	PresetThisMonth
	PresetLastMonth
	PresetCustomCycle
)

// KeyMap defines the key bindings
type KeyMap struct {
	Quit      key.Binding
	DateRange key.Binding
	Refresh   key.Binding
	Help      key.Binding
	Up        key.Binding
	Down      key.Binding
	Enter     key.Binding
	Escape    key.Binding
	NextPort  key.Binding
	PrevPort  key.Binding
}

var DefaultKeyMap = KeyMap{
	Quit:      key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	DateRange: key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "date range")),
	Refresh:   key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	Help:      key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Up:        key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:      key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Enter:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
	Escape:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	NextPort:  key.NewBinding(key.WithKeys("n", "tab"), key.WithHelp("n", "next port")),
	PrevPort:  key.NewBinding(key.WithKeys("N", "shift+tab"), key.WithHelp("N", "prev port")),
}

// Messages
type tickMsg time.Time
type statsMsg struct {
	realtime   *api.RealtimeStatsResult
	historical *api.HistoricalStatsResult
	status     *api.StatusResult
	err        error
}

// Model is the main TUI model
type Model struct {
	// Connection
	client     *client.Client
	socketPath string
	connected  bool
	lastError  string

	// State
	currentView   View
	port          uint16
	ports         []uint16
	portIndex     int
	width, height int

	// Data
	realtimeStats   *api.RealtimeStatsResult
	historicalStats *api.HistoricalStatsResult
	daemonStatus    *api.StatusResult

	// Date range
	datePreset   DateRangePreset
	cycleDay     int
	startDate    string
	endDate      string
	presetCursor int

	// UI state
	keys KeyMap
}

// New creates a new TUI model
func New(socketPath string, port uint16) Model {
	return Model{
		socketPath:   socketPath,
		port:         port,
		currentView:  ViewDashboard,
		datePreset:   PresetThisMonth,
		cycleDay:     1,
		presetCursor: 4, // This Month
		keys:         DefaultKeyMap,
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.connect(),
		m.tick(),
	)
}

func (m Model) connect() tea.Cmd {
	return func() tea.Msg {
		c := client.New(m.socketPath)
		if err := c.Connect(); err != nil {
			return statsMsg{err: err}
		}
		m.client = c
		return m.fetchStats()()
	}
}

func (m Model) tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) fetchStats() tea.Cmd {
	return func() tea.Msg {
		if m.client == nil {
			c := client.New(m.socketPath)
			if err := c.Connect(); err != nil {
				return statsMsg{err: err}
			}
			m.client = c
		}

		// Get daemon status
		status, err := m.client.GetStatus()
		if err != nil {
			return statsMsg{err: err}
		}

		// Get realtime stats for current port
		var realtime *api.RealtimeStatsResult
		if m.port > 0 {
			realtime, err = m.client.GetRealtimeStats(m.port)
			if err != nil {
				return statsMsg{err: err}
			}
		}

		// Calculate date range
		now := time.Now()
		var startDate, endDate string
		switch m.datePreset {
		case PresetToday:
			s, e := storage.GetTodayDates(now)
			startDate, endDate = storage.FormatDateRange(s, e)
		case PresetYesterday:
			s, e := storage.GetYesterdayDates(now)
			startDate, endDate = storage.FormatDateRange(s, e)
		case PresetLast7Days:
			s, e := storage.GetLastNDays(7, now)
			startDate, endDate = storage.FormatDateRange(s, e)
		case PresetLast30Days:
			s, e := storage.GetLastNDays(30, now)
			startDate, endDate = storage.FormatDateRange(s, e)
		case PresetThisMonth:
			s, e := storage.GetCurrentMonthDates(now)
			startDate, endDate = storage.FormatDateRange(s, e)
		case PresetLastMonth:
			s, e := storage.GetLastMonthDates(now)
			startDate, endDate = storage.FormatDateRange(s, e)
		case PresetCustomCycle:
			s, e := storage.GetBillingCycleDates(m.cycleDay, now)
			startDate, endDate = storage.FormatDateRange(s, e)
		}

		// Get historical stats
		var historical *api.HistoricalStatsResult
		if m.port > 0 {
			historical, err = m.client.GetHistoricalStats(m.port, startDate, endDate)
			if err != nil {
				return statsMsg{err: err}
			}
		}

		return statsMsg{
			realtime:   realtime,
			historical: historical,
			status:     status,
		}
	}
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tickMsg:
		return m, tea.Batch(m.fetchStats(), m.tick())

	case statsMsg:
		if msg.err != nil {
			m.connected = false
			m.lastError = msg.err.Error()
		} else {
			m.connected = true
			m.lastError = ""
			m.realtimeStats = msg.realtime
			m.historicalStats = msg.historical
			m.daemonStatus = msg.status
			if msg.status != nil {
				m.ports = msg.status.MonitoredPorts
				// Set first port if none selected
				if m.port == 0 && len(m.ports) > 0 {
					m.port = m.ports[0]
				}
			}
		}
		return m, nil
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.currentView {
	case ViewDashboard:
		return m.handleDashboardKey(msg)
	case ViewDatePicker:
		return m.handleDatePickerKey(msg)
	case ViewHelp:
		return m.handleHelpKey(msg)
	}
	return m, nil
}

func (m Model) handleDashboardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		if m.client != nil {
			m.client.Close()
		}
		return m, tea.Quit

	case key.Matches(msg, m.keys.DateRange):
		m.currentView = ViewDatePicker
		return m, nil

	case key.Matches(msg, m.keys.Help):
		m.currentView = ViewHelp
		return m, nil

	case key.Matches(msg, m.keys.Refresh):
		return m, m.fetchStats()

	case key.Matches(msg, m.keys.NextPort):
		if len(m.ports) > 0 {
			m.portIndex = (m.portIndex + 1) % len(m.ports)
			m.port = m.ports[m.portIndex]
			return m, m.fetchStats()
		}

	case key.Matches(msg, m.keys.PrevPort):
		if len(m.ports) > 0 {
			m.portIndex = (m.portIndex - 1 + len(m.ports)) % len(m.ports)
			m.port = m.ports[m.portIndex]
			return m, m.fetchStats()
		}
	}
	return m, nil
}

func (m Model) handleDatePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Escape):
		m.currentView = ViewDashboard
		return m, nil

	case key.Matches(msg, m.keys.Up):
		if m.presetCursor > 0 {
			m.presetCursor--
		}
		return m, nil

	case key.Matches(msg, m.keys.Down):
		if m.presetCursor < 6 {
			m.presetCursor++
		}
		return m, nil

	case key.Matches(msg, m.keys.Enter):
		m.datePreset = DateRangePreset(m.presetCursor)
		m.currentView = ViewDashboard
		return m, m.fetchStats()
	}
	return m, nil
}

func (m Model) handleHelpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Escape), key.Matches(msg, m.keys.Help), key.Matches(msg, m.keys.Quit):
		m.currentView = ViewDashboard
		return m, nil
	}
	return m, nil
}

// View renders the UI
func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	switch m.currentView {
	case ViewDatePicker:
		return m.viewDatePicker()
	case ViewHelp:
		return m.viewHelp()
	default:
		return m.viewDashboard()
	}
}

// FormatBytes formats bytes into human readable format
func FormatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// FormatRate formats bytes per second
func FormatRate(r float64) string {
	return FormatBytes(uint64(r)) + "/s"
}
