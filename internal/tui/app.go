package tui

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/afcollins/kube-audit-log-tool/internal/audit"
	"github.com/afcollins/kube-audit-log-tool/internal/export"
	"github.com/afcollins/kube-audit-log-tool/internal/metrics"
	"github.com/afcollins/kube-audit-log-tool/internal/mstore"
	"github.com/afcollins/kube-audit-log-tool/internal/store"
	"github.com/afcollins/kube-audit-log-tool/internal/tui/panel"
	"github.com/afcollins/kube-audit-log-tool/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type appState int

const (
	stateFilePicker appState = iota
	stateLoading
	stateDashboard
)

// Audit mode focus constants
const (
	focusVerb      = 0
	focusResource  = 1
	focusUsername   = 2
	focusUserAgent = 3
	focusStatus    = 4
	focusSourceIP  = 5

	primaryFacetCount   = 4
	secondaryFacetStart = 4
	totalFacetCount     = 6
)

type Model struct {
	state      appState
	files      []string
	tempFiles  []string
	width      int
	height     int
	focus      int
	statusMsg  string
	exportPath string

	// Shared panels
	filePicker  *panel.FilePickerPanel
	filterBar   *panel.FilterBar
	timeline    *panel.TimelinePanel
	eventDetail *panel.EventDetailPanel

	// Mode
	metricsMode bool

	// Audit mode
	store         *store.EventStore
	facets        [6]*panel.FacetPanel
	showSecondary bool
	maximized     bool
	eventList     *panel.EventListPanel

	// Metrics mode
	metricStore  *mstore.MetricStore
	metricFacets []*panel.FacetPanel
	metricList   *panel.MetricListPanel
	scatter      *panel.ScatterPanel
	mPrimary     int // number of primary metric facets
	mTotal       int // total visible metric facets

	loadedCount  int
	loadStart    time.Time
	confirmQuit  bool

	// Value range input mode (metrics only)
	valueInput    bool
	valueInputBuf string
}

type filesParsedMsg struct {
	results []*audit.ParseResult
	temps   []string
	elapsed time.Duration
}

type metricsParsedMsg struct {
	results []*metrics.ParseResult
	temps   []string
	elapsed time.Duration
}

type exportDoneMsg struct {
	path  string
	count int
	err   error
}

func NewModel(files []string) Model {
	m := Model{
		store: store.New(),
		files: files,

		filterBar: panel.NewFilterBar(),
		facets: [6]*panel.FacetPanel{
			panel.NewFacetPanel("Verb", "verb"),
			panel.NewFacetPanel("Resource", "resource"),
			panel.NewFacetPanel("User", "username"),
			panel.NewFacetPanel("User Agent", "useragent"),
			panel.NewFacetPanel("Status", "status"),
			panel.NewFacetPanel("Source IP", "sourceip"),
		},
		timeline:    panel.NewTimelinePanel(),
		eventList:   panel.NewEventListPanel(),
		eventDetail: panel.NewEventDetailPanel(),
		metricList:  panel.NewMetricListPanel(),
		scatter:     panel.NewScatterPanel(),
	}

	if len(files) == 0 {
		m.state = stateFilePicker
		m.filePicker = panel.NewFilePickerPanel()
	} else {
		m.state = stateLoading
		m.metricsMode = detectMetricsMode(files)
	}

	return m
}

func detectMetricsMode(files []string) bool {
	for _, f := range files {
		if strings.HasSuffix(f, ".json") || strings.HasSuffix(f, ".json.gz") {
			return true
		}
	}
	return false
}

func (m Model) Init() tea.Cmd {
	if m.state == stateLoading {
		if m.metricsMode {
			return m.loadMetrics()
		}
		return m.loadFiles()
	}
	return nil
}

func (m Model) loadFiles() tea.Cmd {
	files := m.files
	return func() tea.Msg {
		start := time.Now()
		results := make([]*audit.ParseResult, 0, len(files))
		var temps []string

		for i, path := range files {
			result, err := audit.ParseFile(path, i)
			if err != nil {
				continue
			}
			if result.ReadPath != path {
				temps = append(temps, result.ReadPath)
			}
			results = append(results, result)
		}

		return filesParsedMsg{
			results: results,
			temps:   temps,
			elapsed: time.Since(start),
		}
	}
}

