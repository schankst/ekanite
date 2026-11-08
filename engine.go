package ekanite

import (
	"expvar"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ekanite/ekanite/input"
)

// Engine defaults
const (
	DefaultNumShards          = 16
	DefaultIndexDuration      = 24 * time.Hour
	DefaultRetentionPeriod    = 24 * time.Hour
	DefaultRetentionCheckIntv = time.Hour
)

// Engine stats
var (
	stats = expvar.NewMap("engine")
)

// EventIndexer is the interface a system than can index events must implement.
type EventIndexer interface {
	Index(events []*Event) error
}

// Batcher accepts "input events", and once it has a certain number, or a certain amount
// of time has passed, sends those as indexable Events to an Indexer. It also supports a
// maximum number of unprocessed Events it will keep pending. Once this limit is reached,
// it will not accept anymore until outstanding Events are processed.
type Batcher struct {
	indexer  EventIndexer
	size     int
	duration time.Duration

	c    chan *input.Event
	done chan struct{}
}

// NewBatcher returns a Batcher for EventIndexer e, a batching size of sz, a maximum duration
// of dur, and a maximum outstanding count of max.
func NewBatcher(e EventIndexer, sz int, dur time.Duration, max int) *Batcher {
	return &Batcher{
		indexer:  e,
		size:     sz,
		duration: dur,
		c:        make(chan *input.Event, max),
		done:     make(chan struct{}),
	}
}

// Stop signals the batching loop to stop. The current partial batch is
// flushed before the loop exits, so no buffered events are lost (Issue #49).
func (b *Batcher) Stop() {
	close(b.done)
}

// Start starts the batching process.
func (b *Batcher) Start(errChan chan<- error) error {
	go func() {
		// Last-resort guard: a panic in the batching loop must not crash the
		// process (Issue #51). Recovery keeps ingestion alive; the failed
		// batch is dropped and the next one continues.
		defer func() {
			if r := recover(); r != nil {
				stats.Add("batcherPanicsRecovered", 1)
				log.Printf("panic in batcher recovered: %v", r)
			}
		}()
		batch := make([]*Event, 0, b.size)
		timer := time.NewTimer(b.duration)
		timer.Stop() // Stop any first firing.

		send := func() {
			err := b.indexer.Index(batch)
			if err != nil {
				// Reset the batch even on failure so a persistent index error
				// (e.g. disk full) cannot grow this slice without bound
				// (Issue #46). The failed events are dropped; the error is
				// surfaced on errChan so it is not silently swallowed (#7).
				stats.Add("batchIndexedError", 1)
				batch = make([]*Event, 0, b.size)
				if errChan != nil {
					errChan <- err
				}
				return
			}
			stats.Add("batchIndexed", 1)
			stats.Add("eventsIndexed", int64(len(batch)))
			if errChan != nil {
				errChan <- nil
			}
			batch = make([]*Event, 0, b.size)
		}

		for {
			select {
			case event := <-b.c:
				idxEvent := &Event{
					event,
				}
				batch = append(batch, idxEvent)
				if len(batch) == 1 {
					timer.Reset(b.duration)
				}
				if len(batch) == b.size {
					timer.Stop()
					send()
				}
			case <-timer.C:
				stats.Add("batchTimeout", 1)
				send()
			case <-b.done:
				// Ordered shutdown (Issue #49): flush whatever is pending,
				// then exit so the engine can be closed cleanly.
				if len(batch) > 0 {
					send()
				}
				return
			}
		}
	}()

	return nil
}

// C returns the channel on the batcher to which events should be sent.
func (b *Batcher) C() chan<- *input.Event {
	return b.c
}

// Engine is the component that performs all indexing.
type Engine struct {
	path            string        // Path to all indexed data
	NumShards       int           // Number of shards to use when creating an index.
	IndexDuration   time.Duration // Duration of created indexes.
	RetentionPeriod time.Duration // How long after Index end-time to hang onto data.
	// RetentionCheckInterval controls how often expired indexes are swept.
	// Configurable via -retentioncheck (Issue #64).
	RetentionCheckInterval time.Duration
	// WipeEnabled, when set, overwrites index files with zeroes before removal
	// during retention so deleted data is not trivially recoverable from disk
	// (Issue #92). Default off; enabling it has a disk-write cost.
	WipeEnabled bool
	// AuditLogger, when non-nil, receives a JSONL line per retention deletion
	// (timestamp, index path, wiped flag) as an immutable deletion record
	// (Issue #92).
	AuditLogger *log.Logger

	mu      sync.RWMutex
	indexes Indexes

	open     bool
	done     chan struct{}
	wg       sync.WaitGroup
	searchWG sync.WaitGroup

	Logger *log.Logger
}

