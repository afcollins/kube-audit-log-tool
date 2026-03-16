package metrics

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"strings"
	"time"
)

// ParseResult holds events from a single metrics file.
type ParseResult struct {
	Events     []MetricEvent
	RawItems   []json.RawMessage  // raw JSON per original array item
	ReadPath   string
	TempPath   string             // non-empty if a temp file was created (for cleanup)
	JobSummary map[string]any     // populated from jobSummary entries, nil otherwise
}

// rawMetric captures all known fields from both standard and podLatency formats.
type rawMetric struct {
	Timestamp  string            `json:"timestamp"`
	MetricName string            `json:"metricName"`
	UUID       string            `json:"uuid"`
	JobName    string            `json:"jobName"`
	Labels     map[string]string `json:"labels"`
	Value      *float64          `json:"value"`
	Query      string            `json:"query"`
	Metadata   map[string]string `json:"metadata"`

	// podLatency flat fields
	Namespace  string `json:"namespace"`
	PodName    string `json:"podName"`
	NodeName   string `json:"nodeName"`

	// podLatency value fields
	SchedulingLatency      *float64 `json:"schedulingLatency"`
	InitializedLatency     *float64 `json:"initializedLatency"`
	ContainersReadyLatency *float64 `json:"containersReadyLatency"`
	PodReadyLatency        *float64 `json:"podReadyLatency"`
}

// ParseFile parses a .json or .json.gz metrics file.
func ParseFile(path string, fileIndex int) (*ParseResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var reader io.Reader
	readPath := path
	var tempPath string

	if isGzip(path) {
		tmpFile, err := decompressToTemp(f)
		if err != nil {
			return nil, err
		}
		readPath = tmpFile
		tempPath = tmpFile
		tf, err := os.Open(tmpFile)
		if err != nil {
			return nil, err
		}
		defer tf.Close()
		reader = tf
	} else {
		reader = f
	}

	result, err := parseReader(reader, fileIndex)
	if err != nil {
		return nil, err
	}
	result.ReadPath = readPath
	result.TempPath = tempPath
	return result, nil
}

func isGzip(path string) bool {
	return strings.HasSuffix(path, ".gz")
}

func decompressToTemp(r io.Reader) (string, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return "", err
	}
	defer gz.Close()

	tmp, err := os.CreateTemp("", "kube-metrics-*.json")
	if err != nil {
		return "", err
	}
	defer tmp.Close()

	if _, err := io.Copy(tmp, gz); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	return tmp.Name(), nil
}

func parseReader(r io.Reader, fileIndex int) (*ParseResult, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	var rawItems []json.RawMessage
	if err := json.Unmarshal(data, &rawItems); err != nil {
		return nil, err
	}

	result := &ParseResult{
		RawItems: rawItems,
	}

	for arrayIdx, raw := range rawItems {
		var m rawMetric
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}

		if m.MetricName == "jobSummary" {
			var summary map[string]any
			if err := json.Unmarshal(raw, &summary); err == nil {
				result.JobSummary = summary
			}
			continue
		}

		ts, _ := time.Parse(time.RFC3339Nano, m.Timestamp)
		if ts.IsZero() {
			ts, _ = time.Parse("2006-01-02T15:04:05.000Z", m.Timestamp)
		}
		if ts.IsZero() {
			ts, _ = time.Parse("2006-01-02T15:04:05Z", m.Timestamp)
		}

		if m.MetricName == "podLatencyMeasurement" {
			events := explodePodLatency(m, ts, fileIndex, arrayIdx)
			result.Events = append(result.Events, events...)
			continue
		}

		// Standard metric (containerCPU, etc.)
		val := 0.0
		if m.Value != nil {
			val = *m.Value
		}

		labels := make(map[string]string)
		for k, v := range m.Labels {
			labels[k] = v
		}

		result.Events = append(result.Events, MetricEvent{
			Timestamp:  ts,
			MetricName: m.MetricName,
			UUID:       m.UUID,
			JobName:    m.JobName,
			Labels:     labels,
			Value:      val,
			Metadata:   m.Metadata,
			FileIndex:  fileIndex,
			ArrayIndex: arrayIdx,
		})
	}

	return result, nil
}

var podLatencyFields = []struct {
	Name string
	Get  func(*rawMetric) *float64
}{
	{"schedulingLatency", func(m *rawMetric) *float64 { return m.SchedulingLatency }},
	{"initializedLatency", func(m *rawMetric) *float64 { return m.InitializedLatency }},
	{"containersReadyLatency", func(m *rawMetric) *float64 { return m.ContainersReadyLatency }},
	{"podReadyLatency", func(m *rawMetric) *float64 { return m.PodReadyLatency }},
}

func explodePodLatency(m rawMetric, ts time.Time, fileIndex, arrayIdx int) []MetricEvent {
	labels := make(map[string]string)
	if m.Namespace != "" {
		labels["namespace"] = m.Namespace
	}
	if m.PodName != "" {
		labels["pod"] = m.PodName
	}
	if m.NodeName != "" {
		labels["node"] = m.NodeName
	}

	var events []MetricEvent
	for _, field := range podLatencyFields {
		val := field.Get(&m)
		if val == nil {
			continue
		}
		// Clone labels for each event
		l := make(map[string]string, len(labels))
		for k, v := range labels {
			l[k] = v
		}
		events = append(events, MetricEvent{
			Timestamp:  ts,
			MetricName: field.Name,
			UUID:       m.UUID,
			JobName:    m.JobName,
			Labels:     l,
			Value:      *val,
			Metadata:   m.Metadata,
			FileIndex:  fileIndex,
			ArrayIndex: arrayIdx,
		})
	}
	return events
}