func (m Model) loadMetrics() tea.Cmd {
	files := m.files
	return func() tea.Msg {
		start := time.Now()
		results := make([]*metrics.ParseResult, 0, len(files))
		var temps []string

		for i, path := range files {
			result, err := metrics.ParseFile(path, i)
			if err != nil {
				continue
			}
			if result.TempPath != "" {
				temps = append(temps, result.TempPath)
			}
			results = append(results, result)
		}

		return metricsParsedMsg{
			results: results,
			temps:   temps,
			elapsed: time.Since(start),
		}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateSizes()
		return m, nil

	case filesParsedMsg:
		m.store.Load(msg.results)
		m.tempFiles = msg.temps
		total := m.store.TotalCount()
		m.statusMsg = fmt.Sprintf("Loaded %d events in %s", total, msg.elapsed.Round(time.Millisecond))
		m.state = stateDashboard
		m.focus = 0
		m.facets[0].Focused = true
		m.refreshPanels()
		return m, nil

	case metricsParsedMsg:
		m.metricStore = mstore.New()
		m.metricStore.Load(msg.results)
		m.tempFiles = msg.temps
		total := m.metricStore.TotalCount()
		m.statusMsg = fmt.Sprintf("Loaded %d metrics in %s", total, msg.elapsed.Round(time.Millisecond))
		m.state = stateDashboard
		m.buildMetricFacets()
		m.focus = 0
		if len(m.metricFacets) > 0 {
			m.metricFacets[0].Focused = true
		}
		m.refreshPanels()
		m.updateSizes()
		return m, nil

	case exportDoneMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Export error: %v", msg.err)
		} else {
			m.statusMsg = fmt.Sprintf("Exported %d events to %s", msg.count, msg.path)
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m *Model) buildMetricFacets() {
	visible := m.metricStore.VisibleFields()

	var primary, secondary []string
	for _, f := range visible {
		if m.metricStore.IsPrimary(f) {
			primary = append(primary, f)
		} else {
			secondary = append(secondary, f)
		}
	}

	m.metricFacets = nil
	for _, f := range primary {
		m.metricFacets = append(m.metricFacets, panel.NewFacetPanel(f, f))
	}
	m.mPrimary = len(primary)

	for _, f := range secondary {
		m.metricFacets = append(m.metricFacets, panel.NewFacetPanel(f, f))
	}
	m.mTotal = len(m.metricFacets)
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// ctrl+c always quits immediately
	if msg.String() == "ctrl+c" {
		m.cleanup()
		return m, tea.Quit
	}

	// ctrl+z suspends the program
	if msg.String() == "ctrl+z" {
		return m, tea.Suspend
	}

	// Handle quit confirmation
	if m.confirmQuit {
		switch msg.String() {
		case "q", "y":
			m.cleanup()
			return m, tea.Quit
		default:
			m.confirmQuit = false
			m.statusMsg = ""
			return m, nil
		}
	}

	if m.eventDetail.Visible {
		return m.handleDetailKey(msg)
	}

	if m.state == stateFilePicker {
		return m.handleFilePickerKey(msg)
	}

	if m.state != stateDashboard {
		if msg.String() == "q" {
			return m, tea.Quit
		}
		return m, nil
	}

	// Handle value range input mode
	if m.valueInput {
		return m.handleValueInput(msg)
	}

	// Handle facet search mode: intercept keys when a facet is searching
	if fp := m.focusedFacet(); fp != nil {
		if fp.Searching {
			if fp.HandleSearchKey(msg.String()) {
				return m, nil
			}
			// Key was not consumed (e.g. up/down navigation) — fall through
		}
	}

	if m.metricsMode {
		return m.handleMetricsKey(msg)
	}
	return m.handleAuditKey(msg)
}

// focusedFacet returns the currently focused facet panel, or nil.
func (m *Model) focusedFacet() *panel.FacetPanel {
	if m.metricsMode {
		if m.focus >= 0 && m.focus < m.mTotal {
			return m.metricFacets[m.focus]
		}
	} else {
		if m.focus >= 0 && m.focus < totalFacetCount {
			return m.facets[m.focus]
		}
	}
	return nil
}

// ── Audit mode key handling (original logic) ──

func (m Model) handleAuditKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.confirmQuit = true
		m.statusMsg = "Quit? Press q/y to confirm, any other key to cancel"
		return m, nil

	case "tab":
		m.auditFocusNext()
		return m, nil

	case "shift+tab":
		m.auditFocusPrev()
		return m, nil

	case "up", "k":
		m.auditMoveUp()
		return m, nil

	case "down", "j":
		m.auditMoveDown()
		return m, nil

	case "left", "h":
		if m.focusIsTimeline() {
			m.timeline.MoveLeft()
		}
		return m, nil

	case "right", "l":
		if m.focusIsTimeline() {
			m.timeline.MoveRight()
		}
		return m, nil

	case "shift+left", "H":
		if m.focusIsTimeline() {
			m.timeline.PageLeft()
		}
		return m, nil

	case "shift+right", "L":
		if m.focusIsTimeline() {
			m.timeline.PageRight()
		}
		return m, nil

	case "home", "ctrl+a":
		if m.focusIsTimeline() {
			m.timeline.MoveToStart()
		}
		return m, nil

	case "end", "ctrl+e":
		if m.focusIsTimeline() {
			m.timeline.MoveToEnd()
		}
		return m, nil

	case "enter", " ":
		return m.auditSelectCurrent()

	case "c":
		m.store.ClearFilters()
		m.timeline.ClearSelection()
		m.eventList.ResetCursor()
		for _, fp := range m.facets {
			fp.ClearSearch()
		}
		m.refreshPanels()
		m.statusMsg = "Filters cleared"
		return m, nil

	case "f":
		m.showSecondary = !m.showSecondary
		if !m.showSecondary && m.focus >= secondaryFacetStart && m.focus < totalFacetCount {
			m.setAuditFocus(0)
		}
		m.updateSizes()
		return m, nil

	case "/":
		if m.focus >= 0 && m.focus < totalFacetCount {
			m.facets[m.focus].StartSearch()
		}
		return m, nil

	case "e":
		return m, m.exportFiltered()

	case "m":
		if m.focus >= 0 && m.focus < totalFacetCount {
			m.maximized = !m.maximized
		} else if m.maximized {
			m.maximized = false
		}
		if !m.maximized {
			for _, fp := range m.facets {
				fp.MaxItems = 0
			}
			m.updateSizes()
			m.refreshPanels()
		}
		return m, nil

	case "d":
		if m.focus == totalFacetCount+1 { // event list
			return m.showAuditDetail()
		}
		return m, nil

	case "esc":
		if m.maximized {
			m.maximized = false
			for _, fp := range m.facets {
				fp.MaxItems = 0
			}
			m.updateSizes()
			m.refreshPanels()
			return m, nil
		}
		if m.focusIsTimeline() {
			m.timeline.ClearSelection()
			f := m.store.Filters()
			if !f.TimeStart.IsZero() || !f.TimeEnd.IsZero() {
				f.TimeStart = time.Time{}
				f.TimeEnd = time.Time{}
				m.store.SetFilters(f)
				m.eventList.ResetCursor()
				m.refreshPanels()
				m.statusMsg = "Time filter cleared"
			} else {
				m.statusMsg = "Selection cleared"
			}
		} else {
			m.statusMsg = ""
		}
		return m, nil
	}

	return m, nil
}

