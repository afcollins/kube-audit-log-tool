# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Run

```bash
go build -o kube-audit-log-tool .
go run . [file1.log file2.log.gz ...]   # launches TUI; no args shows file picker
```

## Testing

```bash
go test ./...                            # run all tests
go test ./internal/store/               # run tests for a specific package
go test ./internal/audit/ -run TestName  # run a single test
```

There is no Makefile, linter config, or CI pipeline. Use standard `go vet ./...` for linting.

## Architecture

Interactive TUI for exploring Kubernetes API server audit logs and metrics. Built on the Charm ecosystem (bubbletea, lipgloss, bubbles) plus ntcharts for scatter plots.

### Data Flow

1. **Parse**: `internal/audit` streams JSON-lines (.log/.log.gz); `internal/metrics` parses JSON arrays (.json/.json.gz). Mode auto-detected from file extensions.
2. **Store**: `internal/store.EventStore` for audit events; `internal/mstore.MetricStore` for metrics. Both use inverted indexes, filtered slices, and `refilter()` on change. MetricStore supports time, value, and facet filters.
3. **Render** (`internal/tui` + `internal/tui/panel`): Root `Model` in `tui/app.go` orchestrates state. Panel types: Facet, Timeline, ScatterPanel, EventList, MetricList, EventDetail, FilterBar, FilePicker.

### Key Design Decisions

- **No raw JSON in memory** (audit): Re-read from disk via file offset/length. Metrics store raw JSON (small files).
- **Styles in `tui/styles`**: Breaks import cycle between `tui` and `tui/panel`.
- **Focus model**: Integer-based focus index. Audit: 0-5 facets, 6 timeline, 7 event list. Metrics: 0..N facets, N scatter, N+1 metric list.
- **Facet panels are generic**: `FacetPanel` takes a field name; both stores implement `FacetSource`.
- **Facet sort stability**: `TopN()` sorts by count desc, then alphabetically by label to prevent reordering on cursor movement.

### Metrics-Specific Panels

- **ScatterPanel** (`panel/scatter.go`): Plots Value (Y) vs Timestamp (X) using ntcharts linechart. Supports time range selection (Enter), value band selection (v key), and inline value distribution histogram. Configurable constants at top: `histWidth`, `histShowLabels`.
- **MetricListPanel** (`panel/metriclist.go`): Auto-sizes columns (namespace, node, pod) to fit actual data widths from loaded events.
- **MetricStore** (`mstore/store.go`): Dynamic facet discovery. `metricName` is a secondary facet. Secondary facets render in a single evenly-spaced row.

### Dependencies

- `github.com/NimbleMarkets/ntcharts` — scatter chart rendering (linechart with DrawRune for individual points)

## Module

`github.com/afcollins/kube-audit-log-tool` — Go 1.24
