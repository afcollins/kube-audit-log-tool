package panel

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/NimbleMarkets/ntcharts/canvas"
	"github.com/NimbleMarkets/ntcharts/linechart"
	"github.com/afcollins/kube-audit-log-tool/internal/mstore"
	"github.com/afcollins/kube-audit-log-tool/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
)

type ScatterPanel struct {
	Width          int
	Height         int
	Focused        bool
	Cursor         int
	SelectionStart int
	SelectionEnd   int

	graphWidth int
	graphOriX  int
	minTime    time.Time
	maxTime    time.Time
	lastSig    string
}

func NewScatterPanel() *ScatterPanel {
	return &ScatterPanel{
		Height:         styles.TimelinePanelHeight,
		SelectionStart: -1,
		SelectionEnd:   -1,
	}
}

func (sp *ScatterPanel) contentWidth() int {
	w := sp.Width - 4
	if w < 10 {
		w = 10
	}
	return w
}

func (sp *ScatterPanel) MoveLeft() {
	if sp.Cursor > 0 {
		sp.Cursor--
	}
}

func (sp *ScatterPanel) MoveRight() {
	if sp.graphWidth > 0 && sp.Cursor < sp.graphWidth-1 {
		sp.Cursor++
	}
}

func (sp *ScatterPanel) MarkSelection() bool {
	if sp.graphWidth == 0 {
		return false
	}
	if sp.SelectionStart == -1 {
		sp.SelectionStart = sp.Cursor
		sp.SelectionEnd = -1
		return false
	}
	sp.SelectionEnd = sp.Cursor
	if sp.SelectionStart > sp.SelectionEnd {
		sp.SelectionStart, sp.SelectionEnd = sp.SelectionEnd, sp.SelectionStart
	}
	return true
}

func (sp *ScatterPanel) SelectedTimeRange() (time.Time, time.Time) {
	if sp.SelectionStart < 0 || sp.SelectionEnd < 0 || sp.graphWidth <= 0 {
		return time.Time{}, time.Time{}
	}
	duration := sp.maxTime.Sub(sp.minTime)
	start := sp.minTime.Add(duration * time.Duration(sp.SelectionStart) / time.Duration(sp.graphWidth))
	end := sp.minTime.Add(duration * time.Duration(sp.SelectionEnd+1) / time.Duration(sp.graphWidth))
	return start, end
}

func (sp *ScatterPanel) ClearSelection() {
	sp.SelectionStart = -1
	sp.SelectionEnd = -1
}

func (sp *ScatterPanel) CursorTime() string {
	if sp.graphWidth <= 0 || sp.minTime.IsZero() {
		return ""
	}
	duration := sp.maxTime.Sub(sp.minTime)
	t := sp.minTime.Add(duration * time.Duration(sp.Cursor) / time.Duration(sp.graphWidth))
	return t.Format("15:04:05")
}