// Audit focus: 0..5 = facets, 6 = timeline, 7 = event list
func (m *Model) focusIsTimeline() bool {
	return !m.metricsMode && m.focus == totalFacetCount
}

func (m *Model) setAuditFocus(idx int) {
	// Clear old
	if m.focus >= 0 && m.focus < totalFacetCount {
		m.facets[m.focus].Focused = false
	}
	if m.focus == totalFacetCount {
		m.timeline.Focused = false
	}
	if m.focus == totalFacetCount+1 {
		m.eventList.Focused = false
	}

	m.focus = idx

	if idx >= 0 && idx < totalFacetCount {
		m.facets[idx].Focused = true
	}
	if idx == totalFacetCount {
		m.timeline.Focused = true
	}
	if idx == totalFacetCount+1 {
		m.eventList.Focused = true
	}
}

func (m *Model) auditFocusable(idx int) bool {
	maxIdx := totalFacetCount + 1 // timeline + event list
	if idx > maxIdx {
		return false
	}
	if idx >= secondaryFacetStart && idx < totalFacetCount && !m.showSecondary {
		return false
	}
	return true
}

func (m *Model) auditFocusNext() {
	maxIdx := totalFacetCount + 1
	next := m.focus
	for {
		next = (next + 1) % (maxIdx + 1)
		if m.auditFocusable(next) {
			break
		}
	}
	m.setAuditFocus(next)
}

func (m *Model) auditFocusPrev() {
	maxIdx := totalFacetCount + 1
	prev := m.focus
	for {
		prev = (prev - 1 + maxIdx + 1) % (maxIdx + 1)
		if m.auditFocusable(prev) {
			break
		}
	}
	m.setAuditFocus(prev)
}

func (m *Model) auditMoveUp() {
	if m.focus >= 0 && m.focus < totalFacetCount {
		m.facets[m.focus].MoveUp()
	}
	if m.focus == totalFacetCount+1 {
		m.eventList.MoveUp()
	}
}

func (m *Model) auditMoveDown() {
	if m.focus >= 0 && m.focus < totalFacetCount {
		m.facets[m.focus].MoveDown()
	}
	if m.focus == totalFacetCount+1 {
		m.eventList.MoveDown(m.store.FilteredCount())
	}
}

func (m Model) auditSelectCurrent() (tea.Model, tea.Cmd) {
	if m.focus >= 0 && m.focus < totalFacetCount {
		fp := m.facets[m.focus]

		if fp.Selected != "" {
			if fp.Field == "status" {
				code, err := strconv.Atoi(fp.Selected)
				if err == nil {
					m.store.ToggleStatusFilter(code)
				}
			} else {
				m.store.ToggleFilter(fp.Field, fp.Selected)
			}
			m.eventList.ResetCursor()
			m.refreshPanels()
			m.statusMsg = fmt.Sprintf("Cleared filter: %s (%d results)", fp.Field, m.store.FilteredCount())
		} else {
			val := fp.SelectedValue()
			if val == "" {
				return m, nil
			}
			if fp.Field == "status" {
				code, err := strconv.Atoi(val)
				if err == nil {
					m.store.ToggleStatusFilter(code)
				}
			} else {
				m.store.ToggleFilter(fp.Field, val)
			}
			m.eventList.ResetCursor()
			m.refreshPanels()
			m.statusMsg = fmt.Sprintf("Filter: %s = %s (%d results)", fp.Field, val, m.store.FilteredCount())
		}
	}
	if m.focus == totalFacetCount {
		return m.handleTimelineSelect()
	}
	if m.focus == totalFacetCount+1 {
		return m.showAuditDetail()
	}
	return m, nil
}

