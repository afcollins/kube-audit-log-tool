package panel

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/afcollins/kube-audit-log-tool/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
)

type EventDetailPanel struct {
	Visible    bool
	Content    string
	RawJSON    string
	Width      int
	Height     int
	Scroll     int
	lines      []string
	ShowingRaw bool
}

func NewEventDetailPanel() *EventDetailPanel {
	return &EventDetailPanel{}
}

func (ed *EventDetailPanel) Show(summary string, rawJSON []byte) {
	ed.Visible = true
	ed.Scroll = 0
	ed.Content = summary

	// Pretty-print JSON
	var prettyBuf bytes.Buffer
	if err := json.Indent(&prettyBuf, rawJSON, "", "  "); err != nil {
		ed.RawJSON = string(rawJSON)
	} else {
		ed.RawJSON = prettyBuf.String()
	}

	ed.buildLines()
}

func (ed *EventDetailPanel) ToggleRaw() {
	ed.ShowingRaw = !ed.ShowingRaw
	ed.Scroll = 0
	ed.buildLines()
}

func (ed *EventDetailPanel) buildLines() {
	if ed.ShowingRaw {
		ed.lines = strings.Split(ed.RawJSON, "\n")
	} else {
		ed.lines = strings.Split(ed.Content, "\n")
	}
}

func (ed *EventDetailPanel) Hide() {
	ed.Visible = false
}

func (ed *EventDetailPanel) ScrollUp() {
	if ed.Scroll > 0 {
		ed.Scroll--
	}
}

func (ed *EventDetailPanel) ScrollDown() {
	maxScroll := len(ed.lines) - ed.visibleLines()
	if maxScroll < 0 {
		maxScroll = 0
	}
	if ed.Scroll < maxScroll {
		ed.Scroll++
	}
}

func (ed *EventDetailPanel) visibleLines() int {
	return ed.Height - 6
}

func (ed *EventDetailPanel) View() string {
	if !ed.Visible {
		return ""
	}

	modalWidth := ed.Width - 10
	if modalWidth < 40 {
		modalWidth = 40
	}
	modalHeight := ed.Height - 6
	if modalHeight < 10 {
		modalHeight = 10
	}

	var b strings.Builder

	mode := "Summary"
	if ed.ShowingRaw {
		mode = "Raw JSON"
	}
	b.WriteString(styles.TitleStyle.Render("Event Detail ["+mode+"]") + "  [↑↓] prev/next  [←→] scroll  [r]aw toggle  [Esc] close\n\n")

	visibleLines := modalHeight - 4
	end := ed.Scroll + visibleLines
	if end > len(ed.lines) {
		end = len(ed.lines)
	}

	for i := ed.Scroll; i < end; i++ {
		line := ed.lines[i]
		if len(line) > modalWidth-4 {
			line = line[:modalWidth-4]
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(styles.ColorPrimary).
		Padding(1, 2).
		Width(modalWidth).
		Height(modalHeight)

	modal := style.Render(b.String())

	// Center the modal
	hPad := (ed.Width - modalWidth - 4) / 2
	vPad := (ed.Height - modalHeight - 4) / 2
	if hPad < 0 {
		hPad = 0
	}
	if vPad < 0 {
		vPad = 0
	}

	return lipgloss.NewStyle().
		PaddingLeft(hPad).
		PaddingTop(vPad).
		Render(modal)
}
