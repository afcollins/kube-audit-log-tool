package panel

import (
	"fmt"
	"strings"

	"github.com/afcollins/kube-audit-log-tool/internal/metrics"
	"github.com/afcollins/kube-audit-log-tool/internal/mstore"
	"github.com/afcollins/kube-audit-log-tool/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
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

	header := formatMetricRow("TIME", "METRIC", "VALUE", "NAMESPACE", "NODE", "POD", contentWidth)
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(styles.ColorSecondary).Render(header))
	b.WriteString("\n")

	visibleLines := ml.visibleLines()
	if visibleLines < 1 {
		visibleLines = 1
	}

	end := ml.Scroll + visibleLines
	if end > len(indices) {
		end = len(indices)
	}

	for i := ml.Scroll; i < end; i++ {
		e := &s.Events[indices[i]]
		valStr := fmt.Sprintf("%.4g", e.Value)

		line := formatMetricRow(
			e.Timestamp.Format("15:04:05"),
			e.MetricName,
			valStr,
			truncate(e.Labels["namespace"], 15),
			truncate(e.Labels["node"], 20),
			truncate(e.Labels["pod"], 20),
			contentWidth,
		)

		if i == ml.Cursor && ml.Focused {
			line = styles.SelectedStyle.Render(line)
		}

		b.WriteString(line)
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	if len(indices) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(styles.ColorMuted).Render("(no matching metrics)"))
	}

	style := styles.PanelStyle.Width(ml.Width - 2)
	if ml.Focused {
		style = styles.FocusedPanelStyle.Width(ml.Width - 2)
	}

	return style.Height(ml.Height - 2).Render(b.String())
}

func formatMetricRow(time, metric, value, namespace, node, pod string, width int) string {
	return fmt.Sprintf("%-10s %-24s %-12s %-15s %-20s %s",
		truncate(time, 10),
		truncate(metric, 24),
		truncate(value, 12),
		truncate(namespace, 15),
		truncate(node, 20),
		truncate(pod, 20),
	)
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
