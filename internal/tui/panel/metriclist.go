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

	// Compute column widths from data (scan all events, not just filtered)
	colWidths := ml.computeColumnWidths(s, contentWidth)

	header := formatMetricRow("TIME", "METRIC", "VALUE", "NAMESPACE", "NODE", "POD", colWidths)
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
			e.Labels["namespace"],
			e.Labels["node"],
			e.Labels["pod"],
			colWidths,
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

// computeColumnWidths scans all events to find the max width needed for each
// dynamic column (namespace, node, pod), then fits them within available space.
func (ml *MetricListPanel) computeColumnWidths(s *mstore.MetricStore, contentWidth int) [6]int {
	const (
		timeW   = 10
		valueW  = 12
		gaps    = 5 // spaces between 6 columns
		minColW = 3
	)

	// Find max data widths for each dynamic column
	maxMetric, maxNS, maxNode, maxPod := len("METRIC"), len("NAMESPACE"), len("NODE"), len("POD")
	for i := range s.Events {
		e := &s.Events[i]
		if l := len(e.MetricName); l > maxMetric {
			maxMetric = l
		}
		if l := len(e.Labels["namespace"]); l > maxNS {
			maxNS = l
		}
		if l := len(e.Labels["node"]); l > maxNode {
			maxNode = l
		}
		if l := len(e.Labels["pod"]); l > maxPod {
			maxPod = l
		}
	}

	// Start with ideal widths plus padding, then shrink to fit
	const colPad = 2
	metricW := maxMetric + colPad
	nsW := maxNS + colPad
	nodeW := maxNode + colPad
	podW := maxPod

	available := contentWidth - timeW - valueW - gaps
	total := metricW + nsW + nodeW + podW

	if total > available {
		// Scale proportionally
		scale := float64(available) / float64(total)
		metricW = max(minColW, int(float64(metricW)*scale))
		nsW = max(minColW, int(float64(nsW)*scale))
		nodeW = max(minColW, int(float64(nodeW)*scale))
		podW = max(minColW, available-metricW-nsW-nodeW)
		if podW < minColW {
			podW = minColW
		}
	}

	return [6]int{timeW, metricW, valueW, nsW, nodeW, podW}
}

func formatMetricRow(time, metric, value, namespace, node, pod string, w [6]int) string {
	return fmt.Sprintf("%-*s %-*s %-*s %-*s %-*s %-*s",
		w[0], truncate(time, w[0]),
		w[1], truncate(metric, w[1]),
		w[2], truncate(value, w[2]),
		w[3], truncate(namespace, w[3]),
		w[4], truncate(node, w[4]),
		w[5], truncate(pod, w[5]),
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
