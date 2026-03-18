package panel

import (
	"fmt"
	"strings"
	"time"

	"github.com/NimbleMarkets/ntcharts/barchart"
	"github.com/afcollins/kbx/internal/store"
	"github.com/afcollins/kbx/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
)

// TimelineSource provides timeline data. Implemented by both EventStore and MetricStore.
type TimelineSource interface {
	Timeline(buckets int) []store.TimelineBucket
}

type TimelinePanel struct {
	Width      int
	Height     int
	Focused    bool
	Cursor     int
	SelectionStart int // -1 means no selection started
	SelectionEnd   int // -1 means no selection ended
	buckets        []store.TimelineBucket
	graphWidth     int    // actual rendered bar count (may be < len(buckets))
	lastBucketSig  string // tracks whether bucket data changed between renders
}

func NewTimelinePanel() *TimelinePanel {
	return &TimelinePanel{
		Height:         28,
		SelectionStart: -1,
		SelectionEnd:   -1,
	}
}

// contentWidth returns the usable character width inside the panel border+padding.
func (tp *TimelinePanel) contentWidth() int {
	w := tp.Width - 4 // 2 border + 2 padding
	if w < 10 {
		w = 10
	}
	return w
}

func (tp *TimelinePanel) MoveLeft() {
	if tp.Cursor > 0 {
		tp.Cursor--
	}
}

func (tp *TimelinePanel) MoveRight() {
	max := tp.maxCursor()
	if tp.Cursor < max {
		tp.Cursor++
	}
}

func (tp *TimelinePanel) PageLeft() {
	step := tp.pageStep()
	tp.Cursor -= step
	if tp.Cursor < 0 {
		tp.Cursor = 0
	}
}

func (tp *TimelinePanel) PageRight() {
	step := tp.pageStep()
	tp.Cursor += step
	max := tp.maxCursor()
	if tp.Cursor > max {
		tp.Cursor = max
	}
}

func (tp *TimelinePanel) MoveToStart() {
	tp.Cursor = 0
}

func (tp *TimelinePanel) MoveToEnd() {
	tp.Cursor = tp.maxCursor()
}

func (tp *TimelinePanel) maxCursor() int {
	// Use graphWidth when available (set during render) for accurate bounds
	n := tp.graphWidth
	if n == 0 {
		n = len(tp.buckets)
	}
	if n > len(tp.buckets) {
		n = len(tp.buckets)
	}
	if n < 1 {
		return 0
	}
	return n - 1
}

func (tp *TimelinePanel) pageStep() int {
	n := tp.graphWidth / 10
	if n < 5 {
		n = 5
	}
	return n
}

// MarkSelection sets one end of the selection range. First press sets start,
// second press sets end and returns true to signal the app to apply the filter.
func (tp *TimelinePanel) MarkSelection() bool {
	if len(tp.buckets) == 0 {
		return false
	}

	if tp.SelectionStart == -1 {
		// First mark: set start
		tp.SelectionStart = tp.Cursor
		tp.SelectionEnd = -1
		return false
	}

	// Second mark: set end and signal ready to apply
	tp.SelectionEnd = tp.Cursor

	// Ensure start <= end
	if tp.SelectionStart > tp.SelectionEnd {
		tp.SelectionStart, tp.SelectionEnd = tp.SelectionEnd, tp.SelectionStart
	}
	return true
}

// SelectedTimeRange returns the time range of the current selection.
func (tp *TimelinePanel) SelectedTimeRange() (time.Time, time.Time) {
	if tp.SelectionStart < 0 || tp.SelectionEnd < 0 {
		return time.Time{}, time.Time{}
	}
	if tp.SelectionStart >= len(tp.buckets) || tp.SelectionEnd >= len(tp.buckets) {
		return time.Time{}, time.Time{}
	}
	return tp.buckets[tp.SelectionStart].Start, tp.buckets[tp.SelectionEnd].End
}

// ClearSelection resets the selection range.
func (tp *TimelinePanel) ClearSelection() {
	tp.SelectionStart = -1
	tp.SelectionEnd = -1
}

// CursorTime returns the time label for the bucket under the cursor.
func (tp *TimelinePanel) CursorTime() string {
	if tp.Cursor < len(tp.buckets) {
		return tp.buckets[tp.Cursor].Start.Format("15:04:05")
	}
	return ""
}

