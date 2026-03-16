package panel

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/afcollins/kube-audit-log-tool/internal/store"
	"github.com/afcollins/kube-audit-log-tool/internal/tui/styles"
)

type FilterBar struct {
	Width int
}

func NewFilterBar() *FilterBar {
	return &FilterBar{}
}

func (fb *FilterBar) View(s *store.EventStore) string {
	f := s.Filters()
	var tags []string

	if f.Verb != "" {
		tags = append(tags, styles.FilterTagStyle.Render("verb:"+f.Verb))
	}
	if f.Resource != "" {
		tags = append(tags, styles.FilterTagStyle.Render("resource:"+f.Resource))
	}
	if f.Namespace != "" {
		tags = append(tags, styles.FilterTagStyle.Render("ns:"+f.Namespace))
	}
	if f.Username != "" {
		tags = append(tags, styles.FilterTagStyle.Render("user:"+f.Username))
	}
	if f.SourceIP != "" {
		tags = append(tags, styles.FilterTagStyle.Render("ip:"+f.SourceIP))
	}
	if f.UserAgent != "" {
		ua := f.UserAgent
		if len(ua) > 30 {
			ua = ua[:30] + "…"
		}
		tags = append(tags, styles.FilterTagStyle.Render("ua:"+ua))
	}
	if f.StatusCode != 0 {
		tags = append(tags, styles.FilterTagStyle.Render(fmt.Sprintf("status:%d", f.StatusCode)))
	}
	if !f.TimeStart.IsZero() {
		tags = append(tags, styles.FilterTagStyle.Render("from:"+f.TimeStart.Format("15:04:05")))
	}
	if !f.TimeEnd.IsZero() {
		tags = append(tags, styles.FilterTagStyle.Render("to:"+f.TimeEnd.Format("15:04:05")))
	}

	if len(tags) == 0 {
		return styles.FilterBarStyle.Width(fb.Width).Render(
			fmt.Sprintf(" %d events (no filters)", s.TotalCount()),
		)
	}

	countInfo := fmt.Sprintf(" %d/%d events  ", s.FilteredCount(), s.TotalCount())
	return styles.FilterBarStyle.Width(fb.Width).Render(
		countInfo + strings.Join(tags, " ") + "  [c]lear",
	)
}

// ViewMetrics renders the filter bar for metrics mode using dynamic filters.
func (fb *FilterBar) ViewMetrics(filters map[string]string, timeStart, timeEnd time.Time, filtered, total int) string {
	var tags []string

	// Sort filter keys for stable rendering
	keys := make([]string, 0, len(filters))
	for k := range filters {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := filters[k]
		if len(v) > 30 {
			v = v[:30] + "…"
		}
		tags = append(tags, styles.FilterTagStyle.Render(k+":"+v))
	}
	if !timeStart.IsZero() {
		tags = append(tags, styles.FilterTagStyle.Render("from:"+timeStart.Format("15:04:05")))
	}
	if !timeEnd.IsZero() {
		tags = append(tags, styles.FilterTagStyle.Render("to:"+timeEnd.Format("15:04:05")))
	}

	if len(tags) == 0 {
		return styles.FilterBarStyle.Width(fb.Width).Render(
			fmt.Sprintf(" %d metrics (no filters)", total),
		)
	}

	countInfo := fmt.Sprintf(" %d/%d metrics  ", filtered, total)
	return styles.FilterBarStyle.Width(fb.Width).Render(
		countInfo + strings.Join(tags, " ") + "  [c]lear",
	)
}
