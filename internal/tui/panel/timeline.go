package panel

import (
	"fmt"
	"strings"

	"github.com/afcollins/kube-audit-log-tool/internal/store"
	"github.com/afcollins/kube-audit-log-tool/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
)

type TimelinePanel struct {
	Width   int
	Height  int
	Focused bool
}

func NewTimelinePanel() *TimelinePanel {
	return &TimelinePanel{Height: 8}
}

func (tp *TimelinePanel) View(s *store.EventStore) string {
	barWidth := tp.Width - 26 // leave room for timestamps
	if barWidth < 10 {
		barWidth = 10
	}

	buckets := s.Timeline(barWidth)
	if len(buckets) == 0 {
		style := styles.PanelStyle.Width(tp.Width - 2)
		if tp.Focused {
			style = styles.FocusedPanelStyle.Width(tp.Width - 2)
		}
		return style.Render(styles.TitleStyle.Render("Timeline") + "\n(no data)")
	}

	maxCount := 0
	for _, b := range buckets {
		if b.Count > maxCount {
			maxCount = b.Count
		}
	}

	barHeight := tp.Height - 4 // title + time labels + border
	if barHeight < 3 {
		barHeight = 3
	}

	barStyle := lipgloss.NewStyle().Foreground(styles.ColorBar)

	// Build rows from top to bottom
	var rows []string
	for row := barHeight; row >= 1; row-- {
		threshold := float64(row) / float64(barHeight) * float64(maxCount)
		var line strings.Builder
		for _, b := range buckets {
			if float64(b.Count) >= threshold {
				line.WriteString(barStyle.Render(styles.BarCharFull))
			} else {
				line.WriteString(" ")
			}
		}
		rows = append(rows, line.String())
	}

	// Time labels
	startLabel := buckets[0].Start.Format("15:04:05")
	endLabel := buckets[len(buckets)-1].End.Format("15:04:05")
	padding := barWidth - len(startLabel) - len(endLabel)
	if padding < 1 {
		padding = 1
	}
	timeRow := startLabel + strings.Repeat(" ", padding) + endLabel

	var b strings.Builder
	b.WriteString(styles.TitleStyle.Render(fmt.Sprintf("Timeline (max: %d/bucket)", maxCount)))
	b.WriteString("\n")
	b.WriteString(strings.Join(rows, "\n"))
	b.WriteString("\n")
	b.WriteString(timeRow)

	style := styles.PanelStyle.Width(tp.Width - 2)
	if tp.Focused {
		style = styles.FocusedPanelStyle.Width(tp.Width - 2)
	}

	return style.Render(b.String())
}