func (m Model) handleTimelineSelect() (tea.Model, tea.Cmd) {
	if m.timeline.MarkSelection() {
		start, end := m.timeline.SelectedTimeRange()
		f := m.store.Filters()
		f.TimeStart = start
		f.TimeEnd = end
		m.store.SetFilters(f)
		m.eventList.ResetCursor()
		m.refreshPanels()
		m.statusMsg = fmt.Sprintf("Time filter: %s - %s (%d results)",
			start.Format("15:04:05"), end.Format("15:04:05"), m.store.FilteredCount())
	} else {
		m.statusMsg = fmt.Sprintf("Selection start: %s — move cursor and press Enter to set end",
			m.timeline.CursorTime())
	}
	return m, nil
}

func (m Model) handleScatterSelect() (tea.Model, tea.Cmd) {
	if m.scatter.MarkSelection() {
		start, end := m.scatter.SelectedTimeRange()
		m.metricStore.SetTimeFilter(start, end)
		m.metricList.ResetCursor()
		m.refreshPanels()
		m.statusMsg = fmt.Sprintf("Time filter: %s - %s (%d results)",
			start.Format("15:04:05"), end.Format("15:04:05"), m.metricStore.FilteredCount())
	} else {
		m.statusMsg = fmt.Sprintf("Selection start: %s — move cursor and press Enter to set end",
			m.scatter.CursorTime())
	}
	return m, nil
}

func (m Model) handleValueSelect() (tea.Model, tea.Cmd) {
	if m.scatter.MarkValueSelection() {
		vMin, vMax := m.scatter.SelectedValueRange()
		m.metricStore.SetValueFilter(vMin, vMax)
		m.metricList.ResetCursor()
		m.refreshPanels()
		m.statusMsg = fmt.Sprintf("Value filter: %.2f - %.2f (%d results)",
			vMin, vMax, m.metricStore.FilteredCount())
	} else {
		m.statusMsg = fmt.Sprintf("Value band start: %.2f — move ↑↓ and press v to set end",
			m.scatter.CursorValue())
	}
	return m, nil
}

func (m Model) handleValueInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		m.valueInput = false
		m.valueInputBuf = ""
		m.statusMsg = ""
		return m, nil
	case "enter":
		m.valueInput = false
		minV, maxV, err := parseValueRange(m.valueInputBuf)
		if err != nil {
			m.statusMsg = fmt.Sprintf("Invalid range: %v", err)
			return m, nil
		}
		m.metricStore.SetValueFilter(minV, maxV)
		m.metricList.ResetCursor()
		m.refreshPanels()
		m.statusMsg = fmt.Sprintf("Value filter: %s (%d results)",
			formatValueRange(minV, maxV), m.metricStore.FilteredCount())
		return m, nil
	case "backspace":
		if len(m.valueInputBuf) > 0 {
			m.valueInputBuf = m.valueInputBuf[:len(m.valueInputBuf)-1]
		}
		m.statusMsg = "Value range: " + m.valueInputBuf + "█"
		return m, nil
	default:
		if len(key) == 1 && strings.ContainsRune("0123456789.,-eE+", rune(key[0])) {
			m.valueInputBuf += key
			m.statusMsg = "Value range: " + m.valueInputBuf + "█"
		}
		return m, nil
	}
}

func parseFloatOrDefault(s string, def float64) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return def, nil
	}
	return strconv.ParseFloat(s, 64)
}

// parseValueRange parses value range input: "MIN,MAX", "MIN-MAX", "MIN", ",MAX", "-MAX".
func parseValueRange(s string) (float64, float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0, fmt.Errorf("empty input")
	}

	// Find separator: comma, or dash (skipping a leading dash for negatives)
	idx := strings.Index(s, ",")
	if idx < 0 {
		offset := 0
		if s[0] == '-' {
			offset = 1
		}
		if di := strings.Index(s[offset:], "-"); di >= 0 {
			idx = offset + di
		}
	}

	if idx < 0 {
		// No separator — minimum only
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid number: %s", s)
		}
		return v, math.MaxFloat64, nil
	}

	minV, err := parseFloatOrDefault(s[:idx], -math.MaxFloat64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid min: %s", s[:idx])
	}
	maxV, err := parseFloatOrDefault(s[idx+1:], math.MaxFloat64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid max: %s", s[idx+1:])
	}
	return minV, maxV, nil
}

func formatValueRange(minV, maxV float64) string {
	switch {
	case maxV >= math.MaxFloat64/2:
		return fmt.Sprintf("≥ %.2f", minV)
	case minV <= -math.MaxFloat64/2:
		return fmt.Sprintf("≤ %.2f", maxV)
	default:
		return fmt.Sprintf("%.2f - %.2f", minV, maxV)
	}
}

