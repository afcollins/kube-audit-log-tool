package store

import (
	"sort"
	"time"

	"github.com/afcollins/kube-audit-log-tool/internal/audit"
)

type FacetCount struct {
	Value string
	Count int
}

type FilterSet struct {
	Verb       string
	Resource   string
	Namespace  string
	Username   string
	SourceIP   string
	UserAgent  string
	StatusCode int // 0 means no filter
	TimeStart  time.Time
	TimeEnd    time.Time
}

func (f FilterSet) IsEmpty() bool {
	return f.Verb == "" && f.Resource == "" && f.Namespace == "" &&
		f.Username == "" && f.SourceIP == "" && f.UserAgent == "" &&
		f.StatusCode == 0 && f.TimeStart.IsZero() && f.TimeEnd.IsZero()
}

type EventStore struct {
	Events    []audit.AuditEvent
	ReadPaths []string // per FileIndex, the path to read raw JSON from

	verbIdx      *InvertedIndex
	resourceIdx  *InvertedIndex
	namespaceIdx *InvertedIndex
	usernameIdx  *InvertedIndex
	sourceIPIdx  *InvertedIndex
	userAgentIdx *InvertedIndex
	statusIdx    *IntIndex

	filtered []int
	filters  FilterSet
}

func New() *EventStore {
	return &EventStore{
		verbIdx:      NewInvertedIndex(),
		resourceIdx:  NewInvertedIndex(),
		namespaceIdx: NewInvertedIndex(),
		usernameIdx:  NewInvertedIndex(),
		sourceIPIdx:  NewInvertedIndex(),
		userAgentIdx: NewInvertedIndex(),
		statusIdx:    NewIntIndex(),
	}
}

// Load ingests events from multiple parse results, merge-sorting by timestamp.
func (s *EventStore) Load(results []*audit.ParseResult) {
	s.ReadPaths = make([]string, len(results))
	for i, r := range results {
		s.ReadPaths[i] = r.ReadPath
	}

	// k-way merge sort by timestamp
	s.Events = mergeSort(results)

	// Build indexes
	for i := range s.Events {
		e := &s.Events[i]
		s.verbIdx.Add(e.Verb, i)
		s.resourceIdx.Add(e.Resource, i)
		s.namespaceIdx.Add(e.Namespace, i)
		s.usernameIdx.Add(e.Username, i)
		s.sourceIPIdx.Add(e.SourceIP, i)
		s.userAgentIdx.Add(e.UserAgent, i)
		s.statusIdx.Add(e.StatusCode, i)
	}

	// Initial filtered = all
	s.filtered = make([]int, len(s.Events))
	for i := range s.filtered {
		s.filtered[i] = i
	}
}

func mergeSort(results []*audit.ParseResult) []audit.AuditEvent {
	if len(results) == 1 {
		return results[0].Events
	}

	total := 0
	for _, r := range results {
		total += len(r.Events)
	}

	merged := make([]audit.AuditEvent, 0, total)
	cursors := make([]int, len(results))

	for {
		bestFile := -1
		var bestTime time.Time

		for fi, r := range results {
			if cursors[fi] >= len(r.Events) {
				continue
			}
			t := r.Events[cursors[fi]].Timestamp
			if bestFile == -1 || t.Before(bestTime) {
				bestFile = fi
				bestTime = t
			}
		}

		if bestFile == -1 {
			break
		}

		merged = append(merged, results[bestFile].Events[cursors[bestFile]])
		cursors[bestFile]++
	}

	return merged
}

func (s *EventStore) SetFilters(f FilterSet) {
	s.filters = f
	s.refilter()
}

func (s *EventStore) Filters() FilterSet {
	return s.filters
}

func (s *EventStore) ToggleFilter(field, value string) {
	switch field {
	case "verb":
		if s.filters.Verb == value {
			s.filters.Verb = ""
		} else {
			s.filters.Verb = value
		}
	case "resource":
		if s.filters.Resource == value {
			s.filters.Resource = ""
		} else {
			s.filters.Resource = value
		}
	case "namespace":
		if s.filters.Namespace == value {
			s.filters.Namespace = ""
		} else {
			s.filters.Namespace = value
		}
	case "username":
		if s.filters.Username == value {
			s.filters.Username = ""
		} else {
			s.filters.Username = value
		}
	case "sourceip":
		if s.filters.SourceIP == value {
			s.filters.SourceIP = ""
		} else {
			s.filters.SourceIP = value
		}
	case "useragent":
		if s.filters.UserAgent == value {
			s.filters.UserAgent = ""
		} else {
			s.filters.UserAgent = value
		}
	}
	s.refilter()
}

