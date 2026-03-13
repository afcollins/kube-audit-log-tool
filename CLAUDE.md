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

Interactive TUI for exploring Kubernetes API server audit logs. Built on the Charm ecosystem (bubbletea, lipgloss, bubbles).

### Data Flow

1. **Parse** (`internal/audit`): Streams JSON-lines files (`.log` or `.log.gz`) line-by-line. Extracts only indexed fields into `AuditEvent` structs, recording each event's file offset and line length for later raw JSON retrieval. Gzip files are decompressed to temp files first.
2. **Store** (`internal/store`): `EventStore` holds all events in a flat slice, builds inverted indexes (string→[]eventIndex) for each facet field, and maintains a `filtered` slice of matching indices. `FilterSet` defines the active filters; `refilter()` recomputes on every change.
3. **Render** (`internal/tui` + `internal/tui/panel`): Root `Model` in `tui/app.go` orchestrates state (file picker → loading → dashboard). Panel types (Facet, Timeline, EventList, EventDetail, FilterBar, FilePicker) each own their own rendering and cursor state.

### Key Design Decisions

- **No raw JSON in memory**: Events store only extracted fields. Raw JSON is re-read from disk via `ReadRawJSON(path, offset, length)` when the user opens event detail. This keeps memory usage low for large log files.
- **Styles in separate subpackage**: `tui/styles` exists to break an import cycle — both `tui` and `tui/panel` need shared styles/constants.
- **Focus model**: Integer-based focus index (0-5 = facet panels, 6 = timeline, 7 = event list). Secondary facets (index 4-5) are hidden by default, toggled with 'f'.
- **Facet panels are generic**: Each `FacetPanel` takes a field name string that maps to both `EventStore.TopN()` lookups and `ToggleFilter()` calls.

## Module

`github.com/afcollins/kube-audit-log-tool` — Go 1.24