func (m Model) showAuditDetail() (tea.Model, tea.Cmd) {
	idx := m.eventList.SelectedIndex(m.store)
	if idx < 0 {
		return m, nil
	}

	e := &m.store.Events[idx]
	summary := panel.FormatEventDetail(e)

	raw, err := m.store.ReadRawJSON(idx)
	if err != nil {
		m.statusMsg = fmt.Sprintf("Error reading raw JSON: %v", err)
		return m, nil
	}

	m.eventDetail.Width = m.width
	m.eventDetail.Height = m.height
	m.eventDetail.Show(summary, raw)
	return m, nil
}

// ── Metrics mode key handling ──

func (m Model) handleMetricsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	focusTimeline := m.mTotal
	focusList := m.mTotal + 1

	switch msg.String() {
	case "q":
		m.confirmQuit = true
		m.statusMsg = "Quit? Press q/y to confirm, any other key to cancel"
		return m, nil

	case "tab":
		m.metricsFocusNext()
		return m, nil

	case "shift+tab":
		m.metricsFocusPrev()
		return m, nil

	case "up", "k":
		if m.focus >= 0 && m.focus < m.mTotal {
			m.metricFacets[m.focus].MoveUp()
		}
		if m.focus == focusTimeline {
			m.scatter.MoveUp()
		}
		if m.focus == focusList {
			m.metricList.MoveUp()
		}
		return m, nil

	case "down", "j":
		if m.focus >= 0 && m.focus < m.mTotal {
			m.metricFacets[m.focus].MoveDown()
		}
		if m.focus == focusTimeline {
			m.scatter.MoveDown()
		}
		if m.focus == focusList {
			m.metricList.MoveDown(m.metricStore.FilteredCount())
		}
		return m, nil

	case "left", "h":
		if m.focus == focusTimeline {
			m.scatter.MoveLeft()
		}
		return m, nil

	case "right", "l":
		if m.focus == focusTimeline {
			m.scatter.MoveRight()
		}
		return m, nil

	case "shift+left", "H":
		if m.focus == focusTimeline {
			m.scatter.PageLeft()
		}
		return m, nil

	case "shift+right", "L":
		if m.focus == focusTimeline {
			m.scatter.PageRight()
		}
		return m, nil

	case "shift+up", "K":
		if m.focus == focusTimeline {
			m.scatter.PageUp()
		}
		return m, nil

	case "shift+down", "J":
		if m.focus == focusTimeline {
			m.scatter.PageDown()
		}
		return m, nil

	case "home", "ctrl+a":
		if m.focus == focusTimeline {
			m.scatter.MoveToStart()
		}
		return m, nil

	case "end", "ctrl+e":
		if m.focus == focusTimeline {
			m.scatter.MoveToEnd()
		}
		return m, nil

	case "enter", " ":
		return m.metricsSelectCurrent()

	case "v":
		if m.focus == focusTimeline {
			return m.handleValueSelect()
		}
		return m, nil

	case "V":
		m.valueInput = true
		m.valueInputBuf = ""
		m.statusMsg = "Value range: MIN  MIN,MAX  ,MAX  (Enter to apply, Esc to cancel)"
		return m, nil

	case "c":
		m.metricStore.ClearFilters()
		m.scatter.ClearSelection()
		m.scatter.ClearValueSelection()
		m.metricList.ResetCursor()
		for _, fp := range m.metricFacets {
			fp.ClearSearch()
		}
		m.refreshPanels()
		m.statusMsg = "Filters cleared"
		return m, nil

	case "/":
		if m.focus >= 0 && m.focus < m.mTotal {
			m.metricFacets[m.focus].StartSearch()
		}
		return m, nil

	case "f":
		m.showSecondary = !m.showSecondary
		if !m.showSecondary && m.focus >= m.mPrimary && m.focus < m.mTotal {
			m.setMetricsFocus(0)
		}
		m.updateSizes()
		return m, nil

	case "m":
		if m.focus >= 0 && m.focus < m.mTotal {
			m.maximized = !m.maximized
		} else if m.maximized {
			m.maximized = false
		}
		if !m.maximized {
			for _, fp := range m.metricFacets {
				fp.MaxItems = 0
			}
			m.updateSizes()
			m.refreshPanels()
		}
		return m, nil

	case "d":
		if m.focus == focusList {
			return m.showMetricDetail()
		}
		return m, nil

	case "esc":
		if m.maximized {
			m.maximized = false
			for _, fp := range m.metricFacets {
				fp.MaxItems = 0
			}
			m.updateSizes()
			m.refreshPanels()
			return m, nil
		}
		if m.focus == focusTimeline {
			m.scatter.ClearSelection()
			m.scatter.ClearValueSelection()
			hadFilter := false
			if !m.metricStore.TimeStart().IsZero() || !m.metricStore.TimeEnd().IsZero() {
				m.metricStore.ClearTimeFilter()
				hadFilter = true
			}
			if m.metricStore.HasValueFilter() {
				m.metricStore.ClearValueFilter()
				hadFilter = true
			}
			if hadFilter {
				m.metricList.ResetCursor()
				m.refreshPanels()
				m.statusMsg = "Filters cleared"
			} else {
				m.statusMsg = "Selection cleared"
			}
		} else {
			m.statusMsg = ""
		}
		return m, nil
	}

	return m, nil
}

