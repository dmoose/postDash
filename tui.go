package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#58a6ff")).
			PaddingRight(2)

	statsStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8b949e"))

	errorBadge = lipgloss.NewStyle().
			Background(lipgloss.Color("#f85149")).
			Foreground(lipgloss.Color("#fff")).
			Padding(0, 1)

	warnBadge = lipgloss.NewStyle().
			Background(lipgloss.Color("#d29922")).
			Foreground(lipgloss.Color("#000")).
			Padding(0, 1)

	infoBadge = lipgloss.NewStyle().
			Background(lipgloss.Color("#58a6ff")).
			Foreground(lipgloss.Color("#000")).
			Padding(0, 1)

	debugBadge = lipgloss.NewStyle().
			Background(lipgloss.Color("#8b949e")).
			Foreground(lipgloss.Color("#000")).
			Padding(0, 1)

	sourceStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#58a6ff")).Bold(true)
	channelStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#bc8cff"))
	actionStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#d2a8ff"))
	agentStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#7ee787"))
	durationStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#d2a8ff"))
	timeStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#8b949e"))
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#484f58"))
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#8b949e"))
	filterActive  = lipgloss.NewStyle().Foreground(lipgloss.Color("#7ee787"))
)

// TUI messages
type eventMsg Event
type batchEventsMsg []Event
type statsMsg HubStats
type tickMsg time.Time

// tuiModel is the bubbletea model for the TUI dashboard.
type tuiModel struct {
	serverURL string
	events    []Event
	stats     HubStats
	viewport  viewport.Model
	filter    textinput.Model
	editing   bool // whether the filter input is active
	paused    bool
	width     int
	height    int
	ready     bool
	filterStr string
	eventCh   chan Event
}

func newTUIModel(serverURL string) tuiModel {
	fi := textinput.New()
	fi.Placeholder = "type to filter (text or source:x level:error)"
	fi.CharLimit = 200
	fi.Width = 60

	return tuiModel{
		serverURL: strings.TrimRight(serverURL, "/"),
		events:    make([]Event, 0, 500),
		filter:    fi,
		eventCh:   make(chan Event, 256),
	}
}

func (m tuiModel) Init() tea.Cmd {
	return tea.Batch(
		m.loadRecent(),
		m.startSSE(),
		m.waitForEvent(),
		m.pollStats(),
		m.tick(),
	)
}

// loadRecent fetches existing events from the server on startup.
func (m tuiModel) loadRecent() tea.Cmd {
	return func() tea.Msg {
		resp, err := http.Get(m.serverURL + "/events/recent?n=100")
		if err != nil {
			return nil
		}
		defer resp.Body.Close()
		var events []Event
		if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
			return nil
		}
		return batchEventsMsg(events)
	}
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()

		// ctrl+c always quits, no matter what
		if key == "ctrl+c" {
			return m, tea.Quit
		}

		if m.editing {
			switch key {
			case "enter":
				m.editing = false
				m.filter.Blur()
				m.filterStr = m.filter.Value()
				m.refreshViewport()
				return m, nil
			case "esc":
				m.editing = false
				m.filter.Blur()
				// Keep current filter value
				m.refreshViewport()
				return m, nil
			default:
				var cmd tea.Cmd
				m.filter, cmd = m.filter.Update(msg)
				m.filterStr = m.filter.Value()
				m.refreshViewport()
				return m, cmd
			}
		}

		// Normal mode keys
		switch key {
		case "q":
			return m, tea.Quit
		case "p":
			m.paused = !m.paused
			m.refreshViewport()
		case "c":
			m.events = m.events[:0]
			m.refreshViewport()
		case "/":
			m.editing = true
			m.filter.Focus()
			return m, m.filter.Cursor.BlinkCmd()
		case "x":
			// Clear filter
			m.filterStr = ""
			m.filter.SetValue("")
			m.refreshViewport()
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		headerHeight := 4
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-headerHeight)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - headerHeight
		}
		m.refreshViewport()

	case batchEventsMsg:
		m.events = append(m.events, []Event(msg)...)
		if len(m.events) > 500 {
			m.events = m.events[len(m.events)-500:]
		}
		m.refreshViewport()

	case eventMsg:
		if !m.paused {
			m.events = append(m.events, Event(msg))
			if len(m.events) > 500 {
				m.events = m.events[len(m.events)-500:]
			}
			m.refreshViewport()
		}
		cmds = append(cmds, m.waitForEvent())

	case statsMsg:
		m.stats = HubStats(msg)

	case tickMsg:
		cmds = append(cmds, m.pollStats(), m.tick())
	}

	return m, tea.Batch(cmds...)
}

func (m *tuiModel) refreshViewport() {
	if m.ready {
		m.viewport.SetContent(m.renderEvents())
		m.viewport.GotoBottom()
	}
}

