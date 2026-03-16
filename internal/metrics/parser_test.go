package metrics

import (
	"testing"
)

const testdataDir = "../../testdata/collected-metrics-2178a534-fce2-4d66-839b-d874c01dc630/"

func TestParseContainerCPU(t *testing.T) {
	result, err := ParseFile(testdataDir+"containerCPU.json", 0)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	if len(result.Events) == 0 {
		t.Fatal("expected events, got 0")
	}

	e := result.Events[0]
	if e.MetricName != "containerCPU" {
		t.Errorf("MetricName = %q, want %q", e.MetricName, "containerCPU")
	}
	if e.UUID == "" {
		t.Error("UUID is empty")
	}
	if e.Value == 0 {
		t.Error("Value is 0")
	}
	if e.Labels["container"] == "" {
		t.Error("labels[container] is empty")
	}
	if e.Labels["namespace"] == "" {
		t.Error("labels[namespace] is empty")
	}
	if e.Timestamp.IsZero() {
		t.Error("Timestamp is zero")
	}
	if result.JobSummary != nil {
		t.Error("JobSummary should be nil for containerCPU file")
	}
}

func TestParseContainerCPUGzip(t *testing.T) {
	result, err := ParseFile(testdataDir+"containerCPU.json.gz", 0)
	if err != nil {
		t.Fatalf("ParseFile gzip: %v", err)
	}

	if len(result.Events) == 0 {
		t.Fatal("expected events from gzip, got 0")
	}
	if result.TempPath == "" {
		t.Error("TempPath should be set for gzip files")
	}

	e := result.Events[0]
	if e.MetricName != "containerCPU" {
		t.Errorf("MetricName = %q, want %q", e.MetricName, "containerCPU")
	}
}

func TestParsePodLatency(t *testing.T) {
	result, err := ParseFile(testdataDir+"podLatencyMeasurement-node-density.json", 0)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	if len(result.Events) == 0 {
		t.Fatal("expected events, got 0")
	}

	// Each original entry should explode into 4 events
	// Count by metricName
	counts := make(map[string]int)
	for _, e := range result.Events {
		counts[e.MetricName]++
	}

	expectedNames := []string{
		"schedulingLatency", "initializedLatency",
		"containersReadyLatency", "podReadyLatency",
	}
	for _, name := range expectedNames {
		if counts[name] == 0 {
			t.Errorf("expected events with metricName %q, got 0", name)
		}
	}

	// Verify labels are populated from flat fields
	e := result.Events[0]
	if e.Labels["namespace"] == "" {
		t.Error("labels[namespace] should be populated from flat namespace field")
	}
	if e.Labels["pod"] == "" {
		t.Error("labels[pod] should be populated from flat podName field")
	}
	if e.Labels["node"] == "" {
		t.Error("labels[node] should be populated from flat nodeName field")
	}
}

func TestParseJobSummary(t *testing.T) {
	result, err := ParseFile(testdataDir+"jobSummary.json", 0)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	if result.JobSummary == nil {
		t.Fatal("JobSummary should not be nil")
	}

	if result.JobSummary["metricName"] != "jobSummary" {
		t.Errorf("JobSummary metricName = %v, want jobSummary", result.JobSummary["metricName"])
	}

	// jobSummary entries should not appear as events
	if len(result.Events) != 0 {
		t.Errorf("expected 0 events for jobSummary-only file, got %d", len(result.Events))
	}
}