func (m *Model) setMetricsFocus(idx int) {
	// Clear old
	if m.focus >= 0 && m.focus < m.mTotal {
		m.metricFacets[m.focus].Focused = false
	}
	if m.focus == m.mTotal {
		m.scatter.Focused = false
	}
	if m.focus == m.mTotal+1 {
		m.metricList.Focused = false
	}

	m.focus = idx

	if idx >= 0 && idx < m.mTotal {
		m.metricFacets[idx].Focused = true
	}
	if idx == m.mTotal {
		m.scatter.Focused = true
	}
	if idx == m.mTotal+1 {
		m.metricList.Focused = true
	}
}

func (m *Model) metricsFocusable(idx int) bool {
	maxIdx := m.mTotal + 1
	if idx > maxIdx {
		return false
	}
	// Secondary facets hidden when showSecondary is false
	if idx >= m.mPrimary && idx < m.mTotal && !m.showSecondary {
		return false
	}
	return true
}

func (m *Model) metricsFocusNext() {
	maxIdx := m.mTotal + 1
	next := m.focus
	for {
		next = (next + 1) % (maxIdx + 1)
		if m.metricsFocusable(next) {
			break
		}
	}
	m.setMetricsFocus(next)
}

func (m *Model) metricsFocusPrev() {
	maxIdx := m.mTotal + 1
	prev := m.focus
	for {
		prev = (prev - 1 + maxIdx + 1) % (maxIdx + 1)
		if m.metricsFocusable(prev) {
			break
		}
	}
	m.setMetricsFocus(prev)
}

func (m Model) metricsSelectCurrent() (tea.Model, tea.Cmd) {
	if m.focus >= 0 && m.focus < m.mTotal {
		fp := m.metricFacets[m.focus]

		if fp.Selected != "" {
			m.metricStore.ToggleFilter(fp.Field, fp.Selected)
			m.metricList.ResetCursor()
			m.refreshPanels()
			m.statusMsg = fmt.Sprintf("Cleared filter: %s (%d results)", fp.Field, m.metricStore.FilteredCount())
		} else {
			val := fp.SelectedValue()
			if val == "" {
				return m, nil
			}
			m.metricStore.ToggleFilter(fp.Field, val)
			m.metricList.ResetCursor()
			m.refreshPanels()
			m.statusMsg = fmt.Sprintf("Filter: %s = %s (%d results)", fp.Field, val, m.metricStore.FilteredCount())
		}
	}
	if m.focus == m.mTotal {
		return m.handleScatterSelect()
	}
	if m.focus == m.mTotal+1 {
		return m.showMetricDetail()
	}
	return m, nil
}

func (m Model) showMetricDetail() (tea.Model, tea.Cmd) {
	idx := m.metricList.SelectedIndex(m.metricStore)
	if idx < 0 {
		return m, nil
	}

	e := &m.metricStore.Events[idx]
	summary := panel.FormatMetricDetail(e)

	raw, err := m.metricStore.ReadRawJSON(idx)
	if err != nil {
		m.statusMsg = fmt.Sprintf("Error reading raw JSON: %v", err)
		return m, nil
	}

	m.eventDetail.Width = m.width
	m.eventDetail.Height = m.height
	m.eventDetail.Show(summary, raw)
	return m, nil
}

// ── Common handlers ──

func (m Model) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.eventDetail.Hide()
	case "up", "k":
		if m.metricsMode {
			m.metricList.MoveUp()
			return m.showMetricDetail()
		}
		m.eventList.MoveUp()
		return m.showAuditDetail()
	case "down", "j":
		if m.metricsMode {
			m.metricList.MoveDown(m.metricStore.FilteredCount())
			return m.showMetricDetail()
		}
		m.eventList.MoveDown(m.store.FilteredCount())
		return m.showAuditDetail()
	case "left", "h":
		m.eventDetail.ScrollUp()
	case "right", "l":
		m.eventDetail.ScrollDown()
	case "r":
		m.eventDetail.ToggleRaw()
	}
	return m, nil
}

func (m Model) handleFilePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		m.filePicker.MoveUp()
	case "down", "j":
		m.filePicker.MoveDown()
	case " ":
		m.filePicker.ToggleSelection()
	case "enter":
		paths := m.filePicker.SelectedPaths()
		if len(paths) > 0 {
			m.files = paths
			m.metricsMode = detectMetricsMode(paths)
			m.state = stateLoading
			if m.metricsMode {
				return m, m.loadMetrics()
			}
			return m, m.loadFiles()
		}
	}
	return m, nil
}

func (m *Model) filteredCount() int {
	if m.metricsMode {
		return m.metricStore.FilteredCount()
	}
	return m.store.FilteredCount()
}

func (m *Model) refreshPanels() {
	if m.metricsMode {
		for _, fp := range m.metricFacets {
			fp.Update(m.metricStore)
		}
	} else {
		for _, fp := range m.facets {
			fp.Update(m.store)
		}
	}
}

func (m *Model) updateSizes() {
	if m.metricsMode {
		m.updateMetricsSizes()
	} else {
		m.updateAuditSizes()
	}
}

