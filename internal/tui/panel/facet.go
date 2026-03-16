package panel

import (
	"fmt"
	"strings"

	"github.com/afcollins/kube-audit-log-tool/internal/store"
	"github.com/afcollins/kube-audit-log-tool/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
)

const DefaultTopN = 15

// FacetSource provides data for facet panels. Implemented by both EventStore and MetricStore.
type FacetSource interface {
	TopN(field string, n int) []store.FacetCount
	FilterValue(field string) string
}

type FacetPanel struct {
	Title    string
	Field    string // store field name: "verb", "resource", "username", "status", "sourceip", "useragent"
	Items    []store.FacetCount
	Cursor   int
	Focused  bool
	Width    int
	Height   int
	Scroll   int
	Selected string // currently filtered value
	MaxItems int    // override DefaultTopN when set (e.g. maximized view)
}

func NewFacetPanel(title, field string) *FacetPanel {
	return &FacetPanel{
		Title:  title,
		Field:  field,
		Width:  20,
		Height: styles.FacetPanelHeight,
	}
}

func (p *FacetPanel) Update(s FacetSource) {
	topN := DefaultTopN
	if p.MaxItems > 0 {
		topN = p.MaxItems
	}
	p.Items = s.TopN(p.Field, topN)
	p.Selected = s.FilterValue(p.Field)
}

func (p *FacetPanel) MoveUp() {
	if p.Cursor > 0 {
		p.Cursor--
		if p.Cursor < p.Scroll {
			p.Scroll = p.Cursor
		}
	}
}

func (p *FacetPanel) MoveDown() {
	if p.Cursor < len(p.Items)-1 {
		p.Cursor++
		visibleLines := p.visibleLines()
		if p.Cursor >= p.Scroll+visibleLines {
			p.Scroll = p.Cursor - visibleLines + 1
		}
	}
}

func (p *FacetPanel) visibleLines() int {
	return p.Height - 3 // title + border padding
}

func (p *FacetPanel) SelectedValue() string {
	if p.Cursor < len(p.Items) {
		return p.Items[p.Cursor].Value
	}
	return ""
}

func (p *FacetPanel) View() string {
	var b strings.Builder

	title := styles.TitleStyle.Render(p.Title)
	b.WriteString(title)
	b.WriteString("\n")

	visibleLines := p.visibleLines()
	if visibleLines < 1 {
		visibleLines = 1
	}

	maxVal := 0
	for _, item := range p.Items {
		if item.Count > maxVal {
			maxVal = item.Count
		}
	}

	end := p.Scroll + visibleLines
	if end > len(p.Items) {
		end = len(p.Items)
	}

	contentWidth := p.Width - 4 // account for border + padding

	for i := p.Scroll; i < end; i++ {
		item := p.Items[i]

		// Truncate value to fit
		val := item.Value
		if val == "" {
			val = "(empty)"
		}
		countStr := fmt.Sprintf("%d", item.Count)

		// available space: contentWidth - countStr - 2 (for spacing)
		maxNameWidth := contentWidth - len(countStr) - 2
		if maxNameWidth < 5 {
			maxNameWidth = 5
		}
		if len(val) > maxNameWidth {
			val = val[:maxNameWidth-1] + "…"
		}

		line := fmt.Sprintf("%-*s %s", maxNameWidth, val, countStr)

		if item.Value == p.Selected && p.Selected != "" {
			line = styles.FilteredStyle.Render(line)
		} else if i == p.Cursor && p.Focused {
			line = styles.SelectedStyle.Render(line)
		}

		b.WriteString(line)
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	if len(p.Items) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(styles.ColorMuted).Render("(no data)"))
	}

	style := styles.PanelStyle.Width(p.Width - 2) // -2 for border
	if p.Focused {
		style = styles.FocusedPanelStyle.Width(p.Width - 2)
	}

	return style.Height(p.Height - 2).Render(b.String())
}
