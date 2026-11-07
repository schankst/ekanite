package input

import (
	"fmt"
	"github.com/ekanite/ekanite/parser"
	"log"
	"sync/atomic"
	"time"
)

const (
	RFC5424Standard = "RFC5424"
	RFC5424Name     = "syslog"
	FortiOSStandard = "FortiOS"
	FortiOSName     = "fortios"
	BSDStandard     = "BSD"
	BSDName         = "bsd"
)

// StatsCollector records parser/input statistics as key/delta increments.
type StatsCollector func(key string, delta int64)

// LogParser parses a raw log line into result fields.
type LogParser interface {
	Parse(raw []byte, result *map[string]interface{})
	Init()
	// Stats wires the parser's internal counters (e.g. rfc5424Unparsed) into
	// the input statistics map (Issue #35).
	Stats(callback func(key string, delta int64))
}

// A Parser parses the raw input as a map with a timestamp field.
type LogHandler struct {
	Fmt    string
	Raw    []byte
	Result map[string]interface{}
	Parser LogParser
	Stats  func(key string, delta int64)
}

func supportedFormats() [][]string {
	return [][]string{
		{RFC5424Name, RFC5424Standard},
		{FortiOSName, FortiOSStandard},
		{BSDName, BSDStandard},
	}
}

// ValidFormat returns if the given format matches one of the possible formats.
func ValidFormat(f string) bool {
	for _, v := range supportedFormats() {
		if v[0] == f || v[1] == f {
			return true
		}
	}

	return false
}

// NewParser returns a new Parser instance.
func NewParser(f string) (*LogHandler, error) {
	if !ValidFormat(f) {
		return nil, fmt.Errorf("%s is not a valid parser format", f)
	}

	var p = &LogHandler{}

	if f == RFC5424Name || f == RFC5424Standard {
		p.Parser = &parser.RFC5424{}
		p.Fmt = RFC5424Standard
	}
	if f == FortiOSName || f == FortiOSStandard {
		p.Parser = &parser.FortiOS{}
		p.Fmt = FortiOSStandard
	}
	if f == BSDName || f == BSDStandard {
		p.Parser = &parser.BSD{}
		p.Fmt = BSDStandard
	}

	log.Printf("input format parser created for %s", f)
	p.Stats = stats.Add
	p.Parser.Stats(p.Stats) // wire parser-level counters (e.g. rfc5424Unparsed) into expvar "input"
	p.Parser.Init()
	return p, nil
}

// Warn window for aggregated unparsed-line logging (Issue #35): unparsed lines
// are logged once per minute, so a noisy sender cannot flood the log.
const unparsedWarnInterval = time.Minute

var (
	unparsedCount     int64
	lastUnparsedWarn  int64 // unix nanos of the last warning
	unparsedWarnTotal int64
)

// warnUnparsed records an unparsed line and, once per minute, logs an
// aggregated warning with the running totals so silent input loss is visible
// (Issue #35). The per-minute window keeps high-volume non-syslog traffic from
// flooding the log while still surfacing the problem promptly.
func warnUnparsed() {
	n := atomic.AddInt64(&unparsedCount, 1)
	total := atomic.AddInt64(&unparsedWarnTotal, 1)
	last := atomic.LoadInt64(&lastUnparsedWarn)
	now := time.Now().UnixNano()
	if now-last < int64(unparsedWarnInterval) {
		return
	}
	if atomic.CompareAndSwapInt64(&lastUnparsedWarn, last, now) {
		log.Printf("WARNING: %d unparsed line(s) in the last minute (%d total) — check sender format (syslog/fortios) or use raw-line fallback (Issue #16/#35)", n, total)
		atomic.StoreInt64(&unparsedCount, 0)
	}
}
func (p *LogHandler) Parse(b []byte) bool {
	p.Result = map[string]interface{}{}
	p.Raw = b
	p.Parser.Parse(p.Raw, &p.Result)
	if len(p.Result) == 0 {
		// The event is not dropped: collectors forward it as an unparsed
		// raw-line fallback (Issue #16). Count it here so the loss is
		// observable even for parser formats without their own counter, and
		// surface a throttled warning (Issue #35).
		if p.Stats != nil {
			p.Stats("unparsed", 1)
		}
		warnUnparsed()
		return false
	}
	return true
}