func (m *Model) updateAuditSizes() {
	facetRows := 1
	if m.showSecondary {
		facetRows = 2
	}

	// Compute proportional heights
	available := m.height - styles.FilterBarHeight - styles.StatusBarHeight
	if available < 15 {
		available = 15
	}

	facetTotalRatio := styles.FacetHeightRatio * facetRows
	totalRatio := facetTotalRatio + styles.TimelineHeightRatio + styles.ListHeightRatio
	facetH := available * styles.FacetHeightRatio / totalRatio
	timelineH := available * styles.TimelineHeightRatio / totalRatio
	listH := available - (facetH * facetRows) - timelineH

	if facetH < styles.MinFacetPanelHeight {
		facetH = styles.MinFacetPanelHeight
	}
	if timelineH < 8 {
		timelineH = 8
	}
	if listH < 5 {
		listH = 5
	}

	primaryWidth := m.width / primaryFacetCount
	for i := 0; i < primaryFacetCount; i++ {
		m.facets[i].Width = primaryWidth
		m.facets[i].Height = facetH
	}

	secondaryWidth := m.width / (totalFacetCount - primaryFacetCount)
	for i := secondaryFacetStart; i < totalFacetCount; i++ {
		m.facets[i].Width = secondaryWidth
		m.facets[i].Height = facetH
	}

	m.filterBar.Width = m.width
	m.timeline.Width = m.width
	m.timeline.Height = timelineH
	m.eventList.Width = m.width
	m.eventList.Height = listH
}

func (m *Model) updateMetricsSizes() {
	if m.mTotal == 0 {
		return
	}

	// Primary: up to 4 across, then wrap
	perRow := m.mPrimary
	if perRow > 4 {
		perRow = 4
	}
	if perRow == 0 {
		perRow = 1
	}

	// Account for primary rows if > 4
	primaryRows := (m.mPrimary + 3) / 4
	if primaryRows < 1 {
		primaryRows = 1
	}
	secCount := m.mTotal - m.mPrimary
	facetRows := primaryRows
	if m.showSecondary && secCount > 0 {
		facetRows++
	}

	// Compute proportional heights
	available := m.height - styles.FilterBarHeight - styles.StatusBarHeight
	if available < 15 {
		available = 15
	}

	facetTotalRatio := styles.FacetHeightRatio * facetRows
	totalRatio := facetTotalRatio + styles.TimelineHeightRatio + styles.ListHeightRatio
	facetH := available * styles.FacetHeightRatio / totalRatio
	scatterH := available * styles.TimelineHeightRatio / totalRatio
	listH := available - (facetH * facetRows) - scatterH

	if facetH < styles.MinFacetPanelHeight {
		facetH = styles.MinFacetPanelHeight
	}
	if scatterH < 8 {
		scatterH = 8
	}
	if listH < 5 {
		listH = 5
	}

	primaryWidth := m.width / perRow
	for i := 0; i < m.mPrimary && i < m.mTotal; i++ {
		m.metricFacets[i].Width = primaryWidth
		m.metricFacets[i].Height = facetH
	}

	// Secondary: all in one row, evenly spaced
	if secCount > 0 {
		secWidth := m.width / secCount
		for i := m.mPrimary; i < m.mTotal; i++ {
			m.metricFacets[i].Width = secWidth
			m.metricFacets[i].Height = facetH
		}
	}

	m.filterBar.Width = m.width
	m.scatter.Width = m.width
	m.scatter.Height = scatterH
	m.metricList.Width = m.width
	m.metricList.Height = listH
}

func (m Model) exportFiltered() tea.Cmd {
	s := m.store
	return func() tea.Msg {
		path := fmt.Sprintf("audit-export-%s.json", time.Now().Format("20060102-150405"))
		count, err := export.ExportJSON(s, path)
		return exportDoneMsg{path: path, count: count, err: err}
	}
}

func (m *Model) cleanup() {
	for _, tmp := range m.tempFiles {
		os.Remove(tmp)
	}
}

// ── Views ──

func (m Model) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	switch m.state {
	case stateFilePicker:
		return m.filePicker.View()
	case stateLoading:
		label := "audit events"
		if m.metricsMode {
			label = "metrics"
		}
		return lipgloss.NewStyle().Padding(2, 4).Render(
			styles.TitleStyle.Render("Loading "+label+"...") + "\n\n" +
				"Parsing " + strings.Join(m.files, ", ") + "...\n" +
				styles.HelpStyle.Render("This may take a moment for large files."),
		)
	case stateDashboard:
		if m.metricsMode {
			return m.metricsDashboardView()
		}
		return m.dashboardView()
	}

	return ""
}

