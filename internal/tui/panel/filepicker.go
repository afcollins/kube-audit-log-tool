package panel

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/afcollins/kube-audit-log-tool/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
)

type FilePickerPanel struct {
	Dir      string
	Files    []string
	Cursor   int
	Scroll   int
	Selected map[string]bool
	Width    int
	Height   int
}

func NewFilePickerPanel() *FilePickerPanel {
	dir, _ := os.Getwd()
	fp := &FilePickerPanel{
		Dir:      dir,
		Selected: make(map[string]bool),
		Width:    80,
		Height:   20,
	}
	fp.Refresh()
	return fp
}

func (fp *FilePickerPanel) Refresh() {
	fp.Files = nil
	entries, err := os.ReadDir(fp.Dir)
	if err != nil {
		return
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".log") || strings.HasSuffix(name, ".log.gz") ||
			strings.HasSuffix(name, ".json") || strings.HasSuffix(name, ".json.gz") {
			fp.Files = append(fp.Files, name)
		}
	}
	sort.Strings(fp.Files)
}

func (fp *FilePickerPanel) MoveUp() {
	if fp.Cursor > 0 {
		fp.Cursor--
		if fp.Cursor < fp.Scroll {
			fp.Scroll = fp.Cursor
		}
	}
}

func (fp *FilePickerPanel) MoveDown() {
	if fp.Cursor < len(fp.Files)-1 {
		fp.Cursor++
		vis := fp.Height - 6
		if fp.Cursor >= fp.Scroll+vis {
			fp.Scroll = fp.Cursor - vis + 1
		}
	}
}

func (fp *FilePickerPanel) ToggleSelection() {
	if fp.Cursor < len(fp.Files) {
		name := fp.Files[fp.Cursor]
		if fp.Selected[name] {
			delete(fp.Selected, name)
		} else {
			fp.Selected[name] = true
		}
	}
}

func (fp *FilePickerPanel) SelectedPaths() []string {
	var paths []string
	for name := range fp.Selected {
		paths = append(paths, filepath.Join(fp.Dir, name))
	}
	sort.Strings(paths)
	return paths
}

func (fp *FilePickerPanel) View() string {
	var b strings.Builder

	b.WriteString(styles.TitleStyle.Render("Select audit log or metrics files"))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(styles.ColorMuted).Render(
		fmt.Sprintf("Dir: %s", fp.Dir)))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(styles.ColorMuted).Render(
		"[Space] toggle  [Enter] load selected  [q] quit"))
	b.WriteString("\n\n")

	vis := fp.Height - 6
	if vis < 1 {
		vis = 1
	}
	end := fp.Scroll + vis
	if end > len(fp.Files) {
		end = len(fp.Files)
	}

	for i := fp.Scroll; i < end; i++ {
		name := fp.Files[i]
		prefix := "  "
		if fp.Selected[name] {
			prefix = lipgloss.NewStyle().Foreground(styles.ColorSuccess).Render("* ")
		}

		line := prefix + name
		if i == fp.Cursor {
			line = styles.SelectedStyle.Render(line)
		}

		b.WriteString(line)
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	if len(fp.Files) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(styles.ColorDanger).Render(
			"No .log, .log.gz, .json, or .json.gz files found in current directory"))
	}

	sel := len(fp.Selected)
	if sel > 0 {
		b.WriteString(fmt.Sprintf("\n\n%d file(s) selected", sel))
	}

	return lipgloss.NewStyle().Padding(2, 4).Render(b.String())
}
