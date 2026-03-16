package store

import (
	"time"

	"github.com/afcollins/kube-audit-log-tool/internal/audit"
)

type TimelineBucket struct {
	Start time.Time
	End   time.Time
	Count int
}

// BuildTimeline creates time-bucketed histogram data from audit events.
func BuildTimeline(events []audit.AuditEvent, indices []int, targetBuckets int) []TimelineBucket {
	return BuildTimelineFunc(func(i int) time.Time { return events[i].Timestamp }, indices, targetBuckets)
}

// BuildTimelineFunc creates time-bucketed histogram data using a timestamp accessor.
// This allows reuse across different event types (audit events, metrics, etc.).
func BuildTimelineFunc(timestamp func(i int) time.Time, indices []int, targetBuckets int) []TimelineBucket {
	if len(indices) == 0 {
		return nil
	}

	minT := timestamp(indices[0])
	maxT := timestamp(indices[0])
	for _, i := range indices {
		t := timestamp(i)
		if t.Before(minT) {
			minT = t
		}
		if t.After(maxT) {
			maxT = t
		}
	}

	span := maxT.Sub(minT)
	if span == 0 {
		return []TimelineBucket{{Start: minT, End: maxT, Count: len(indices)}}
	}

	if targetBuckets < 1 {
		targetBuckets = 30
	}

	bucketSize := span / time.Duration(targetBuckets)
	if bucketSize < time.Second {
		bucketSize = time.Second
	}

	numBuckets := int(span/bucketSize) + 1
	buckets := make([]TimelineBucket, numBuckets)
	for i := range buckets {
		buckets[i].Start = minT.Add(time.Duration(i) * bucketSize)
		buckets[i].End = minT.Add(time.Duration(i+1) * bucketSize)
	}

	for _, idx := range indices {
		t := timestamp(idx)
		bi := int(t.Sub(minT) / bucketSize)
		if bi >= numBuckets {
			bi = numBuckets - 1
		}
		buckets[bi].Count++
	}

	return buckets
}