// NewEngine returns a new indexing engine, which will use any data located at path.
func NewEngine(path string) *Engine {
	return &Engine{
		path:                   path,
		NumShards:              DefaultNumShards,
		IndexDuration:          DefaultIndexDuration,
		RetentionPeriod:        DefaultRetentionPeriod,
		RetentionCheckInterval: DefaultRetentionCheckIntv,
		done:                   make(chan struct{}),
		Logger:                 log.New(os.Stderr, "[engine] ", log.LstdFlags),
	}
}

// Open opens the engine.
func (e *Engine) Open() error {
	if err := os.MkdirAll(e.path, 0700); err != nil {
		return err
	}
	d, err := os.Open(e.path)
	if err != nil {
		return fmt.Errorf("failed to open engine: %w", err)
	}

	fis, err := d.Readdir(0)
	if err != nil {
		return err
	}

	// Open all indexes.
	for _, fi := range fis {
		if !fi.IsDir() || strings.HasPrefix(fi.Name(), ".") {
			continue
		}
		indexPath := filepath.Join(e.path, fi.Name())
		i, err := OpenIndex(indexPath)
		if err != nil {
			return fmt.Errorf("engine failed to open at index %s: %w", indexPath, err)
		}
		log.Printf("engine opened index with %d shard(s) at %s", len(i.Shards), indexPath)
		e.indexes = append(e.indexes, i)
		slices.SortFunc(e.indexes, func(a, b *Index) int {
			if sortIndex(a, b) {
				return -1
			}
			if sortIndex(b, a) {
				return 1
			}
			return 0
		})
	}

	e.wg.Add(1)
	go e.runRetentionEnforcement()

	e.open = true
	return nil
}

// Close closes the engine in an ordered manner (Issue #49): first the done
// channel signals in-flight searches and the retention loop to stop, then it
// waits for both before closing the underlying indexes — so no goroutine
// touches a closed index.
func (e *Engine) Close() error {
	if !e.open {
		return nil
	}

	close(e.done)
	e.wg.Wait()
	e.searchWG.Wait()

	for _, i := range e.indexes {
		if err := i.Close(); err != nil {
			return err
		}
	}

	e.open = false
	return nil
}

// Total returns the total number of documents indexed.
func (e *Engine) Total() (uint64, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var total uint64
	for _, i := range e.indexes {
		t, err := i.Total()
		if err != nil {
			return 0, err
		}
		total += t
	}
	return total, nil
}

// runRetentionEnforcement periodically runs retention enforcement.
func (e *Engine) runRetentionEnforcement() {
	defer e.wg.Done()
	for {
		select {
		case <-e.done:
			return

		case <-time.After(e.RetentionCheckInterval):
			stats.Add("retentionEnforcementRun", 1)
			e.enforceRetention()
		}
	}
}

// enforceRetention removes indexes which have aged out.
func (e *Engine) enforceRetention() {
	e.mu.Lock()
	defer e.mu.Unlock()

	filtered := e.indexes[:0]
	for _, i := range e.indexes {
		if i.Expired(time.Now().UTC(), e.RetentionPeriod) {
			wiped := false
			if e.WipeEnabled {
				if err := WipeIndex(i); err != nil {
					e.Logger.Printf("retention wipe failed for index %s: %s", i.path, err.Error())
				} else {
					wiped = true
				}
			}
			if err := DeleteIndex(i); err != nil {
				e.Logger.Printf("retention enforcement failed to delete index %s: %s", i.path, err.Error())
			} else {
				e.Logger.Printf("retention enforcement deleted index %s", i.path)
				stats.Add("retentionEnforcementDeletions", 1)
				if e.AuditLogger != nil {
					// Immutable deletion record (JSONL): timestamp, path, wiped flag.
					e.AuditLogger.Printf(`{"ts":%q,"index":%q,"wiped":%t}`,
						time.Now().UTC().Format(time.RFC3339), i.path, wiped)
				}
			}
		} else {
			filtered = append(filtered, i)
		}
	}
	e.indexes = filtered
}

// indexForReferenceTime returns an index suitable for indexing an event
// for the given reference time. Must be called under RLock.
func (e *Engine) indexForReferenceTime(t time.Time) *Index {
	for _, i := range e.indexes {
		if i.Contains(t) {
			return i
		}
	}
	return nil
}

