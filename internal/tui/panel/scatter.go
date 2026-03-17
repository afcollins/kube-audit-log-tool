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

// Scatter panel configuration constants.
const (
	histWidth      = 22   // character width of the value histogram (including separator)
	histShowLabels = true // show count labels on histogram bars
)

type ScatterPanel struct {
	Width          int
	Height         int
	Focused        bool
	Cursor         int // X-axis (time) cursor
	SelectionStart int // time selection start
	SelectionEnd   int // time selection end

	ValueCursor   int // Y-axis cursor (0 = minV, valueSteps-1 = maxV)
	ValueSelStart int // value selection start (-1 = unset)
	ValueSelEnd   int // value selection end (-1 = unset)

	graphWidth  int
	graphOriX   int
	chartHeight int
	minTime     time.Time
	maxTime     time.Time
	minValue    float64
	maxValue    float64
	lastSig     string
}

func NewScatterPanel() *ScatterPanel {
	return &ScatterPanel{
		Height:         28,
		SelectionStart: -1,
		SelectionEnd:   -1,
		ValueSelStart:  -1,
		ValueSelEnd:    -1,
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

func (sp *ScatterPanel) MoveUp() {
	max := sp.chartHeight - 1
	if max < 1 {
		max = 1
	}
	if sp.ValueCursor < max {
		sp.ValueCursor++
	}
}

func (sp *ScatterPanel) MoveDown() {
	if sp.ValueCursor > 0 {
		sp.ValueCursor--
	}
}

// CursorValue returns the Y value at the current ValueCursor position.
func (sp *ScatterPanel) CursorValue() float64 {
	steps := sp.chartHeight - 1
	if steps < 1 {
		steps = 1
	}
	return sp.minValue + (sp.maxValue-sp.minValue)*float64(sp.ValueCursor)/float64(steps)
}

// stepValue returns the Y value at a given step position.
func (sp *ScatterPanel) stepValue(step int) float64 {
	steps := sp.chartHeight - 1
	if steps < 1 {
		steps = 1
	}
	return sp.minValue + (sp.maxValue-sp.minValue)*float64(step)/float64(steps)
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

func (sp *ScatterPanel) MarkValueSelection() bool {
	if sp.ValueSelStart == -1 {
		sp.ValueSelStart = sp.ValueCursor
		sp.ValueSelEnd = -1
		return false
	}
	sp.ValueSelEnd = sp.ValueCursor
	if sp.ValueSelStart > sp.ValueSelEnd {
		sp.ValueSelStart, sp.ValueSelEnd = sp.ValueSelEnd, sp.ValueSelStart
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

func (sp *ScatterPanel) SelectedValueRange() (float64, float64) {
	if sp.ValueSelStart < 0 || sp.ValueSelEnd < 0 {
		return 0, 0
	}
	return sp.stepValue(sp.ValueSelStart), sp.stepValue(sp.ValueSelEnd)
}

func (sp *ScatterPanel) ClearSelection() {
	sp.SelectionStart = -1
	sp.SelectionEnd = -1
}

func (sp *ScatterPanel) ClearValueSelection() {
	sp.ValueSelStart = -1
	sp.ValueSelEnd = -1
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
	sp.chartHeight = sp.Height - 7
	if sp.chartHeight < 3 {
		sp.chartHeight = 3
	}
	chartHeight := sp.chartHeight

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
	sp.minValue = minV
	sp.maxValue = maxV

	// Clear stale selections on data change
	sig := fmt.Sprintf("%d:%s:%s:%.4f:%.4f", len(filtered),
		minT.Format(time.RFC3339Nano), maxT.Format(time.RFC3339Nano), minV, maxV)
	if sig != sp.lastSig {
		sp.SelectionStart = -1
		sp.SelectionEnd = -1
		sp.ValueSelStart = -1
		sp.ValueSelEnd = -1
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

	// Allocate width: chart gets the remainder, histogram gets fixed width
	chartCW := cw - histWidth
	if chartCW < 15 {
		chartCW = 15
	}

	lc := linechart.New(chartCW, chartHeight, minX, maxX, minV, maxV,
		linechart.WithXYSteps(0, 2),
		linechart.WithYLabelFormatter(yLabelFmt),
		linechart.WithStyles(
			lipgloss.NewStyle().Foreground(styles.ColorMuted),
			lipgloss.NewStyle().Foreground(styles.ColorMuted),
			lipgloss.NewStyle().Foreground(styles.ColorBar),
		),
	)

	sp.graphWidth = lc.GraphWidth()
	sp.graphOriX = lc.Origin().X

	if sp.graphWidth > 0 && sp.Cursor >= sp.graphWidth {
		sp.Cursor = sp.graphWidth - 1
	}
	if sp.ValueCursor >= chartHeight {
		sp.ValueCursor = chartHeight - 1
	}

	// Plot data points
	for _, idx := range filtered {
		e := &ms.Events[idx]
		x := float64(e.Timestamp.UnixMilli())
		lc.DrawRune(canvas.Float64Point{X: x, Y: e.Value}, '·')
	}

	// Draw value selection band (shaded region between two Y boundaries)
	if sp.Focused && sp.ValueSelStart >= 0 {
		bandStyle := lipgloss.NewStyle().Foreground(styles.ColorAccent)
		lo := sp.ValueSelStart
		hi := lo
		if sp.ValueSelEnd >= 0 {
			hi = sp.ValueSelEnd
		}
		loVal := sp.stepValue(lo)
		hiVal := sp.stepValue(hi)
		for i := 0; i <= sp.graphWidth; i++ {
			x := minX + (maxX-minX)*float64(i)/float64(sp.graphWidth)
			lc.DrawRuneWithStyle(canvas.Float64Point{X: x, Y: loVal}, '─', bandStyle)
			if hi != lo {
				lc.DrawRuneWithStyle(canvas.Float64Point{X: x, Y: hiVal}, '─', bandStyle)
			}
		}
	}

	// Draw horizontal cursor line at valueCursor position
	if sp.Focused {
		cursorLineStyle := lipgloss.NewStyle().Foreground(styles.ColorPrimary)
		cursorY := sp.CursorValue()
		for i := 0; i <= sp.graphWidth; i++ {
			x := minX + (maxX-minX)*float64(i)/float64(sp.graphWidth)
			lc.DrawRuneWithStyle(canvas.Float64Point{X: x, Y: cursorY}, '─', cursorLineStyle)
		}
	}

	lc.DrawXYAxisAndLabel()
	chartStr := lc.View()

	// Build value histogram
	histLines := sp.buildHistogram(filtered, ms, chartHeight, minV, maxV)

	// Join chart lines with histogram lines
	chartLines := strings.Split(chartStr, "\n")
	var joinedChart strings.Builder
	for i, line := range chartLines {
		joinedChart.WriteString(line)
		if i < len(histLines) {
			joinedChart.WriteString(histLines[i])
		}
		if i < len(chartLines)-1 {
			joinedChart.WriteString("\n")
		}
	}

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
	if sp.Focused {
		titleParts += fmt.Sprintf("  Y: %.2f", sp.CursorValue())
	}
	if sp.ValueSelStart >= 0 && sp.ValueSelEnd >= 0 {
		vMin, vMax := sp.SelectedValueRange()
		titleParts += fmt.Sprintf("  band: %.2f-%.2f", vMin, vMax)
	} else if sp.ValueSelStart >= 0 {
		titleParts += fmt.Sprintf("  band start: %.2f (press v for end)", sp.stepValue(sp.ValueSelStart))
	}
	if sp.SelectionStart >= 0 && sp.SelectionEnd >= 0 {
		s, e := sp.SelectedTimeRange()
		titleParts += fmt.Sprintf("  T: %s-%s", s.Format("15:04:05"), e.Format("15:04:05"))
	} else if sp.SelectionStart >= 0 {
		titleParts += fmt.Sprintf("  T start: %s", sp.CursorTime())
	}

	b.WriteString(styles.TitleStyle.Render(titleParts))
	b.WriteString("\n")
	b.WriteString(joinedChart.String())
	b.WriteString("\n")
	b.WriteString(axisRow.String())
	b.WriteString("\n")
	b.WriteString(cursorRow.String())
	b.WriteString("\n")
	b.WriteString(timeRow.String())
	if sp.Focused {
		b.WriteString("\n")
		b.WriteString(styles.HelpStyle.Render("[←→] time  [↑↓] value  [Enter] time range  [v] value range  [Esc] clear"))
	}

	return panelStyle.Render(b.String())
}

// buildHistogram creates histogram lines aligned to the chart rows.
// One bucket per row, with bars and right-aligned count labels.
func (sp *ScatterPanel) buildHistogram(filtered []int, ms *mstore.MetricStore, chartHeight int, minV, maxV float64) []string {
	nBuckets := chartHeight
	vRange := maxV - minV

	// Count events per bucket
	bucketCounts := make([]int, nBuckets)
	for _, idx := range filtered {
		e := &ms.Events[idx]
		bi := int((e.Value - minV) / vRange * float64(nBuckets))
		if bi >= nBuckets {
			bi = nBuckets - 1
		}
		if bi < 0 {
			bi = 0
		}
		bucketCounts[bi]++
	}

	maxCount := 0
	for _, c := range bucketCounts {
		if c > maxCount {
			maxCount = c
		}
	}

	// Label width from max count
	labelWidth := len(fmt.Sprintf("%d", maxCount))

	// Bar area: histWidth - 1(separator) - 1(space) - labelWidth
	maxBarWidth := histWidth - 2 - labelWidth
	if maxBarWidth < 1 {
		maxBarWidth = 1
	}

	barStyle := lipgloss.NewStyle().Foreground(styles.ColorBar)
	sepStyle := lipgloss.NewStyle().Foreground(styles.ColorMuted)
	labelStyle := lipgloss.NewStyle().Foreground(styles.ColorMuted)

	// Row 0 = top (maxV), row chartHeight-1 = bottom (minV)
	// Bucket 0 = minV, bucket nBuckets-1 = maxV
	lines := make([]string, chartHeight)
	for r := 0; r < chartHeight; r++ {
		bi := chartHeight - 1 - r
		count := bucketCounts[bi]

		barLen := 0
		if maxCount > 0 {
			barLen = count * maxBarWidth / maxCount
		}

		var line strings.Builder
		line.WriteString(sepStyle.Render("│"))
		if barLen > 0 {
			line.WriteString(barStyle.Render(strings.Repeat(styles.BarCharFull, barLen)))
		}
		pad := maxBarWidth - barLen
		line.WriteString(strings.Repeat(" ", pad))
		if histShowLabels {
			line.WriteString(labelStyle.Render(fmt.Sprintf(" %*d", labelWidth, count)))
		}

		lines[r] = line.String()
	}

	return lines
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
