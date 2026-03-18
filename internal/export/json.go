package export

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/afcollins/kbx/internal/store"
)

// ExportJSON writes the raw JSON of filtered events to the given file path.
func ExportJSON(s *store.EventStore, path string) (int, error) {
	f, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	indices := s.Filtered()
	f.WriteString("[\n")

	written := 0
	for i, idx := range indices {
		raw, err := s.ReadRawJSON(idx)
		if err != nil {
			return written, fmt.Errorf("reading event %d: %w", idx, err)
		}

		// Pretty-print the JSON
		var obj interface{}
		if err := json.Unmarshal(raw, &obj); err != nil {
			f.Write(raw)
		} else {
			pretty, _ := json.MarshalIndent(obj, "  ", "  ")
			f.WriteString("  ")
			f.Write(pretty)
		}

		if i < len(indices)-1 {
			f.WriteString(",")
		}
		f.WriteString("\n")
		written++
	}

	f.WriteString("]\n")
	return written, nil
}
