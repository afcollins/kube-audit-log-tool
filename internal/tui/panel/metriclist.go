package panel

import (
	"fmt"
	"strings"

	"github.com/afcollins/kube-audit-log-tool/internal/metrics"
	"github.com/afcollins/kube-audit-log-tool/internal/mstore"
	"github.com/afcollins/kube-audit-log-tool/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

type MetricListPanel struct {
	Width   int
	Height  int
	Focused bool
	Cursor  int
	Scroll  int
}

func NewMetricListPanel() *MetricListPanel {
	return &MetricListPanel{Height: 15}
}

func (ml *MetricListPanel) MoveUp() {
	if ml.Cursor > 0 {
		ml.Cursor--
		if ml.Cursor < ml.Scroll {
			ml.Scroll = ml.Cursor
		}
	}
}

func (ml *MetricListPanel) MoveDown(maxItems int) {
	if ml.Cursor < maxItems-1 {
		ml.Cursor++
		visible := ml.visibleLines()
		if ml.Cursor >= ml.Scroll+visible {
			ml.Scroll = ml.Cursor - visible + 1
		}
	}
}

func (ml *MetricListPanel) visibleLines() int {
	return ml.Height - 4
}

func (ml *MetricListPanel) SelectedIndex(s *mstore.MetricStore) int {
	indices := s.Filtered()
	if ml.Cursor < len(indices) {
		return indices[ml.Cursor]
	}
	return -1
}

func (ml *MetricListPanel) ResetCursor() {
	ml.Cursor = 0
	ml.Scroll = 0
}

func (ml *MetricListPanel) View(s *mstore.MetricStore) string {
	indices := s.Filtered()

	var b strings.Builder
	b.WriteString(styles.TitleStyle.Render(fmt.Sprintf("Metrics (%d)", len(indices))))
	b.WriteString("\n")

	contentWidth := ml.Width - 4

	visibleLines := ml.visibleLines()
	if visibleLines < 1 {
		visibleLines = 1
	}

	end := ml.Scroll + visibleLines
	if end > len(indices) {
		end = len(indices)
	}

	if len(indices) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(styles.ColorMuted).Render("(no matching metrics)"))
	} else {
		rows := make([][]string, 0, end-ml.Scroll)
		for i := ml.Scroll; i < end; i++ {
			e := &s.Events[indices[i]]
			rows = append(rows, []string{
				e.Timestamp.Format("15:04:05"),
				e.MetricName,
				fmt.Sprintf("%.4g", e.Value),
				e.Labels["namespace"],
				e.Labels["node"],
				e.Labels["pod"],
			})
		}

		scroll := ml.Scroll
		cursor := ml.Cursor
		focused := ml.Focused

		t := newListTable(contentWidth).
			Headers("TIME", "METRIC", "VALUE", "NAMESPACE", "NODE", "POD").
			Rows(rows...).
			StyleFunc(func(row, col int) lipgloss.Style {
				if row == table.HeaderRow {
					return listHeaderStyle()
				}
				actualIdx := scroll + row
				if actualIdx == cursor && focused {
					return listSelectedStyle()
				}
				return listCellStyle()
			})

		b.WriteString(t.Render())
	}

	style := styles.PanelStyle.Width(ml.Width - 2)
	if ml.Focused {
		style = styles.FocusedPanelStyle.Width(ml.Width - 2)
	}

	return style.Height(ml.Height - 2).Render(b.String())
}

// FormatMetricDetail returns a pretty-printed detail of a metric event.
func FormatMetricDetail(e *metrics.MetricEvent) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Timestamp:   %s\n", e.Timestamp.Format("2006-01-02 15:04:05.000")))
	b.WriteString(fmt.Sprintf("Metric:      %s\n", e.MetricName))
	b.WriteString(fmt.Sprintf("Value:       %g\n", e.Value))
	b.WriteString(fmt.Sprintf("UUID:        %s\n", e.UUID))
	b.WriteString(fmt.Sprintf("Job:         %s\n", e.JobName))
	b.WriteString("\nLabels:\n")
	for k, v := range e.Labels {
		b.WriteString(fmt.Sprintf("  %-12s %s\n", k+":", v))
	}
	if len(e.Metadata) > 0 {
		b.WriteString("\nMetadata:\n")
		for k, v := range e.Metadata {
			b.WriteString(fmt.Sprintf("  %-20s %s\n", k+":", v))
		}
	}
	return b.String()
}
