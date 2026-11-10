package ekanite

import (
	"strconv"
	"testing"
	"time"

	"github.com/schankst/ekanite/input"
)

// benchLine is a representative RFC 5424 line for the indexing benchmarks.
const benchLine = `<165>1 2024-01-01T00:00:00Z host1 app1 123 ID47 - an application event log entry`

const (
	// benchEvents is the number of events indexed per operation; it matches the
	// 100k-event throughput question (Issue #77).
	benchEvents = 100000
	// benchBatchSize is the production indexing batch size (DefaultBatchSize).
	benchBatchSize = 300
	// benchShards is the production shard count (DefaultNumShards).
	benchShards = 4
)

// makeBenchEvents builds n distinct indexable events sharing one reference time
// so they land in a single index (the realistic steady-state shape). Distinct
// Sequence values keep the derived DocIDs unique.
func makeBenchEvents(n int) []*Event {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	evs := make([]*Event, n)
	for i := range evs {
		evs[i] = &Event{Event: &input.Event{
			Text:          benchLine + " #" + strconv.Itoa(i),
			ReceptionTime: base,
			Sequence:      int64(i),
		}}
	}
	return evs
}

// BenchmarkBatchIndexing measures end-to-end batch-indexing throughput over a
// 100k-event workload fed in production-sized batches (Issue #77). events/s =
// benchEvents * 1e9 / ns-per-op.
func BenchmarkBatchIndexing(b *testing.B) {
	eng := NewEngine(b.TempDir())
	eng.NumShards = benchShards
	if err := eng.Open(); err != nil {
		b.Fatalf("open engine: %v", err)
	}
	defer eng.Close()

	events := makeBenchEvents(benchEvents)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for off := 0; off < len(events); off += benchBatchSize {
			end := min(off+benchBatchSize, len(events))
			if err := eng.Index(events[off:end]); err != nil {
				b.Fatalf("index batch: %v", err)
			}
		}
	}
}
