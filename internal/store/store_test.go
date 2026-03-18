package store

import (
	"testing"

	"github.com/afcollins/kbx/internal/audit"
)

func loadTestStore(t *testing.T) *EventStore {
	t.Helper()
	result, err := audit.ParseFile("../../sample_events.log", 0)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	s := New()
	s.Load([]*audit.ParseResult{result})
	return s
}

func TestStoreLoad(t *testing.T) {
	s := loadTestStore(t)

	if s.TotalCount() != 5 {
		t.Fatalf("expected 5 total events, got %d", s.TotalCount())
	}
	if s.FilteredCount() != 5 {
		t.Fatalf("expected 5 filtered events, got %d", s.FilteredCount())
	}
}

func TestTopN(t *testing.T) {
	s := loadTestStore(t)

	verbs := s.TopN("verb", 10)
	if len(verbs) != 1 {
		t.Fatalf("expected 1 unique verb, got %d", len(verbs))
	}
	if verbs[0].Value != "list" || verbs[0].Count != 5 {
		t.Errorf("expected verb=list count=5, got %q count=%d", verbs[0].Value, verbs[0].Count)
	}

	statuses := s.TopN("status", 10)
	if len(statuses) != 2 {
		t.Fatalf("expected 2 unique statuses, got %d", len(statuses))
	}
}

func TestFiltering(t *testing.T) {
	s := loadTestStore(t)

	// Filter by status 503
	s.ToggleStatusFilter(503)
	if s.FilteredCount() != 2 {
		t.Errorf("expected 2 events with status 503, got %d", s.FilteredCount())
	}

	// Check that TopN reflects the filter
	resources := s.TopN("resource", 10)
	for _, r := range resources {
		if r.Value == "csidrivers" || r.Value == "leases" {
			t.Errorf("resource %q should not appear in 503-filtered results", r.Value)
		}
	}

	// Clear and verify
	s.ClearFilters()
	if s.FilteredCount() != 5 {
		t.Errorf("expected 5 after clear, got %d", s.FilteredCount())
	}
}

func TestToggleFilter(t *testing.T) {
	s := loadTestStore(t)

	s.ToggleFilter("resource", "leases")
	if s.FilteredCount() != 2 {
		t.Errorf("expected 2 events with resource=leases, got %d", s.FilteredCount())
	}

	// Toggle same value again should clear it
	s.ToggleFilter("resource", "leases")
	if s.FilteredCount() != 5 {
		t.Errorf("expected 5 after toggling off, got %d", s.FilteredCount())
	}
}

func TestTimeline(t *testing.T) {
	s := loadTestStore(t)

	buckets := s.Timeline(10)
	if len(buckets) == 0 {
		t.Error("expected non-empty timeline")
	}

	total := 0
	for _, b := range buckets {
		total += b.Count
	}
	if total != 5 {
		t.Errorf("expected timeline total=5, got %d", total)
	}
}
