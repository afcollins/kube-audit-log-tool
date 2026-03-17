package mstore

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/afcollins/kube-audit-log-tool/internal/metrics"
	"github.com/afcollins/kube-audit-log-tool/internal/store"
)

// Default primary fields shown without toggling secondary facets.
var DefaultPrimaryFields = []string{"namespace", "node", "pod", "container"}

type MetricStore struct {
	Events     []metrics.MetricEvent
	RawItems   []json.RawMessage
	JobSummary map[string]any

	fieldIndexes  map[string]*store.InvertedIndex
	FieldNames    []string // all discovered facet-able field names, sorted
	PrimaryFields []string

	filtered  []int
	filters   map[string]string
	timeStart time.Time
	timeEnd   time.Time
}

func New() *MetricStore {
	return &MetricStore{
		fieldIndexes:  make(map[string]*store.InvertedIndex),
		PrimaryFields: DefaultPrimaryFields,
		filters:       make(map[string]string),
	}
}

// Load ingests events from multiple parse results, merge-sorting by timestamp.
func (s *MetricStore) Load(results []*metrics.ParseResult) {
	// Collect all events and raw items
	var allEvents []metrics.MetricEvent
	var allRaw []json.RawMessage

	for _, r := range results {
		allEvents = append(allEvents, r.Events...)
		allRaw = append(allRaw, r.RawItems...)
		if r.JobSummary != nil {
			s.JobSummary = r.JobSummary
		}
	}

	// Sort by timestamp
	sort.SliceStable(allEvents, func(i, j int) bool {
		return allEvents[i].Timestamp.Before(allEvents[j].Timestamp)
	})

	s.Events = allEvents
	s.RawItems = allRaw

	// Discover fields and build indexes
	fieldSet := make(map[string]bool)
	for i := range s.Events {
		e := &s.Events[i]
		s.addToIndex("metricName", e.MetricName, i)
		fieldSet["metricName"] = true

		s.addToIndex("uuid", e.UUID, i)
		fieldSet["uuid"] = true

		s.addToIndex("jobName", e.JobName, i)
		fieldSet["jobName"] = true

		for k, v := range e.Labels {
			s.addToIndex(k, v, i)
			fieldSet[k] = true
		}
	}

	// Sort field names
	s.FieldNames = make([]string, 0, len(fieldSet))
	for k := range fieldSet {
		s.FieldNames = append(s.FieldNames, k)
	}
	sort.Strings(s.FieldNames)

	// Initialize filtered = all
	s.filtered = make([]int, len(s.Events))
	for i := range s.filtered {
		s.filtered[i] = i
	}
}

func (s *MetricStore) addToIndex(field, value string, eventIndex int) {
	idx, ok := s.fieldIndexes[field]
	if !ok {
		idx = store.NewInvertedIndex()
		s.fieldIndexes[field] = idx
	}
	idx.Add(value, eventIndex)
}

func (s *MetricStore) fieldValue(e *metrics.MetricEvent, field string) string {
	switch field {
	case "metricName":
		return e.MetricName
	case "uuid":
		return e.UUID
	case "jobName":
		return e.JobName
	default:
		return e.Labels[field]
	}
}

// TopN returns the top N facet counts for the given field from filtered events.
func (s *MetricStore) TopN(field string, n int) []store.FacetCount {
	counts := make(map[string]int)
	for _, i := range s.filtered {
		val := s.fieldValue(&s.Events[i], field)
		if val != "" {
			counts[val]++
		}
	}

	result := make([]store.FacetCount, 0, len(counts))
	for v, c := range counts {
		result = append(result, store.FacetCount{Value: v, Count: c})
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Count != result[j].Count {
			return result[i].Count > result[j].Count
		}
		return result[i].Value < result[j].Value
	})

	if n > 0 && len(result) > n {
		result = result[:n]
	}
	return result
}

// FilterValue returns the active filter value for a field, or "" if none.
func (s *MetricStore) FilterValue(field string) string {
	return s.filters[field]
}

// ActiveFilters returns all active filters as a map.
func (s *MetricStore) ActiveFilters() map[string]string {
	return s.filters
}

func (s *MetricStore) ToggleFilter(field, value string) {
	if s.filters[field] == value {
		delete(s.filters, field)
	} else {
		s.filters[field] = value
	}
	s.refilter()
}

func (s *MetricStore) ClearFilters() {
	s.filters = make(map[string]string)
	s.timeStart = time.Time{}
	s.timeEnd = time.Time{}
	s.refilter()
}

func (s *MetricStore) SetTimeFilter(start, end time.Time) {
	s.timeStart = start
	s.timeEnd = end
	s.refilter()
}

func (s *MetricStore) ClearTimeFilter() {
	s.timeStart = time.Time{}
	s.timeEnd = time.Time{}
	s.refilter()
}

func (s *MetricStore) TimeStart() time.Time { return s.timeStart }
func (s *MetricStore) TimeEnd() time.Time   { return s.timeEnd }

func (s *MetricStore) refilter() {
	noFieldFilters := len(s.filters) == 0
	noTimeFilters := s.timeStart.IsZero() && s.timeEnd.IsZero()

	if noFieldFilters && noTimeFilters {
		s.filtered = make([]int, len(s.Events))
		for i := range s.filtered {
			s.filtered[i] = i
		}
		return
	}

	result := make([]int, 0, len(s.Events))
	for i := range s.Events {
		if s.matchesFilters(i) {
			result = append(result, i)
		}
	}
	s.filtered = result
}

func (s *MetricStore) matchesFilters(i int) bool {
	e := &s.Events[i]

	for field, val := range s.filters {
		if s.fieldValue(e, field) != val {
			return false
		}
	}

	if !s.timeStart.IsZero() && e.Timestamp.Before(s.timeStart) {
		return false
	}
	if !s.timeEnd.IsZero() && e.Timestamp.After(s.timeEnd) {
		return false
	}
	return true
}

func (s *MetricStore) Filtered() []int     { return s.filtered }
func (s *MetricStore) FilteredCount() int  { return len(s.filtered) }
func (s *MetricStore) TotalCount() int     { return len(s.Events) }

// Timeline returns histogram buckets for the filtered events.
func (s *MetricStore) Timeline(buckets int) []store.TimelineBucket {
	return store.BuildTimelineFunc(
		func(i int) time.Time { return s.Events[i].Timestamp },
		s.filtered, buckets,
	)
}

// TimelineAll returns histogram buckets for all events (ignoring filters).
func (s *MetricStore) TimelineAll(buckets int) []store.TimelineBucket {
	all := make([]int, len(s.Events))
	for i := range all {
		all[i] = i
	}
	return store.BuildTimelineFunc(
		func(i int) time.Time { return s.Events[i].Timestamp },
		all, buckets,
	)
}

// ReadRawJSON returns the raw JSON for a metric event.
func (s *MetricStore) ReadRawJSON(eventIndex int) ([]byte, error) {
	e := &s.Events[eventIndex]
	if e.ArrayIndex < len(s.RawItems) {
		return s.RawItems[e.ArrayIndex], nil
	}
	return nil, nil
}

// VisibleFields returns fields that have more than 1 unique value in filtered events.
func (s *MetricStore) VisibleFields() []string {
	var visible []string
	for _, field := range s.FieldNames {
		counts := s.TopN(field, 2)
		if len(counts) > 1 {
			visible = append(visible, field)
		}
	}
	return visible
}

// IsPrimary returns whether a field is in the primary fields list.
func (s *MetricStore) IsPrimary(field string) bool {
	for _, f := range s.PrimaryFields {
		if f == field {
			return true
		}
	}
	return false
}
