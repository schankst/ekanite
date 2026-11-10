package ekanite

import (
	"io"
	"log"
	"strconv"
	"testing"
	"time"

	"github.com/schankst/ekanite/input"
)

// benchLine is a representative RFC 5424 line (with structured data) for the
// indexing benchmarks.
const benchLine = `<165>1 2024-01-01T00:00:00Z host1 app1 123 ID47 [exampleSDID@32473 iut="3" eventSource="Application"] an application event log entry`

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
// so they land in a single index. The parsed fields come from the real parser,
// so the document shape — and therefore the index size — matches production
// instead of benchmarking a Message-only event (see docs/CAPACITY.md).
func makeBenchEvents(b testing.TB, n int) []*Event {
	b.Helper()
	prev := log.Writer()
	log.SetOutput(io.Discard)
	p, err := input.NewParser("syslog")
	log.SetOutput(prev)
	if err != nil {
		b.Fatalf("NewParser: %v", err)
	}

	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	evs := make([]*Event, n)
	for i := range evs {
		line := benchLine + " #" + strconv.Itoa(i)
		var parsed map[string]any
		if p.Parse([]byte(line)) {
			parsed = make(map[string]any, len(p.Result))
			for k, v := range p.Result {
				parsed[k] = v
			}
		}
		evs[i] = &Event{Event: &input.Event{
			Text:          line,
			Parsed:        parsed,
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

	events := makeBenchEvents(b, benchEvents)
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