// createIndex creates an index with a given start and end time and adds the
// created index to the Engine's store. It must be called under lock.
func (e *Engine) createIndex(startTime, endTime time.Time) (*Index, error) {
	// There cannot be two indexes with the same start time, since this would mean
	// two indexes with the same path. So if an index already exists with the requested
	// start time, use that index's end time as the start time.
	var idx *Index
	for _, i := range e.indexes {
		if i.startTime == startTime {
			idx = i
			break
		}
	}
	if idx != nil {
		startTime = idx.endTime // XXX This could still align with another start time! Needs some sort of loop.
		if startTime.After(endTime) {
			return nil, fmt.Errorf("index start time %s after requested end time %s",
				startTime.UTC().Format(indexNameLayout), endTime.UTC().Format(indexNameLayout))
		}
	}

	i, err := NewIndex(e.path, startTime, endTime, e.NumShards)
	if err != nil {
		return nil, err
	}
	e.indexes = append(e.indexes, i)
	slices.SortFunc(e.indexes, func(a, b *Index) int {
		if sortIndex(a, b) {
			return -1
		}
		if sortIndex(b, a) {
			return 1
		}
		return 0
	})

	e.Logger.Printf("index %s created with %d shards, start time: %s, end time: %s",
		i.Path(), e.NumShards, i.StartTime(), i.EndTime())
	return i, nil
}

// createIndexForReferenceTime creates an index suitable for indexing an event at the given
// reference time.
func (e *Engine) createIndexForReferenceTime(rt time.Time) (*Index, error) {
	start := rt.Truncate(e.IndexDuration).UTC()
	end := start.Add(e.IndexDuration).UTC()
	return e.createIndex(start, end)
}

// Index indexes a batch of Events. It blocks until all processing has completed.
func (e *Engine) Index(events []*Event) error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var wg sync.WaitGroup

	// De-multiplex the batch into sub-batches, one sub-batch for each Index.
	subBatches := make(map[*Index][]Document, 0)

	for _, ev := range events {
		index := e.indexForReferenceTime(ev.ReferenceTime())
		if index == nil {
			// Take a RWLock, check again, and create a new index if necessary.
			// Doing this in a function makes lock management foolproof. An index
			// creation failure is returned as a batch error instead of crashing
			// the process (Issue #51).
			var err error
			func() {
				e.mu.RUnlock()
				defer e.mu.RLock()
				e.mu.Lock()
				defer e.mu.Unlock()

				index = e.indexForReferenceTime(ev.ReferenceTime())
				if index == nil {
					index, err = e.createIndexForReferenceTime(ev.ReferenceTime())
				}
			}()
			if err != nil {
				return fmt.Errorf("failed to create index for %s: %w", ev.ReferenceTime(), err)
			}
		}

		if _, ok := subBatches[index]; !ok {
			subBatches[index] = make([]Document, 0)
		}
		subBatches[index] = append(subBatches[index], ev)
	}

	// Index each batch in parallel.
	errCh := make(chan error, len(subBatches))
	for index, subBatch := range subBatches {
		wg.Add(1)
		go func(i *Index, b []Document) {
			defer wg.Done()
			errCh <- i.Index(b)
		}(index, subBatch)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}

// Search performs a search.
func (e *Engine) Search(query string) (<-chan string, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	stats.Add("queriesRx", 1)

	// Buffer channel to control how many docs are sent back.
	c := make(chan string, 1)

	e.searchWG.Add(1)
	go func() {
		defer e.searchWG.Done()
		// Sequentially search each index, starting with the earliest in time.
		// This could be done in parallel but more sorting would be required.
		for i := len(e.indexes) - 1; i >= 0; i-- {
			// Abort on shutdown so a search cannot keep the process alive
			// or race with index close (Issue #49/#13).
			select {
			case <-e.done:
				close(c)
				return
			default:
			}
			e.Logger.Printf("searching index %s", e.indexes[i].Path())
			ids, err := e.indexes[i].Search(query)
			if err != nil {
				e.Logger.Println("error performing search:", err.Error())
				break
			}
			for _, id := range ids {
				b, err := e.indexes[i].Document(id)
				if err != nil {
					e.Logger.Println("error getting document:", err.Error())
					break
				}
				stats.Add("docsIDsRetrieved", 1)
				select {
				case c <- string(b): // There is excessive byte-slice-to-strings here.
				case <-e.done:
					close(c)
					return
				}
			}
		}
		close(c)
	}()

	return c, nil
}

// Path returns the path to the directory of indexed data.
func (e *Engine) Path() string {
	return e.path
}
