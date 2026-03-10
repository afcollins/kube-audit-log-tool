package tui

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/afcollins/kube-audit-log-tool/internal/audit"
	"github.com/afcollins/kube-audit-log-tool/internal/export"
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

// Focus panels: 0-3 = primary facets, 4-5 = secondary facets, 6 = timeline, 7 = event list
const (
	focusVerb      = 0
	focusResource  = 1
	focusUsername   = 2
	focusUserAgent = 3
	focusStatus    = 4
	focusSourceIP  = 5
	focusTimeline  = 6
	focusEventList = 7

	primaryFacetCount   = 4
	secondaryFacetStart = 4
	totalFacetCount     = 6
)

type Model struct {
	state      appState
	store      *store.EventStore
	files      []string
	tempFiles  []string // temp files to clean up on exit
	width      int
	height     int
	focus      int
	statusMsg  string
	exportPath string

	filePicker     *panel.FilePickerPanel
	filterBar      *panel.FilterBar
	facets         [6]*panel.FacetPanel
	showSecondary  bool
	timeline       *panel.TimelinePanel
	eventList      *panel.EventListPanel
	eventDetail    *panel.EventDetailPanel

	loadedCount int
	loadStart   time.Time
}

type filesParsedMsg struct {
	results []*audit.ParseResult
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
			panel.NewFacetPanel("Status", "status"),       // secondary
			panel.NewFacetPanel("Source IP", "sourceip"),   // secondary
		},
		timeline:    panel.NewTimelinePanel(),
		eventList:   panel.NewEventListPanel(),
		eventDetail: panel.NewEventDetailPanel(),
	}

	if len(files) == 0 {
		m.state = stateFilePicker
		m.filePicker = panel.NewFilePickerPanel()
	} else {
		m.state = stateLoading
	}

	return m
}

