package panel

import (
	"github.com/afcollins/kube-audit-log-tool/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

// newListTable creates a borderless lipgloss table pre-configured for use
// inside an event or metric list panel. The caller adds rows and may wrap
// the result with StyleFunc for cursor/error highlighting.
func newListTable(width int) *table.Table {
	return table.New().
		Width(width).
		Wrap(false).
		BorderTop(false).
		BorderBottom(false).
		BorderLeft(false).
		BorderRight(false).
		BorderColumn(false).
		BorderHeader(false).
		BorderRow(false)
}

// listHeaderStyle returns the style used for header cells.
func listHeaderStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.ColorSecondary).
		PaddingRight(1)
}

// listCellStyle returns the base style for data cells.
func listCellStyle() lipgloss.Style {
	return lipgloss.NewStyle().PaddingRight(1)
}

// listSelectedStyle returns the style for the selected/cursor row.
func listSelectedStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(styles.ColorPrimary).
		PaddingRight(1)
}

// listDangerStyle returns the style for error rows.
func listDangerStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(styles.ColorDanger).
		PaddingRight(1)
}
