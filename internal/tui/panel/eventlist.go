package panel

import (
	"fmt"
	"strings"

	"github.com/afcollins/kube-audit-log-tool/internal/audit"
	"github.com/afcollins/kube-audit-log-tool/internal/store"
	"github.com/afcollins/kube-audit-log-tool/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

type EventListPanel struct {
	Width   int
	Height  int
	Focused bool
	Cursor  int
	Scroll  int
}

func NewEventListPanel() *EventListPanel {
	return &EventListPanel{Height: 15}
}

func (el *EventListPanel) MoveUp() {
	if el.Cursor > 0 {
		el.Cursor--
		if el.Cursor < el.Scroll {
			el.Scroll = el.Cursor
		}
	}
}

func (el *EventListPanel) MoveDown(maxItems int) {
	if el.Cursor < maxItems-1 {
		el.Cursor++
		visible := el.visibleLines()
		if el.Cursor >= el.Scroll+visible {
			el.Scroll = el.Cursor - visible + 1
		}
	}
}

func (el *EventListPanel) visibleLines() int {
	return el.Height - 4 // title + header + border
}

func (el *EventListPanel) SelectedIndex(s *store.EventStore) int {
	indices := s.Filtered()
	if el.Cursor < len(indices) {
		return indices[el.Cursor]
	}
	return -1
}

func (el *EventListPanel) ResetCursor() {
	el.Cursor = 0
	el.Scroll = 0
}

func (el *EventListPanel) View(s *store.EventStore) string {
	indices := s.Filtered()

	var b strings.Builder
	b.WriteString(styles.TitleStyle.Render(fmt.Sprintf("Events (%d)", len(indices))))
	b.WriteString("\n")

	contentWidth := el.Width - 4

	visibleLines := el.visibleLines()
	if visibleLines < 1 {
		visibleLines = 1
	}

	end := el.Scroll + visibleLines
	if end > len(indices) {
		end = len(indices)
	}

	if len(indices) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(styles.ColorMuted).Render("(no matching events)"))
	} else {
		rows := make([][]string, 0, end-el.Scroll)
		for i := el.Scroll; i < end; i++ {
			e := &s.Events[indices[i]]
			rows = append(rows, []string{
				e.Timestamp.Format("15:04:05"),
				e.Verb,
				e.Resource,
				e.Namespace,
				e.Username,
				fmt.Sprintf("%d", e.StatusCode),
			})
		}

		scroll := el.Scroll
		cursor := el.Cursor
		focused := el.Focused
		events := s.Events
		filteredIndices := indices

		t := newListTable(contentWidth).
			Headers("TIME", "VERB", "RESOURCE", "NAMESPACE", "USER", "CODE").
			Rows(rows...).
			StyleFunc(func(row, col int) lipgloss.Style {
				if row == table.HeaderRow {
					return listHeaderStyle()
				}
				actualIdx := scroll + row
				if actualIdx == cursor && focused {
					return listSelectedStyle()
				}
				if actualIdx < len(filteredIndices) {
					e := &events[filteredIndices[actualIdx]]
					if e.StatusCode >= 400 {
						return listDangerStyle()
					}
				}
				return listCellStyle()
			})

		b.WriteString(t.Render())
	}

	style := styles.PanelStyle.Width(el.Width - 2)
	if el.Focused {
		style = styles.FocusedPanelStyle.Width(el.Width - 2)
	}

	return style.Height(el.Height - 2).Render(b.String())
}

// FormatEventDetail returns a pretty-printed detail of an event for the modal.
func FormatEventDetail(e *audit.AuditEvent) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Timestamp:  %s\n", e.Timestamp.Format("2006-01-02 15:04:05.000")))
	b.WriteString(fmt.Sprintf("Verb:       %s\n", e.Verb))
	b.WriteString(fmt.Sprintf("Resource:   %s\n", e.Resource))
	b.WriteString(fmt.Sprintf("API Group:  %s\n", e.APIGroup))
	b.WriteString(fmt.Sprintf("Version:    %s\n", e.APIVersion))
	b.WriteString(fmt.Sprintf("Namespace:  %s\n", e.Namespace))
	b.WriteString(fmt.Sprintf("User:       %s\n", e.Username))
	b.WriteString(fmt.Sprintf("Source IP:  %s\n", e.SourceIP))
	b.WriteString(fmt.Sprintf("User Agent: %s\n", e.UserAgent))
	b.WriteString(fmt.Sprintf("Status:     %d\n", e.StatusCode))
	return b.String()
}