func (m Model) Init() tea.Cmd {
	if m.state == stateLoading {
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
				// Skip files that fail to parse
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
		m.focus = focusVerb
		m.facets[focusVerb].Focused = true
		m.refreshPanels()
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

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Event detail modal takes priority
	if m.eventDetail.Visible {
		return m.handleDetailKey(msg)
	}

	if m.state == stateFilePicker {
		return m.handleFilePickerKey(msg)
	}

	if m.state != stateDashboard {
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		return m, nil
	}

	switch msg.String() {
	case "q", "ctrl+c":
		m.cleanup()
		return m, tea.Quit

	case "tab":
		m.focusNext()
		return m, nil

	case "shift+tab":
		m.focusPrev()
		return m, nil

	case "up", "k":
		m.moveUp()
		return m, nil

	case "down", "j":
		m.moveDown()
		return m, nil

	case "left", "h":
		if m.focus == focusTimeline {
			m.timeline.MoveLeft()
		}
		return m, nil

	case "right", "l":
		if m.focus == focusTimeline {
			m.timeline.MoveRight()
		}
		return m, nil

	case "enter", " ":
		return m.selectCurrent()

	case "c":
		m.store.ClearFilters()
		m.timeline.ClearSelection()
		m.eventList.ResetCursor()
		m.refreshPanels()
		m.statusMsg = "Filters cleared"
		return m, nil

	case "f":
		m.showSecondary = !m.showSecondary
		if !m.showSecondary && m.focus >= secondaryFacetStart && m.focus < totalFacetCount {
			m.setFocus(focusVerb)
		}
		m.updateSizes()
		return m, nil

	case "e":
		return m, m.exportFiltered()

	case "d":
		if m.focus == focusEventList {
			return m.showDetail()
		}
		return m, nil

	case "esc":
		if m.focus == focusTimeline {
			// Clear time range filter and selection
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

func (m Model) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.eventDetail.Hide()
	case "up", "k":
		m.eventList.MoveUp()
		return m.showDetail()
	case "down", "j":
		m.eventList.MoveDown(m.store.FilteredCount())
		return m.showDetail()
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
			m.state = stateLoading
			return m, m.loadFiles()
		}
	}
	return m, nil
}

func (m *Model) setFocus(idx int) {
	// Clear old focus
	if m.focus >= 0 && m.focus < totalFacetCount {
		m.facets[m.focus].Focused = false
	}
	if m.focus == focusTimeline {
		m.timeline.Focused = false
	}
	if m.focus == focusEventList {
		m.eventList.Focused = false
	}

	m.focus = idx

	// Set new focus
	if idx >= 0 && idx < totalFacetCount {
		m.facets[idx].Focused = true
	}
	if idx == focusTimeline {
		m.timeline.Focused = true
	}
	if idx == focusEventList {
		m.eventList.Focused = true
	}
}

func (m *Model) isFocusable(idx int) bool {
	if idx >= secondaryFacetStart && idx < totalFacetCount && !m.showSecondary {
		return false
	}
	return true
}

func (m *Model) focusNext() {
	next := m.focus
	for {
		next = (next + 1) % (focusEventList + 1)
		if m.isFocusable(next) {
			break
		}
	}
	m.setFocus(next)
}

func (m *Model) focusPrev() {
	prev := m.focus
	for {
		prev = (prev - 1 + focusEventList + 1) % (focusEventList + 1)
		if m.isFocusable(prev) {
			break
		}
	}
	m.setFocus(prev)
}

func (m *Model) moveUp() {
	if m.focus >= 0 && m.focus <= 5 {
		m.facets[m.focus].MoveUp()
	}
	if m.focus == focusEventList {
		m.eventList.MoveUp()
	}
}

func (m *Model) moveDown() {
	if m.focus >= 0 && m.focus <= 5 {
		m.facets[m.focus].MoveDown()
	}
	if m.focus == focusEventList {
		m.eventList.MoveDown(m.store.FilteredCount())
	}
}

func (m Model) selectCurrent() (tea.Model, tea.Cmd) {
	if m.focus >= 0 && m.focus <= 5 {
		fp := m.facets[m.focus]

		if fp.Selected != "" {
			// Panel has an active filter — clear it
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
			// No active filter — apply the value under cursor
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
	if m.focus == focusTimeline {
		if m.timeline.MarkSelection() {
			// Both start and end are set — apply time filter
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
	if m.focus == focusEventList {
		return m.showDetail()
	}
	return m, nil
}

func (m Model) showDetail() (tea.Model, tea.Cmd) {
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

func (m *Model) refreshPanels() {
	for _, fp := range m.facets {
		fp.Update(m.store)
	}
}

func (m *Model) updateSizes() {
	// Primary facets: 4 panels across
	primaryWidth := m.width / primaryFacetCount
	for i := 0; i < primaryFacetCount; i++ {
		m.facets[i].Width = primaryWidth
		m.facets[i].Height = styles.FacetPanelHeight
	}

	// Secondary facets: 2 panels across
	secondaryWidth := m.width / (totalFacetCount - primaryFacetCount)
	for i := secondaryFacetStart; i < totalFacetCount; i++ {
		m.facets[i].Width = secondaryWidth
		m.facets[i].Height = styles.FacetPanelHeight
	}

	m.filterBar.Width = m.width
	m.timeline.Width = m.width
	m.timeline.Height = styles.TimelinePanelHeight
	m.eventList.Width = m.width

	facetRows := 1
	if m.showSecondary {
		facetRows = 2
	}
	remaining := m.height - (styles.FacetPanelHeight * facetRows) - styles.FilterBarHeight - styles.TimelinePanelHeight - styles.StatusBarHeight
	if remaining < 5 {
		remaining = 5
	}
	m.eventList.Height = remaining
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

func (m Model) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	switch m.state {
	case stateFilePicker:
		return m.filePicker.View()
	case stateLoading:
		return lipgloss.NewStyle().Padding(2, 4).Render(
			styles.TitleStyle.Render("Loading audit events...") + "\n\n" +
				"Parsing " + strings.Join(m.files, ", ") + "...\n" +
				styles.HelpStyle.Render("This may take a moment for large files."),
		)
	case stateDashboard:
		return m.dashboardView()
	}

	return ""
}

func (m Model) dashboardView() string {
	var sections []string

	// Filter bar
	sections = append(sections, m.filterBar.View(m.store))

	// Primary facet panels row
	primaryRow := lipgloss.JoinHorizontal(lipgloss.Top,
		m.facets[0].View(), m.facets[1].View(), m.facets[2].View(), m.facets[3].View(),
	)
	sections = append(sections, primaryRow)

	// Secondary facet panels row (toggle with 'f')
	if m.showSecondary {
		secondaryRow := lipgloss.JoinHorizontal(lipgloss.Top,
			m.facets[4].View(), m.facets[5].View(),
		)
		sections = append(sections, secondaryRow)
	}

	// Timeline
	sections = append(sections, m.timeline.View(m.store))

	// Event list
	sections = append(sections, m.eventList.View(m.store))

	// Status bar
	help := styles.HelpStyle.Render(
		"[Tab] focus  [↑↓] navigate  [Enter/Space] filter  [f] more facets  [d] detail  [e] export  [c] clear  [q] quit",
	)
	status := ""
	if m.statusMsg != "" {
		status = styles.StatusBarStyle.Render(m.statusMsg) + "  "
	}
	sections = append(sections, status+help)

	dashboard := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Overlay detail modal if visible
	if m.eventDetail.Visible {
		return m.eventDetail.View()
	}

	return dashboard
}

func Run(files []string) error {
	m := NewModel(files)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
