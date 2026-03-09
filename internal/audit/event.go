package audit

import "time"

type AuditEvent struct {
	Verb       string
	Resource   string
	APIGroup   string
	APIVersion string
	Namespace  string
	Username   string
	SourceIP   string
	UserAgent  string
	StatusCode int
	Timestamp  time.Time
	FileIndex  int
	FileOffset int64
	LineLength int
}
