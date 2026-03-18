package audit

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"time"
)

type rawEvent struct {
	Verb      string `json:"verb"`
	ObjectRef struct {
		Resource   string `json:"resource"`
		APIGroup   string `json:"apiGroup"`
		APIVersion string `json:"apiVersion"`
		Namespace  string `json:"namespace"`
	} `json:"objectRef"`
	User struct {
		Username string `json:"username"`
	} `json:"user"`
	SourceIPs      []string `json:"sourceIPs"`
	UserAgent      string   `json:"userAgent"`
	ResponseStatus struct {
		Code int `json:"code"`
	} `json:"responseStatus"`
	StageTimestamp string `json:"stageTimestamp"`
}

// ParseResult holds events from a single file and the path to read raw JSON from.
type ParseResult struct {
	Events   []AuditEvent
	ReadPath string // path for offset-based raw JSON reads (temp file for gzip)
}

// ParseFile parses a .log or .log.gz file into AuditEvents, tracking byte offsets.
// For .gz files, it decompresses to a temp file and returns that path in ReadPath.
func ParseFile(path string, fileIndex int) (*ParseResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var reader io.Reader
	readPath := path

	if isGzip(path) {
		tmpFile, err := decompressToTemp(f)
		if err != nil {
			return nil, err
		}
		readPath = tmpFile
		// Re-open the decompressed temp file for parsing
		tf, err := os.Open(tmpFile)
		if err != nil {
			return nil, err
		}
		defer tf.Close()
		reader = tf
	} else {
		reader = f
	}

	events, err := parseReader(reader, fileIndex)
	if err != nil {
		return nil, err
	}

	return &ParseResult{Events: events, ReadPath: readPath}, nil
}

func isGzip(path string) bool {
	n := len(path)
	return n > 3 && path[n-3:] == ".gz"
}

func decompressToTemp(r io.Reader) (string, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return "", err
	}
	defer gz.Close()

	tmp, err := os.CreateTemp("", "kube-audit-*.log")
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

func parseReader(r io.Reader, fileIndex int) ([]AuditEvent, error) {
	br := bufio.NewReaderSize(r, 1024*1024)

	// Peek at first non-whitespace byte to detect JSON array vs JSON-lines
	isArray := false
	for {
		b, err := br.Peek(1)
		if err != nil {
			return nil, nil
		}
		if b[0] == ' ' || b[0] == '\t' || b[0] == '\n' || b[0] == '\r' {
			br.ReadByte()
			continue
		}
		isArray = b[0] == '['
		break
	}

	if isArray {
		return parseJSONArray(br, fileIndex)
	}
	return parseJSONLines(br, fileIndex)
}

// parseJSONLines parses one JSON object per line, tracking byte offsets for raw re-reading.
func parseJSONLines(r io.Reader, fileIndex int) ([]AuditEvent, error) {
	var events []AuditEvent
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	var offset int64
	for scanner.Scan() {
		line := scanner.Bytes()
		lineLen := len(line)
		if lineLen == 0 {
			offset += int64(lineLen) + 1
			continue
		}

		if e, ok := parseRawEvent(line, fileIndex, offset, lineLen); ok {
			events = append(events, e)
		}

		offset += int64(lineLen) + 1 // +1 for newline
	}

	return events, scanner.Err()
}

// parseJSONArray parses a JSON array of audit event objects using json.Decoder.
func parseJSONArray(r io.Reader, fileIndex int) ([]AuditEvent, error) {
	var events []AuditEvent
	dec := json.NewDecoder(r)

	// Consume opening '['
	if _, err := dec.Token(); err != nil {
		return nil, err
	}

	for dec.More() {
		var obj json.RawMessage
		if err := dec.Decode(&obj); err != nil {
			continue
		}
		if e, ok := parseRawEvent(obj, fileIndex, 0, len(obj)); ok {
			events = append(events, e)
		}
	}

	return events, nil
}

func parseRawEvent(data []byte, fileIndex int, offset int64, lineLen int) (AuditEvent, bool) {
	var raw rawEvent
	if err := json.Unmarshal(data, &raw); err != nil {
		return AuditEvent{}, false
	}

	ts, _ := time.Parse(time.RFC3339Nano, raw.StageTimestamp)

	sourceIP := ""
	if len(raw.SourceIPs) > 0 {
		sourceIP = raw.SourceIPs[0]
	}

	return AuditEvent{
		Verb:       raw.Verb,
		Resource:   raw.ObjectRef.Resource,
		APIGroup:   raw.ObjectRef.APIGroup,
		APIVersion: raw.ObjectRef.APIVersion,
		Namespace:  raw.ObjectRef.Namespace,
		Username:   raw.User.Username,
		SourceIP:   sourceIP,
		UserAgent:  raw.UserAgent,
		StatusCode: raw.ResponseStatus.Code,
		Timestamp:  ts,
		FileIndex:  fileIndex,
		FileOffset: offset,
		LineLength: lineLen,
	}, true
}

// ReadRawJSON reads the raw JSON line for an event from the given file path.
func ReadRawJSON(path string, offset int64, length int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	buf := make([]byte, length)
	_, err = f.ReadAt(buf, offset)
	if err != nil {
		return nil, err
	}
	return buf, nil
}