func (m tuiModel) View() string {
	if !m.ready {
		return "Connecting..."
	}

	// Line 1: title + stats
	header := titleStyle.Render("eventrelay")
	header += statsStyle.Render(fmt.Sprintf(
		"events: %d  rate: %.1f/s  clients: %d",
		m.stats.TotalEvents, m.stats.RecentRate, m.stats.ClientCount,
	))
	if errs, ok := m.stats.ByLevel["error"]; ok && errs > 0 {
		header += "  " + errorBadge.Render(fmt.Sprintf("ERR %d", errs))
	}
	if warns, ok := m.stats.ByLevel["warn"]; ok && warns > 0 {
		header += "  " + warnBadge.Render(fmt.Sprintf("WARN %d", warns))
	}

	// Line 2: filter
	var filterLine string
	if m.editing {
		filterLine = m.filter.View()
	} else if m.filterStr != "" {
		filterLine = filterActive.Render("filter: " + m.filterStr)
	} else {
		filterLine = dimStyle.Render("no filter")
	}

	// Line 3: help
	help := helpStyle.Render("  /:filter  x:clear-filter  p:pause  c:clear  q:quit  ctrl+c:force-quit")
	if m.paused {
		help += "  " + warnBadge.Render("PAUSED")
	}

	return header + "\n" + filterLine + "\n" + help + "\n" + m.viewport.View()
}

func (m tuiModel) renderEvents() string {
	var sb strings.Builder
	for _, evt := range m.events {
		if !m.matchesFilter(evt) {
			continue
		}
		sb.WriteString(m.renderEvent(evt))
		sb.WriteByte('\n')
	}
	if sb.Len() == 0 {
		if m.filterStr != "" {
			return dimStyle.Render("  No events match filter: " + m.filterStr)
		}
		return dimStyle.Render("  Waiting for events...")
	}
	return sb.String()
}

func (m tuiModel) renderEvent(evt Event) string {
	var parts []string

	switch evt.Level {
	case "error":
		parts = append(parts, errorBadge.Render("ERR"))
	case "warn":
		parts = append(parts, warnBadge.Render("WRN"))
	case "info":
		parts = append(parts, infoBadge.Render("INF"))
	case "debug":
		parts = append(parts, debugBadge.Render("DBG"))
	}

	parts = append(parts, sourceStyle.Render(evt.Source))
	if evt.Channel != "" {
		parts = append(parts, channelStyle.Render(evt.Channel))
	}
	if evt.Action != "" {
		parts = append(parts, actionStyle.Render(evt.Action))
	}
	if evt.AgentID != "" {
		parts = append(parts, agentStyle.Render(evt.AgentID))
	}
	if evt.DurationMS != nil {
		parts = append(parts, durationStyle.Render(fmt.Sprintf("%dms", *evt.DurationMS)))
	}
	parts = append(parts, timeStyle.Render(evt.TS.Format("15:04:05")))

	line := strings.Join(parts, " ")
	if len(evt.Data) > 0 {
		data, _ := json.Marshal(evt.Data)
		if len(data) < 80 {
			line += "  " + dimStyle.Render(string(data))
		}
	}
	return line
}

// matchesFilter does substring matching by default. Supports key:value
// syntax for field-specific matching.
func (m tuiModel) matchesFilter(evt Event) bool {
	if m.filterStr == "" {
		return true
	}

	for part := range strings.FieldsSeq(m.filterStr) {
		kv := strings.SplitN(part, ":", 2)
		if len(kv) == 2 {
			key, val := kv[0], strings.ToLower(kv[1])
			switch key {
			case "source":
				if !strings.Contains(strings.ToLower(evt.Source), val) {
					return false
				}
			case "channel":
				if !strings.Contains(strings.ToLower(evt.Channel), val) {
					return false
				}
			case "level":
				if strings.ToLower(evt.Level) != val {
					return false
				}
			case "action":
				if !strings.Contains(strings.ToLower(evt.Action), val) {
					return false
				}
			case "agent":
				if !strings.Contains(strings.ToLower(evt.AgentID), val) {
					return false
				}
			default:
				// Unknown key — treat as plain text search
				if !eventContains(evt, strings.ToLower(part)) {
					return false
				}
			}
		} else {
			// Plain text — match against any field
			if !eventContains(evt, strings.ToLower(part)) {
				return false
			}
		}
	}
	return true
}

func eventContains(evt Event, term string) bool {
	return strings.Contains(strings.ToLower(evt.Source), term) ||
		strings.Contains(strings.ToLower(evt.Channel), term) ||
		strings.Contains(strings.ToLower(evt.Action), term) ||
		strings.Contains(strings.ToLower(evt.Level), term) ||
		strings.Contains(strings.ToLower(evt.AgentID), term)
}

// startSSE launches a persistent goroutine that reads the SSE stream.
func (m tuiModel) startSSE() tea.Cmd {
	return func() tea.Msg {
		go func() {
			for {
				m.readSSEStream()
				time.Sleep(2 * time.Second)
			}
		}()
		return nil
	}
}

func (m tuiModel) readSSEStream() {
	url := m.serverURL + "/events/stream"
	resp, err := http.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var evt Event
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &evt); err != nil {
			continue
		}
		m.eventCh <- evt
	}
}

func (m tuiModel) waitForEvent() tea.Cmd {
	return func() tea.Msg {
		evt, ok := <-m.eventCh
		if !ok {
			return nil
		}
		return eventMsg(evt)
	}
}

func (m tuiModel) pollStats() tea.Cmd {
	return func() tea.Msg {
		resp, err := http.DefaultClient.Get(m.serverURL + "/events/stats")
		if err != nil {
			return nil
		}
		defer resp.Body.Close()
		var stats HubStats
		json.NewDecoder(resp.Body).Decode(&stats)
		return statsMsg(stats)
	}
}

func (m tuiModel) tick() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
