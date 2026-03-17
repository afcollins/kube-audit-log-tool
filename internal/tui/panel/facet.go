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

	Searching   bool   // true when search input is active
	SearchQuery string // current search substring
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
	if p.SearchQuery != "" {
		topN = 0 // fetch all values when searching
	}
	p.Items = s.TopN(p.Field, topN)
	p.Selected = s.FilterValue(p.Field)
}

func (p *FacetPanel) MoveUp() {
	items := p.filteredItems()
	if p.Cursor > 0 {
		p.Cursor--
		if p.Cursor < p.Scroll {
			p.Scroll = p.Cursor
		}
	}
	_ = items
}

func (p *FacetPanel) MoveDown() {
	items := p.filteredItems()
	if p.Cursor < len(items)-1 {
		p.Cursor++
		visibleLines := p.visibleLines()
		if p.Cursor >= p.Scroll+visibleLines {
			p.Scroll = p.Cursor - visibleLines + 1
		}
	}
}

func (p *FacetPanel) visibleLines() int {
	h := p.Height - 3 // title + border padding
	if p.Searching {
		h-- // search input row
	}
	return h
}

func (p *FacetPanel) SelectedValue() string {
	items := p.filteredItems()
	if p.Cursor < len(items) {
		return items[p.Cursor].Value
	}
	return ""
}

// StartSearch enters search mode.
func (p *FacetPanel) StartSearch() {
	p.Searching = true
	p.SearchQuery = ""
	p.Cursor = 0
	p.Scroll = 0
}

// StopSearch exits search mode, keeping the current query as a filter.
func (p *FacetPanel) StopSearch() {
	p.Searching = false
}

// ClearSearch exits search mode and clears the query.
func (p *FacetPanel) ClearSearch() {
	p.Searching = false
	p.SearchQuery = ""
	p.Cursor = 0
	p.Scroll = 0
}

// HandleSearchKey processes a key during search mode. Returns true if the key
// was consumed (callers should not process it further).
func (p *FacetPanel) HandleSearchKey(key string) bool {
	switch key {
	case "esc":
		p.ClearSearch()
		return true
	case "enter":
		p.StopSearch()
		return true
	case "backspace":
		if len(p.SearchQuery) > 0 {
			p.SearchQuery = p.SearchQuery[:len(p.SearchQuery)-1]
			p.Cursor = 0
			p.Scroll = 0
		}
		return true
	default:
		// Only accept printable single characters
		if len(key) == 1 && key[0] >= 32 && key[0] <= 126 {
			p.SearchQuery += key
			p.Cursor = 0
			p.Scroll = 0
			return true
		}
		// Allow navigation keys to pass through
		if key == "up" || key == "k" || key == "down" || key == "j" {
			return false
		}
		return true
	}
}

// filteredItems returns Items filtered by SearchQuery (case-insensitive substring match).
func (p *FacetPanel) filteredItems() []store.FacetCount {
	if p.SearchQuery == "" {
		return p.Items
	}
	query := strings.ToLower(p.SearchQuery)
	var result []store.FacetCount
	for _, item := range p.Items {
		if strings.Contains(strings.ToLower(item.Value), query) {
			result = append(result, item)
		}
	}
	return result
}

func (p *FacetPanel) View() string {
	var b strings.Builder

	title := styles.TitleStyle.Render(p.Title)
	b.WriteString(title)
	b.WriteString("\n")

	// Show search input when active
	if p.Searching {
		searchStyle := lipgloss.NewStyle().Foreground(styles.ColorAccent)
		b.WriteString(searchStyle.Render("/" + p.SearchQuery + "█"))
		b.WriteString("\n")
	} else if p.SearchQuery != "" {
		searchStyle := lipgloss.NewStyle().Foreground(styles.ColorMuted)
		b.WriteString(searchStyle.Render("/" + p.SearchQuery))
		b.WriteString("\n")
	}

	items := p.filteredItems()

	visibleLines := p.visibleLines()
	if visibleLines < 1 {
		visibleLines = 1
	}

	end := p.Scroll + visibleLines
	if end > len(items) {
		end = len(items)
	}

	contentWidth := p.Width - 4 // account for border + padding

	for i := p.Scroll; i < end; i++ {
		item := items[i]

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

	if len(items) == 0 && p.SearchQuery != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(styles.ColorMuted).Render("(no matches)"))
	} else if len(items) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(styles.ColorMuted).Render("(no data)"))
	}

	style := styles.PanelStyle.Width(p.Width - 2) // -2 for border
	if p.Focused {
		style = styles.FocusedPanelStyle.Width(p.Width - 2)
	}

	return style.Height(p.Height - 2).Render(b.String())
}