func (m Model) dashboardView() string {
	// Maximized facet panel — render full screen
	if m.maximized && m.focus >= 0 && m.focus < totalFacetCount {
		fp := m.facets[m.focus]
		fp.Width = m.width
		fp.Height = m.height - styles.StatusBarHeight
		fp.MaxItems = (fp.Height - 3)
		fp.Update(m.store)
		help := styles.HelpStyle.Render("[m/Esc] restore  [↑↓] navigate  [Enter/Space] filter  [q] quit")
		return fp.View() + "\n" + help
	}

	var sections []string

	sections = append(sections, m.filterBar.View(m.store))

	primaryRow := lipgloss.JoinHorizontal(lipgloss.Top,
		m.facets[0].View(), m.facets[1].View(), m.facets[2].View(), m.facets[3].View(),
	)
	sections = append(sections, primaryRow)

	if m.showSecondary {
		secondaryRow := lipgloss.JoinHorizontal(lipgloss.Top,
			m.facets[4].View(), m.facets[5].View(),
		)
		sections = append(sections, secondaryRow)
	}

	sections = append(sections, m.timeline.View(m.store))
	sections = append(sections, m.eventList.View(m.store))

	help := styles.HelpStyle.Render(
		"[Tab] focus  [↑↓] navigate  [Enter/Space] filter  [f] more facets  [m] maximize  [d] detail  [e] export  [c] clear  [q] quit",
	)
	status := ""
	if m.statusMsg != "" {
		status = styles.StatusBarStyle.Render(m.statusMsg) + "  "
	}
	sections = append(sections, status+help)

	dashboard := lipgloss.JoinVertical(lipgloss.Left, sections...)

	if m.eventDetail.Visible {
		return m.eventDetail.View()
	}

	return dashboard
}

func (m Model) metricsDashboardView() string {
	// Maximized facet panel
	if m.maximized && m.focus >= 0 && m.focus < m.mTotal {
		fp := m.metricFacets[m.focus]
		fp.Width = m.width
		fp.Height = m.height - styles.StatusBarHeight
		fp.MaxItems = (fp.Height - 3)
		fp.Update(m.metricStore)
		help := styles.HelpStyle.Render("[m/Esc] restore  [↑↓] navigate  [Enter/Space] filter  [q] quit")
		return fp.View() + "\n" + help
	}

	var sections []string

	// Job summary info bar
	if m.metricStore.JobSummary != nil {
		sections = append(sections, m.jobSummaryBar())
	}

	// Filter bar
	sections = append(sections, m.filterBar.ViewMetrics(
		m.metricStore.ActiveFilters(),
		m.metricStore.TimeStart(), m.metricStore.TimeEnd(),
		m.metricStore.FilteredCount(), m.metricStore.TotalCount(),
	))

	// Primary facet panels — rows of up to 4
	if m.mPrimary > 0 {
		for rowStart := 0; rowStart < m.mPrimary; rowStart += 4 {
			rowEnd := rowStart + 4
			if rowEnd > m.mPrimary {
				rowEnd = m.mPrimary
			}
			var views []string
			for i := rowStart; i < rowEnd; i++ {
				views = append(views, m.metricFacets[i].View())
			}
			sections = append(sections, lipgloss.JoinHorizontal(lipgloss.Top, views...))
		}
	}

	// Secondary facet panels (toggle with 'f') — single row
	if m.showSecondary && m.mTotal > m.mPrimary {
		var views []string
		for i := m.mPrimary; i < m.mTotal; i++ {
			views = append(views, m.metricFacets[i].View())
		}
		sections = append(sections, lipgloss.JoinHorizontal(lipgloss.Top, views...))
	}

	// Scatter plot
	sections = append(sections, m.scatter.View(m.metricStore))

	// Metric list
	sections = append(sections, m.metricList.View(m.metricStore))

	// Status bar
	help := styles.HelpStyle.Render(
		"[Tab] focus  [↑↓] navigate  [Enter/Space] filter  [f] more facets  [m] maximize  [d] detail  [c] clear  [q] quit",
	)
	status := ""
	if m.statusMsg != "" {
		status = styles.StatusBarStyle.Render(m.statusMsg) + "  "
	}
	sections = append(sections, status+help)

	dashboard := lipgloss.JoinVertical(lipgloss.Left, sections...)

	if m.eventDetail.Visible {
		return m.eventDetail.View()
	}

	return dashboard
}

func (m Model) jobSummaryBar() string {
	js := m.metricStore.JobSummary
	var parts []string
	if v, ok := js["clusterName"].(string); ok && v != "" {
		parts = append(parts, "cluster:"+v)
	}
	if v, ok := js["platform"].(string); ok && v != "" {
		parts = append(parts, "platform:"+v)
	}
	if v, ok := js["k8sVersion"].(string); ok && v != "" {
		parts = append(parts, "k8s:"+v)
	}
	if v, ok := js["ocpVersion"].(string); ok && v != "" {
		parts = append(parts, "ocp:"+v)
	}
	if v, ok := js["sdnType"].(string); ok && v != "" {
		parts = append(parts, "sdn:"+v)
	}
	if v, ok := js["totalNodes"].(float64); ok {
		parts = append(parts, fmt.Sprintf("nodes:%.0f", v))
	}
	if len(parts) == 0 {
		return ""
	}
	return styles.FilterBarStyle.Width(m.width).Render(" " + strings.Join(parts, "  "))
}

func Run(files []string) error {
	m := NewModel(files)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
