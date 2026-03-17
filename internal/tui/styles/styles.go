package styles

import "github.com/charmbracelet/lipgloss"

var (
	ColorPrimary   = lipgloss.Color("#7D56F4")
	ColorSecondary = lipgloss.Color("#4B8BBE")
	ColorAccent    = lipgloss.Color("#F7DC6F")
	ColorMuted     = lipgloss.Color("#888888")
	ColorDanger    = lipgloss.Color("#E74C3C")
	ColorSuccess   = lipgloss.Color("#2ECC71")
	ColorBar       = lipgloss.Color("#5DADE2")

	PanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorMuted).
			Padding(0, 1)

	FocusedPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorPrimary).
				Padding(0, 1)

	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary)

	SelectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(ColorPrimary)

	FilteredStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorAccent)

	FilterBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#333333")).
			Padding(0, 1)

	FilterTagStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#000000")).
			Background(ColorAccent).
			Padding(0, 1)

	StatusBarStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	HelpStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	BarCharFull  = "█"
	BarCharEmpty = "░"
)

// Fixed-height elements.
const (
	FilterBarHeight = 2
	StatusBarHeight = 2
)

// Proportional height ratios for the three main sections.
// These are relative weights — the available height (after filter/status bars)
// is divided among them.
const (
	FacetHeightRatio    = 25 // percentage of available height per facet row
	TimelineHeightRatio = 40 // percentage of available height for timeline/scatter
	ListHeightRatio     = 35 // percentage of available height for event/metric list
)

// MinFacetPanelHeight prevents facets from becoming unusably small.
const MinFacetPanelHeight = 6
