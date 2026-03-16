package metrics

import "time"

// MetricEvent represents a single normalized metric measurement.
// For podLatencyMeasurement files, each latency field is exploded into a separate event.
type MetricEvent struct {
	Timestamp  time.Time
	MetricName string            // "containerCPU", "schedulingLatency", etc.
	UUID       string
	JobName    string
	Labels     map[string]string // merged from labels{} object + flat fields
	Value      float64
	Metadata   map[string]string // ocpVersion, ocpMajorVersion
	FileIndex  int
	ArrayIndex int // position in original JSON array
}
