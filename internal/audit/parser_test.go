package audit

import (
	"os"
	"testing"
)

func TestParseFile(t *testing.T) {
	result, err := ParseFile("../../sample_events.log", 0)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if len(result.Events) != 5 {
		t.Fatalf("expected 5 events, got %d", len(result.Events))
	}

	// Check first event
	e := result.Events[0]
	if e.Verb != "list" {
		t.Errorf("expected verb 'list', got %q", e.Verb)
	}
	if e.Resource != "csidrivers" {
		t.Errorf("expected resource 'csidrivers', got %q", e.Resource)
	}
	if e.Username != "system:apiserver" {
		t.Errorf("expected username 'system:apiserver', got %q", e.Username)
	}
	if e.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", e.StatusCode)
	}
	if e.SourceIP != "::1" {
		t.Errorf("expected sourceIP '::1', got %q", e.SourceIP)
	}
	if e.APIGroup != "storage.k8s.io" {
		t.Errorf("expected apiGroup 'storage.k8s.io', got %q", e.APIGroup)
	}

	// Check second event has status 503
	if result.Events[1].StatusCode != 503 {
		t.Errorf("expected event 2 status 503, got %d", result.Events[1].StatusCode)
	}

	// Check offset-based raw JSON reading works
	raw, err := ReadRawJSON(result.ReadPath, e.FileOffset, e.LineLength)
	if err != nil {
		t.Fatalf("ReadRawJSON failed: %v", err)
	}
	if len(raw) == 0 {
		t.Error("expected non-empty raw JSON")
	}
	// Should start with '{'
	if raw[0] != '{' {
		t.Errorf("expected raw JSON to start with '{', got %c", raw[0])
	}
}

func TestParseGzipFile(t *testing.T) {
	gzPath := "../../ip-10-0-31-19.us-west-2.compute.internal-audit-2023-09-01T16-27-38.073.log.gz"
	if _, err := os.Stat(gzPath); os.IsNotExist(err) {
		t.Skip("gzip test file not available")
	}

	result, err := ParseFile(gzPath, 0)
	if err != nil {
		t.Fatalf("ParseFile (gzip) failed: %v", err)
	}

	if len(result.Events) == 0 {
		t.Error("expected events from gzip file")
	}

	// Cleanup temp file
	if result.ReadPath != gzPath {
		os.Remove(result.ReadPath)
	}

	t.Logf("Parsed %d events from gzip file", len(result.Events))
}
