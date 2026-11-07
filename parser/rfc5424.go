package parser

import (
	"regexp"
	"strconv"
	"strings"
)

// NoValue is the placeholder used for RFC5424 NILVALUE fields ("-"). The
// parser stores nil in the result map instead of the literal "-" so that
// absent values are distinguishable from real ones (e.g. no msgid vs. an
// actual hyphen in the field) and do not pollute search/filter output.
const NoValue = ""

// BOM is the UTF-8 byte order mark RFC5424 §6.4 permits at the start of a
// message. It is stripped during parsing so it does not pollute search text.
const BOM = "\xef\xbb\xbf"

// RFC5424 parses RFC 5424 syslog lines into Ekanite event fields.
type RFC5424 struct {
	matcher *regexp.Regexp
	sdEle   *regexp.Regexp
	sdParam *regexp.Regexp
	stats   func(key string, delta int64)
}

// Stats sets the statistics callback for RFC5424 parsing. Stored per instance
// so concurrent collectors wiring their own parser do not race on a shared
// global counter target (Issue #3).
func (p *RFC5424) Stats(callback func(key string, delta int64)) {
	p.stats = callback
}

// add bumps a statistic if a callback is wired.
func (p *RFC5424) add(key string, delta int64) {
	if p.stats != nil {
		p.stats(key, delta)
	}
}

// Init compiles the RFC5424 matcher.
func (p *RFC5424) Init() {
	leading := `(?s)`
	// PRI: 1-3 numeric digits (validated against 0-191 in Parse).
	pri := `<([0-9]{1,3})>`
	// VERSION: RFC 5424 allows up to 3 digits (most commonly "1").
	ver := `([0-9]{1,3})`
	ts := `([^ ]+)`
	host := `([^ ]+)`
	app := `([^ ]+)`
	// PROCID: either NILVALUE "-" or printable ASCII (not just digits) — e.g.
	// "web-01", "systemd", per RFC 5424.
	pid := `(-|[\x21-\x7e]+)`
	// MSGID: NILVALUE "-" or printable ASCII including dots (PRINTUSASCII).
	id := `(-|[\x21-\x7e]+)`
	// MSG: everything after MSGID (and after optional Structured Data); the
	// structured-data separator is handled in Parse via the SD scanner.
	msg := `(.+$)`
	p.matcher = regexp.MustCompile(leading + pri + ver + `\s` + ts + `\s` + host + `\s` + app + `\s` + pid + `\s` + id + `\s` + msg)
	// Structured Data element: [NAME k1="v1" k2="v2"] ... [NAME2 ...].
	p.sdEle = regexp.MustCompile(`\[([^\s=\]]+)((?:\s+[^\s=\]]+="[^"]*")*)\]`)
	// A single NAME="value" parameter inside a structured-data element.
	p.sdParam = regexp.MustCompile(`\s+([^\s=\]]+)="([^"]*)"`)
}

// Parse parses a raw RFC5424 line into named fields.
// It never overwrites an empty result on failure so callers can detect a failed
// parse (len(*result) == 0). On success it always includes a "timestamp" key
// (or nil if the field was NILVALUE "-").
func (p *RFC5424) Parse(raw []byte, result *map[string]interface{}) {
	m := p.matcher.FindStringSubmatch(strings.TrimPrefix(string(raw), BOM))
	if m == nil || len(m) != 9 {
		p.add("rfc5424Unparsed", 1)
		return
	}
	pri, err := strconv.Atoi(m[1])
	// RFC 5424 §6.2.1: PRI is a single byte in the range 0-191.
	if err != nil || pri < 0 || pri > 191 {
		p.add("rfc5424Unparsed", 1)
		return
	}
	ver, _ := strconv.Atoi(m[2])

	var pid interface{}
	if m[6] != "-" {
		if n, err := strconv.Atoi(m[6]); err == nil {
			pid = n
		} else {
			// Non-numeric PROCID (e.g. "web-01"): keep the raw value so it is
			// neither dropped nor confused with a real numeric PID.
			pid = m[6]
		}
	}

	out := map[string]interface{}{
		"priority":   pri,
		"facility":   pri / 8,
		"severity":   pri % 8,
		"version":    ver,
		"timestamp":  nilIfDash(m[3]),
		"host":       nilIfDash(m[4]),
		"app":        nilIfDash(m[5]),
		"pid":        pid,
		"message_id": nilIfDash(m[7]),
	}

	// The MSG capture group holds either the structured data followed by the
	// message, or (for NILVALUE SD) just the message.
	sdText, message := p.splitSD(m[8])
	if sdText != nil {
		out["structured_data"] = sdText
	}
	out["message"] = message

	p.add("rfc5424Parsed", 1)
	*result = out
}

// nilIfDash returns nil for the RFC5424 NILVALUE "-", otherwise the value.
func nilIfDash(v string) interface{} {
	if v == "-" {
		return nil
	}
	return v
}

// splitSD scans a leading RFC5424 structured-data sequence from rest and
// returns the decoded structured data (a []map[string]interface{}, or nil if
// rest does not begin with an SD element) plus the remaining message.
func (p *RFC5424) splitSD(rest string) (sd []map[string]interface{}, msg string) {
	loc := p.sdEle.FindStringIndex(rest)
	if loc == nil || loc[0] != 0 {
		return nil, rest
	}
	pos := 0
	for pos < len(rest) {
		loc := p.sdEle.FindStringIndex(rest[pos:])
		if loc == nil || loc[0] != 0 {
			break
		}
		block := rest[pos+loc[0] : pos+loc[1]]
		ele := p.sdEle.FindStringSubmatch(block)
		if ele == nil {
			break
		}
		m := map[string]interface{}{}
		for _, prm := range p.sdParam.FindAllStringSubmatch(ele[2], -1) {
			m[prm[1]] = prm[2]
		}
		sd = append(sd, map[string]interface{}{ele[1]: m})
		pos += loc[1]
	}
	// After the last SD element an optional single space precedes the message.
	rest = rest[pos:]
	rest = strings.TrimPrefix(rest, " ")
	return sd, rest
}