func (tp *TimelinePanel) View(s TimelineSource) string {
	cw := tp.contentWidth()

	panelStyle := styles.PanelStyle.Width(tp.Width - 2)
	if tp.Focused {
		panelStyle = styles.FocusedPanelStyle.Width(tp.Width - 2)
	}

	chartHeight := tp.Height - 6
	if chartHeight < 3 {
		chartHeight = 3
	}

	// First pass: get buckets at full width to determine Y-axis label width
	tp.buckets = s.Timeline(cw)
	if len(tp.buckets) == 0 {
		return panelStyle.Render(styles.TitleStyle.Render("Timeline") + "\n(no data)")
	}

	maxCount := 0
	for _, b := range tp.buckets {
		if b.Count > maxCount {
			maxCount = b.Count
		}
	}

	// Y-axis label width: right-aligned count + separator
	yLabelW := len(fmt.Sprintf("%d", maxCount)) + 1
	graphWidth := cw - yLabelW
	if graphWidth < 10 {
		graphWidth = 10
	}

	// Re-request buckets at correct graph width
	tp.buckets = s.Timeline(graphWidth)
	maxCount = 0
	for _, b := range tp.buckets {
		if b.Count > maxCount {
			maxCount = b.Count
		}
	}

	if tp.Cursor >= len(tp.buckets) {
		tp.Cursor = len(tp.buckets) - 1
	}

	sig := tp.bucketSignature()
	if sig != tp.lastBucketSig {
		tp.SelectionStart = -1
		tp.SelectionEnd = -1
		tp.lastBucketSig = sig
	}

	// Build barchart using ntcharts
	barNormal := lipgloss.NewStyle().Foreground(styles.ColorBar)
	barSelected := lipgloss.NewStyle().Foreground(styles.ColorAccent)
	barCursor := lipgloss.NewStyle().Foreground(styles.ColorPrimary)

	bc := barchart.New(graphWidth, chartHeight,
		barchart.WithNoAxis(),
		barchart.WithBarGap(0),
		barchart.WithNoAutoBarWidth(),
		barchart.WithBarWidth(1),
	)

	for col, bucket := range tp.buckets {
		style := barNormal
		if col == tp.Cursor && tp.Focused {
			style = barCursor
		} else if tp.inSelection(col) {
			style = barSelected
		}
		bc.Push(barchart.BarData{
			Values: []barchart.BarValue{{Value: float64(bucket.Count), Style: style}},
		})
	}
	bc.Draw()

	// Split chart output into lines and prepend Y-axis labels
	chartLines := strings.Split(bc.View(), "\n")

	// Measure actual rendered width from the chart output
	if len(chartLines) > 0 {
		tp.graphWidth = lipgloss.Width(chartLines[0])
	} else {
		tp.graphWidth = graphWidth
	}
	if tp.Cursor > tp.maxCursor() {
		tp.Cursor = tp.maxCursor()
	}
	axisStyle := lipgloss.NewStyle().Foreground(styles.ColorMuted)
	labelFmt := fmt.Sprintf("%%%dd ", yLabelW-1)

	var rows []string
	for i, line := range chartLines {
		// Show labels at top, middle, and bottom rows
		var label string
		switch {
		case i == 0:
			label = axisStyle.Render(fmt.Sprintf(labelFmt, maxCount))
		case i == len(chartLines)/2:
			label = axisStyle.Render(fmt.Sprintf(labelFmt, maxCount/2))
		case i == len(chartLines)-1:
			label = axisStyle.Render(fmt.Sprintf(labelFmt, 0))
		default:
			label = strings.Repeat(" ", yLabelW)
		}
		rows = append(rows, label+line)
	}

	// X-axis line
	rows = append(rows, strings.Repeat(" ", yLabelW)+axisStyle.Render(strings.Repeat("─", tp.graphWidth)))

	// Cursor indicator row
	cursorBarStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(styles.ColorPrimary)
	selectedBarStyle := lipgloss.NewStyle().Foreground(styles.ColorAccent)

	var cursorRow strings.Builder
	cursorRow.WriteString(strings.Repeat(" ", yLabelW))
	for col := 0; col < tp.graphWidth; col++ {
		if col == tp.Cursor && tp.Focused {
			cursorRow.WriteString(cursorBarStyle.Render("▲"))
		} else if tp.inSelection(col) {
			cursorRow.WriteString(selectedBarStyle.Render("─"))
		} else {
			cursorRow.WriteString(" ")
		}
	}
	rows = append(rows, cursorRow.String())

	// Time labels aligned to graph area
	startLabel := tp.buckets[0].Start.Format("15:04:05")
	endLabel := tp.buckets[len(tp.buckets)-1].End.Format("15:04:05")
	timePadding := tp.graphWidth - len(startLabel) - len(endLabel)
	if timePadding < 1 {
		timePadding = 1
	}
	timeRow := strings.Repeat(" ", yLabelW) + startLabel + strings.Repeat(" ", timePadding) + endLabel

	// Title with context
	var b strings.Builder
	titleParts := fmt.Sprintf("Timeline (max: %d/bucket)", maxCount)
	if tp.Focused {
		cursorBucket := tp.buckets[tp.Cursor]
		titleParts += fmt.Sprintf("  cursor: %s (%d events)",
			cursorBucket.Start.Format("15:04:05"), cursorBucket.Count)
	}
	if tp.SelectionStart >= 0 && tp.SelectionEnd >= 0 {
		s, e := tp.SelectedTimeRange()
		titleParts += fmt.Sprintf("  selected: %s-%s", s.Format("15:04:05"), e.Format("15:04:05"))
	} else if tp.SelectionStart >= 0 {
		titleParts += fmt.Sprintf("  start: %s (press Enter for end)",
			tp.buckets[tp.SelectionStart].Start.Format("15:04:05"))
	}
	b.WriteString(styles.TitleStyle.Render(titleParts))
	b.WriteString("\n")
	b.WriteString(strings.Join(rows, "\n"))
	b.WriteString("\n")
	b.WriteString(timeRow)
	if tp.Focused {
		b.WriteString("\n")
		b.WriteString(styles.HelpStyle.Render("[←→] move  [⇧←→] page  [Home/End] jump  [Enter] mark start/end  [Esc] clear"))
	}

	return panelStyle.Render(b.String())
}

func (tp *TimelinePanel) bucketSignature() string {
	if len(tp.buckets) == 0 {
		return ""
	}
	first := tp.buckets[0]
	last := tp.buckets[len(tp.buckets)-1]
	return fmt.Sprintf("%d:%s:%s", len(tp.buckets),
		first.Start.Format(time.RFC3339Nano),
		last.End.Format(time.RFC3339Nano))
}

func (tp *TimelinePanel) inSelection(col int) bool {
	if tp.SelectionStart < 0 {
		return false
	}
	if tp.SelectionEnd < 0 {
		// Selection in progress — highlight range from start to cursor
		start, end := tp.SelectionStart, tp.Cursor
		if start > end {
			start, end = end, start
		}
		return col >= start && col <= end
	}
	start, end := tp.SelectionStart, tp.SelectionEnd
	if start > end {
		start, end = end, start
	}
	return col >= start && col <= end
}