func (s *EventStore) ToggleStatusFilter(code int) {
	if s.filters.StatusCode == code {
		s.filters.StatusCode = 0
	} else {
		s.filters.StatusCode = code
	}
	s.refilter()
}

func (s *EventStore) ClearFilters() {
	s.filters = FilterSet{}
	s.refilter()
}

func (s *EventStore) refilter() {
	if s.filters.IsEmpty() {
		s.filtered = make([]int, len(s.Events))
		for i := range s.filtered {
			s.filtered[i] = i
		}
		return
	}

	var candidates []int

	// Start with the most selective index if a filter is set
	candidates = s.allIndices()

	result := make([]int, 0, len(candidates))
	for _, i := range candidates {
		if s.matchesFilters(i) {
			result = append(result, i)
		}
	}

	s.filtered = result
}

func (s *EventStore) allIndices() []int {
	indices := make([]int, len(s.Events))
	for i := range indices {
		indices[i] = i
	}
	return indices
}

func (s *EventStore) matchesFilters(i int) bool {
	e := &s.Events[i]
	f := &s.filters

	if f.Verb != "" && e.Verb != f.Verb {
		return false
	}
	if f.Resource != "" && e.Resource != f.Resource {
		return false
	}
	if f.Namespace != "" && e.Namespace != f.Namespace {
		return false
	}
	if f.Username != "" && e.Username != f.Username {
		return false
	}
	if f.SourceIP != "" && e.SourceIP != f.SourceIP {
		return false
	}
	if f.UserAgent != "" && e.UserAgent != f.UserAgent {
		return false
	}
	if f.StatusCode != 0 && e.StatusCode != f.StatusCode {
		return false
	}
	if !f.TimeStart.IsZero() && e.Timestamp.Before(f.TimeStart) {
		return false
	}
	if !f.TimeEnd.IsZero() && e.Timestamp.After(f.TimeEnd) {
		return false
	}
	return true
}

// Filtered returns the current filtered event indices.
func (s *EventStore) Filtered() []int {
	return s.filtered
}

// FilteredCount returns the count of filtered events.
func (s *EventStore) FilteredCount() int {
	return len(s.filtered)
}

// TotalCount returns the total number of events.
func (s *EventStore) TotalCount() int {
	return len(s.Events)
}

// TopN returns the top N facet counts for the given field from filtered events.
func (s *EventStore) TopN(field string, n int) []FacetCount {
	counts := make(map[string]int)

	for _, i := range s.filtered {
		e := &s.Events[i]
		var val string
		switch field {
		case "verb":
			val = e.Verb
		case "resource":
			val = e.Resource
		case "namespace":
			val = e.Namespace
		case "username":
			val = e.Username
		case "sourceip":
			val = e.SourceIP
		case "useragent":
			val = e.UserAgent
		case "status":
			val = statusString(e.StatusCode)
		default:
			continue
		}
		counts[val]++
	}

	result := make([]FacetCount, 0, len(counts))
	for v, c := range counts {
		result = append(result, FacetCount{Value: v, Count: c})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Count > result[j].Count
	})

	if n > 0 && len(result) > n {
		result = result[:n]
	}
	return result
}

func statusString(code int) string {
	if code == 0 {
		return "unknown"
	}
	// Simple int to string without importing strconv in this helper
	s := ""
	c := code
	for c > 0 {
		s = string(rune('0'+c%10)) + s
		c /= 10
	}
	return s
}

// Timeline returns histogram buckets for the filtered events.
func (s *EventStore) Timeline(buckets int) []TimelineBucket {
	return BuildTimeline(s.Events, s.filtered, buckets)
}

// ReadRawJSON returns the raw JSON for an event.
func (s *EventStore) ReadRawJSON(eventIndex int) ([]byte, error) {
	e := &s.Events[eventIndex]
	return audit.ReadRawJSON(s.ReadPaths[e.FileIndex], e.FileOffset, e.LineLength)
}
