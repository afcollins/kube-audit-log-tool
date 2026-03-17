package panel

import (
	"github.com/afcollins/kube-audit-log-tool/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

// newListTable creates a borderless lipgloss table pre-configured for use
// inside an event or metric list panel.
func newListTable() *table.Table {
	return table.New().
		Wrap(false).
		BorderTop(false).
		BorderBottom(false).
		BorderLeft(false).
		BorderRight(false).
		BorderColumn(false).
		BorderHeader(false).
		BorderRow(false)
}

// columnWidthCache holds pre-computed column widths so they remain stable
// across renders regardless of which rows are currently visible.
type columnWidthCache struct {
	widths  []int
	dataLen int // number of data rows used to compute widths
	tableW  int // table width used to compute widths
}

// computeColumnWidths scans all data values to find the max width per column,
// then distributes available space proportionally. The result is cached and
// reused until the data length or table width changes.
func (c *columnWidthCache) computeColumnWidths(headers []string, allValues func(col int) []string, totalWidth int, dataLen int) []int {
	if c.widths != nil && c.dataLen == dataLen && c.tableW == totalWidth {
		return c.widths
	}

	nCols := len(headers)
	maxW := make([]int, nCols)
	for i, h := range headers {
		maxW[i] = len(h)
	}

	for col := 0; col < nCols; col++ {
		for _, v := range allValues(col) {
			if l := len(v); l > maxW[col] {
				maxW[col] = l
			}
		}
	}

	// Each column's Width in lipgloss includes padding. With PaddingRight(1),
	// a column needs maxDataWidth + 1 to display content without truncation.
	const cellPad = 1
	ideal := make([]int, nCols)
	total := 0
	for i, w := range maxW {
		ideal[i] = w + cellPad
		total += ideal[i]
	}

	widths := make([]int, nCols)
	if total <= totalWidth {
		copy(widths, ideal)
	} else {
		scale := float64(totalWidth) / float64(total)
		used := 0
		for i := range widths {
			if i == nCols-1 {
				widths[i] = totalWidth - used
			} else {
				w := int(float64(ideal[i]) * scale)
				if w < 3 {
					w = 3
				}
				widths[i] = w
				used += w
			}
		}
	}

	c.widths = widths
	c.dataLen = dataLen
	c.tableW = totalWidth
	return widths
}

func listStyleWithWidth(base lipgloss.Style, widths []int, col int) lipgloss.Style {
	if col < len(widths) {
		return base.Width(widths[col])
	}
	return base
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
