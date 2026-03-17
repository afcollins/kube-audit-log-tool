package panel

import (
	"fmt"
	"strings"
	"time"

	"github.com/afcollins/kube-audit-log-tool/internal/store"
	"github.com/afcollins/kube-audit-log-tool/internal/tui/styles"
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
	lastBucketSig  string // tracks whether bucket data changed between renders
}

func NewTimelinePanel() *TimelinePanel {
	return &TimelinePanel{
		Height:         styles.TimelinePanelHeight,
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
	if tp.Cursor < len(tp.buckets)-1 {
		tp.Cursor++
	}
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

	// Show filtered events so timeline reflects active filters
	tp.buckets = s.Timeline(cw)
	if len(tp.buckets) == 0 {
		style := styles.PanelStyle.Width(tp.Width - 2)
		if tp.Focused {
			style = styles.FocusedPanelStyle.Width(tp.Width - 2)
		}
		return style.Render(styles.TitleStyle.Render("Timeline") + "\n(no data)")
	}

	// Clamp cursor to valid range
	if tp.Cursor >= len(tp.buckets) {
		tp.Cursor = len(tp.buckets) - 1
	}

	// Clear stale selection when bucket data changes (e.g., after filter applied)
	sig := tp.bucketSignature()
	if sig != tp.lastBucketSig {
		tp.SelectionStart = -1
		tp.SelectionEnd = -1
		tp.lastBucketSig = sig
	}

	maxCount := 0
	for _, b := range tp.buckets {
		if b.Count > maxCount {
			maxCount = b.Count
		}
	}

	barHeight := tp.Height - 5
	if barHeight < 3 {
		barHeight = 3
	}

	barStyle := lipgloss.NewStyle().Foreground(styles.ColorBar)
	selectedBarStyle := lipgloss.NewStyle().Foreground(styles.ColorAccent)
	cursorBarStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(styles.ColorPrimary)

	// Build rows from top to bottom
	var rows []string
	for row := barHeight; row >= 1; row-- {
		threshold := float64(row) / float64(barHeight) * float64(maxCount)
		var line strings.Builder
		for col, b := range tp.buckets {
			isCursor := col == tp.Cursor && tp.Focused
			isSelected := tp.inSelection(col)
			filled := float64(b.Count) >= threshold

			switch {
			case filled && isCursor:
				line.WriteString(cursorBarStyle.Render(styles.BarCharFull))
			case filled && isSelected:
				line.WriteString(selectedBarStyle.Render(styles.BarCharFull))
			case filled:
				line.WriteString(barStyle.Render(styles.BarCharFull))
			case isCursor:
				line.WriteString(cursorBarStyle.Render(" "))
			case isSelected:
				line.WriteString(selectedBarStyle.Render("·"))
			default:
				line.WriteString(" ")
			}
		}
		rows = append(rows, line.String())
	}

	// X-axis line
	axisStyle := lipgloss.NewStyle().Foreground(styles.ColorMuted)
	rows = append(rows, axisStyle.Render(strings.Repeat("─", cw)))

	// Cursor indicator row
	var cursorRow strings.Builder
	for col := range tp.buckets {
		if col == tp.Cursor && tp.Focused {
			cursorRow.WriteString(cursorBarStyle.Render("▲"))
		} else if tp.inSelection(col) {
			cursorRow.WriteString(selectedBarStyle.Render("─"))
		} else {
			cursorRow.WriteString(" ")
		}
	}
	rows = append(rows, cursorRow.String())

	// Time labels — align to content width
	startLabel := tp.buckets[0].Start.Format("15:04:05")
	endLabel := tp.buckets[len(tp.buckets)-1].End.Format("15:04:05")
	timePadding := cw - len(startLabel) - len(endLabel)
	if timePadding < 1 {
		timePadding = 1
	}
	timeRow := startLabel + strings.Repeat(" ", timePadding) + endLabel

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
		b.WriteString(styles.HelpStyle.Render("[←→] move  [Enter] mark start/end  [Esc] clear range"))
	}

	style := styles.PanelStyle.Width(tp.Width - 2)
	if tp.Focused {
		style = styles.FocusedPanelStyle.Width(tp.Width - 2)
	}

	return style.Render(b.String())
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
		// Only start is set — highlight just the start bucket
		return col == tp.SelectionStart
	}
	start, end := tp.SelectionStart, tp.SelectionEnd
	if start > end {
		start, end = end, start
	}
	return col >= start && col <= end
}
