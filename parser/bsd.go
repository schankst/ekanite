package parser

import (
	"regexp"
	"strconv"
	"time"
)

// BSD parses RFC 3164 (BSD syslog) lines into Ekanite event fields.
// Example: "Oct 31 10:00:00 host1 app1[123]: message text"
// The optional leading PRI ("<34>Oct ...") is honored when present.
type BSD struct {
	matcher *regexp.Regexp
	stats   func(key string, delta int64)
}

// Stats sets the statistics callback for BSD parsing. Stored per instance so
// concurrent collectors do not race on a shared global (Issue #3).
func (p *BSD) Stats(callback func(key string, delta int64)) {
	p.stats = callback
}

// add bumps a statistic if a callback is wired.
func (p *BSD) add(key string, delta int64) {
	if p.stats != nil {
		p.stats(key, delta)
	}
}

// Init compiles the BSD matcher.
func (p *BSD) Init() {
	// Optional PRI, then "Mmm dd hh:mm:ss hostname tag[pid]: message".
	// Tag may be "app", "app[123]", or "app: message" without PID.
	p.matcher = regexp.MustCompile(`(?s)^(?:<(\d{1,3})>)?([A-Z][a-z]{2})\s+(\d{1,2})\s+(\d{1,2}):(\d{2}):(\d{2})\s+(\S+)\s+([^:\[\]]+?)(?:\[(\d+)\])?:\s?(.*)$`)
}

var bsdMonths = map[string]time.Month{
	"Jan": time.January, "Feb": time.February, "Mar": time.March,
	"Apr": time.April, "May": time.May, "Jun": time.June,
	"Jul": time.July, "Aug": time.August, "Sep": time.September,
	"Oct": time.October, "Nov": time.November, "Dec": time.December,
}

// Parse parses a raw RFC 3164 line into named fields. A non-matching line
// leaves the result untouched (Parser then counts/forwards it unparsed).
func (p *BSD) Parse(raw []byte, result *map[string]any) {
	m := p.matcher.FindStringSubmatch(string(raw))
	if m == nil {
		p.add("bsdUnparsed", 1)
		return
	}
	p.add("bsdParsed", 1)

	month, ok := bsdMonths[m[2]]
	if !ok {
		p.add("bsdUnparsed", 1)
		return
	}
	day, _ := strconv.Atoi(m[3])
	hour, _ := strconv.Atoi(m[4])
	minute, _ := strconv.Atoi(m[5])
	second, _ := strconv.Atoi(m[6])

	// RFC 3164 has no year: use the year of the reception time (per the
	// feature plan), and the local zone since BSD syslog is sender-local time.
	now := time.Now()
	t := time.Date(now.Year(), month, day, hour, minute, second, 0, time.Local).Format(time.RFC3339)

	var pri int
	if m[1] != "" {
		pri, _ = strconv.Atoi(m[1])
	}
	// Build the result with the same key conventions as the RFC5424 parser.
	out := map[string]any{
		"timestamp": t,
		"host":      m[7],
		"app":       m[8],
		"message":   m[10],
	}
	if pri > 0 {
		out["priority"] = pri
		out["facility"] = pri / 8
		out["severity"] = pri % 8
	}
	if m[9] != "" {
		if pid, err := strconv.Atoi(m[9]); err == nil {
			out["pid"] = pid
		}
	}
	*result = out
}
