package parser

import (
	"fmt"
	"strings"
	"time"
)

// FortiOS parses FortiGate FortiOS key-value syslog lines into Ekanite event fields.
type FortiOS struct{}

var fortiosStats = func(key string, delta int64) {}

// Stats sets the statistics callback for FortiOS parsing.
func (p *FortiOS) Stats(callback func(key string, delta int64)) {
	fortiosStats = callback
}

// Init prepares the FortiOS parser for use.
func (p *FortiOS) Init() {}

// Parse parses a raw FortiOS line into named fields; returns nothing (result map).
func (p *FortiOS) Parse(raw []byte, result *map[string]interface{}) {
	fields := parseFortiOSKV(string(raw))
	if len(fields) == 0 {
		fortiosStats("fortiosUnparsed", 1)
		return
	}

	ts, ok := fortiOSTimestamp(fields["date"], fields["time"])
	if !ok {
		fortiosStats("fortiosUnparsed", 1)
		return
	}

	out := make(map[string]interface{}, len(fields)+4)
	for k, v := range fields {
		out[k] = v
	}

	out["timestamp"] = ts
	if v, ok := fields["devname"]; ok {
		out["host"] = v
	}
	if v, ok := fields["app"]; ok {
		out["app"] = v
	}
	if v, ok := fields["srcip"]; ok {
		out["source_ip"] = v
	}
	if msg, ok := fields["msg"]; ok && msg != "" {
		out["message"] = msg
	} else {
		out["message"] = string(raw)
	}

	fortiosStats("fortiosParsed", 1)
	*result = out
}

func fortiOSTimestamp(date, t string) (string, bool) {
	if date == "" || t == "" {
		return "", false
	}
	// If time already carries a timezone indicator, use it as-is; otherwise assume UTC.
	if strings.HasSuffix(t, "Z") || strings.ContainsAny(t, "+") || hasTimeZoneOffset(t) {
		candidate := fmt.Sprintf("%sT%s", date, t)
		if parsed, err := time.Parse(time.RFC3339Nano, candidate); err == nil {
			return parsed.Format(time.RFC3339Nano), true
		}
		if parsed, err := time.Parse("2006-01-02T15:04:05Z07:00", candidate); err == nil {
			return parsed.Format(time.RFC3339), true
		}
		// Try without colon in offset (e.g. +0200)
		if parsed, err := time.Parse("2006-01-02T15:04:05Z0700", candidate); err == nil {
			return parsed.Format(time.RFC3339), true
		}
		return "", false
	}
	candidate := fmt.Sprintf("%sT%sZ", date, t)
	if parsed, err := time.Parse(time.RFC3339Nano, candidate); err == nil {
		return parsed.Format(time.RFC3339Nano), true
	}
	if parsed, err := time.Parse(time.RFC3339, candidate); err == nil {
		return parsed.Format(time.RFC3339), true
	}
	return "", false
}

func hasTimeZoneOffset(t string) bool {
	// Detect a trailing -HH:MM or -HHMM offset (time contains '-' after the time portion HH:MM:SS).
	if len(t) < 6 {
		return false
	}
	// Look for '-' that is not at position 0 and appears after a colon (time separator)
	for i := len(t) - 1; i >= 3; i-- {
		if t[i] == '-' {
			return true
		}
	}
	return false
}

func parseFortiOSKV(s string) map[string]string {
	out := make(map[string]string)
	s = strings.TrimRight(s, "\r\n")
	i := 0
	n := len(s)
	for i < n {
		// skip whitespace
		for i < n && (s[i] == ' ' || s[i] == '\t') {
			i++
		}
		if i >= n {
			break
		}
		// read key up to first '='
		keyStart := i
		for i < n && s[i] != '=' && s[i] != ' ' && s[i] != '\t' {
			i++
		}
		if i >= n || s[i] != '=' {
			// malformed token, skip to next whitespace
			for i < n && s[i] != ' ' && s[i] != '\t' {
				i++
			}
			continue
		}
		key := s[keyStart:i]
		i++ // skip '='
		if i >= n {
			out[key] = ""
			break
		}
		var val string
		if s[i] == '"' {
			i++ // skip opening quote
			valStart := i
			for i < n && s[i] != '"' {
				i++
			}
			val = s[valStart:i]
			if i < n {
				i++ // skip closing quote
			}
		} else {
			valStart := i
			for i < n && s[i] != ' ' && s[i] != '\t' {
				i++
			}
			val = s[valStart:i]
		}
		if key != "" {
			out[key] = val
		}
	}
	return out
}