func (sp *ScatterPanel) View(ms *mstore.MetricStore) string {
	cw := sp.contentWidth()
	chartHeight := sp.Height - 7
	if chartHeight < 3 {
		chartHeight = 3
	}

	filtered := ms.Filtered()
	panelStyle := styles.PanelStyle.Width(sp.Width - 2)
	if sp.Focused {
		panelStyle = styles.FocusedPanelStyle.Width(sp.Width - 2)
	}

	if len(filtered) == 0 {
		return panelStyle.Render(styles.TitleStyle.Render("Scatter") + "\n(no data)")
	}

	// Find min/max timestamp and value
	var minT, maxT time.Time
	minV, maxV := math.MaxFloat64, -math.MaxFloat64
	for i, idx := range filtered {
		e := &ms.Events[idx]
		if i == 0 || e.Timestamp.Before(minT) {
			minT = e.Timestamp
		}
		if i == 0 || e.Timestamp.After(maxT) {
			maxT = e.Timestamp
		}
		if e.Value < minV {
			minV = e.Value
		}
		if e.Value > maxV {
			maxV = e.Value
		}
	}

	if minT.Equal(maxT) {
		maxT = minT.Add(time.Second)
	}
	if minV == maxV {
		minV -= 1
		maxV += 1
	}

	sp.minTime = minT
	sp.maxTime = maxT

	// Clear stale selection on data change
	sig := fmt.Sprintf("%d:%s:%s:%.4f:%.4f", len(filtered),
		minT.Format(time.RFC3339Nano), maxT.Format(time.RFC3339Nano), minV, maxV)
	if sig != sp.lastSig {
		sp.SelectionStart = -1
		sp.SelectionEnd = -1
		sp.lastSig = sig
	}

	minX := float64(minT.UnixMilli())
	maxX := float64(maxT.UnixMilli())

	yLabelFmt := func(_ int, v float64) string {
		absV := math.Abs(v)
		switch {
		case absV >= 10000:
			return fmt.Sprintf("%.0f", v)
		case absV >= 100:
			return fmt.Sprintf("%.0f", v)
		case absV >= 1:
			return fmt.Sprintf("%.1f", v)
		default:
			return fmt.Sprintf("%.2f", v)
		}
	}

	lc := linechart.New(cw, chartHeight, minX, maxX, minV, maxV,
		linechart.WithXYSteps(0, 2),
		linechart.WithYLabelFormatter(yLabelFmt),
		linechart.WithStyles(
			lipgloss.NewStyle().Foreground(styles.ColorMuted),
			lipgloss.NewStyle().Foreground(styles.ColorMuted),
			lipgloss.NewStyle().Foreground(styles.ColorBar),
		),
	)

	for _, idx := range filtered {
		e := &ms.Events[idx]
		x := float64(e.Timestamp.UnixMilli())
		lc.DrawRune(canvas.Float64Point{X: x, Y: e.Value}, '·')
	}

	lc.DrawXYAxisAndLabel()

	sp.graphWidth = lc.GraphWidth()
	sp.graphOriX = lc.Origin().X

	if sp.graphWidth > 0 && sp.Cursor >= sp.graphWidth {
		sp.Cursor = sp.graphWidth - 1
	}

	chartStr := lc.View()

	// X-axis line aligned to graph area
	axisStyle := lipgloss.NewStyle().Foreground(styles.ColorMuted)
	var axisRow strings.Builder
	if sp.graphOriX > 0 {
		axisRow.WriteString(strings.Repeat(" ", sp.graphOriX+1))
	}
	axisRow.WriteString(axisStyle.Render(strings.Repeat("─", sp.graphWidth)))

	// Cursor row aligned to graph area
	cursorBarStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(styles.ColorPrimary)
	selectedBarStyle := lipgloss.NewStyle().Foreground(styles.ColorAccent)

	var cursorRow strings.Builder
	if sp.graphOriX > 0 {
		cursorRow.WriteString(strings.Repeat(" ", sp.graphOriX+1))
	}
	for col := 0; col < sp.graphWidth; col++ {
		if col == sp.Cursor && sp.Focused {
			cursorRow.WriteString(cursorBarStyle.Render("▲"))
		} else if sp.inSelection(col) {
			cursorRow.WriteString(selectedBarStyle.Render("─"))
		} else {
			cursorRow.WriteString(" ")
		}
	}

	// Time labels aligned to graph area
	startLabel := minT.Format("15:04:05")
	endLabel := maxT.Format("15:04:05")
	labelPad := sp.graphWidth - len(startLabel) - len(endLabel)
	if labelPad < 1 {
		labelPad = 1
	}
	var timeRow strings.Builder
	if sp.graphOriX > 0 {
		timeRow.WriteString(strings.Repeat(" ", sp.graphOriX+1))
	}
	timeRow.WriteString(startLabel)
	timeRow.WriteString(strings.Repeat(" ", labelPad))
	timeRow.WriteString(endLabel)

	// Title with context
	var b strings.Builder
	titleParts := fmt.Sprintf("Scatter (%.1f - %.1f, %d pts)", minV, maxV, len(filtered))
	if sp.Focused && sp.graphWidth > 0 {
		titleParts += fmt.Sprintf("  cursor: %s", sp.CursorTime())
	}
	if sp.SelectionStart >= 0 && sp.SelectionEnd >= 0 {
		s, e := sp.SelectedTimeRange()
		titleParts += fmt.Sprintf("  selected: %s-%s", s.Format("15:04:05"), e.Format("15:04:05"))
	} else if sp.SelectionStart >= 0 {
		titleParts += fmt.Sprintf("  start: %s (press Enter for end)", sp.CursorTime())
	}

	b.WriteString(styles.TitleStyle.Render(titleParts))
	b.WriteString("\n")
	b.WriteString(chartStr)
	b.WriteString("\n")
	b.WriteString(axisRow.String())
	b.WriteString("\n")
	b.WriteString(cursorRow.String())
	b.WriteString("\n")
	b.WriteString(timeRow.String())
	if sp.Focused {
		b.WriteString("\n")
		b.WriteString(styles.HelpStyle.Render("[←→] move  [Enter] mark start/end  [Esc] clear range"))
	}

	return panelStyle.Render(b.String())
}

func (sp *ScatterPanel) inSelection(col int) bool {
	if sp.SelectionStart < 0 {
		return false
	}
	if sp.SelectionEnd < 0 {
		return col == sp.SelectionStart
	}
	start, end := sp.SelectionStart, sp.SelectionEnd
	if start > end {
		start, end = end, start
	}
	return col >= start && col <= end
}
