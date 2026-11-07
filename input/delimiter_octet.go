package input

import (
	"strconv"
	"strings"
)

// frameDelimiter splits a byte stream into complete messages. Implementations
// are stateful and single-goroutine: Push feeds one byte at a time and returns
// a complete message once a full frame has been consumed. flushRemainder
// returns any buffered message when the stream closes (or is interrupted).
type frameDelimiter interface {
	Push(b byte) (string, bool)
	flushRemainder() (string, bool)
}

// RFC 6587 §3.4.1 octet-counting framing: "MSG-LEN SP SYSLOG-MSG", where
// MSG-LEN is the length in octets of the SYSLOG-MSG (which itself begins with
// the PRI). No delimiter terminates the message; exactly MSG-LEN octets follow
// the length prefix.
type octetDelimiter struct {
	buf     []byte
	msgLen  int
	haveLen bool
	maxSize int
	broken  bool
}

// NewOctetDelimiter returns an initialized RFC 6587 octet-counting delimiter.
// maxSize caps the accepted message length; longer frames are counted as
// truncation errors instead of being buffered unboundedly (Issue #33/#47).
func NewOctetDelimiter(maxSize int) *octetDelimiter {
	return &octetDelimiter{maxSize: maxSize}
}

// brokenLenErr is counted via the input stats map in Push; returning the flag
// keeps the delimiter itself alloc-free on error paths.
var octetStats = func(key string, delta int64) {}

// SetStats wires the octet delimiter's error counter into the input stats map.
func (o *octetDelimiter) SetStats(callback func(key string, delta int64)) {
	octetStats = callback
}

// Push consumes one byte. It returns a message exactly when a full octet frame
// has been read. Over-length or malformed length prefixes set the broken state
// (the frame is abandoned) and are counted via "octetFramingError".
func (o *octetDelimiter) Push(b byte) (string, bool) {
	if o.broken {
		return "", false
	}
	if !o.haveLen {
		// Length prefix: one or more digits terminated by a single space.
		if b == ' ' {
			// Empty length is invalid; resync by scanning for the next
			// digits+space token rather than abandoning the stream.
			if len(o.buf) == 0 {
				octetStats("octetFramingError", 1)
				return "", false
			}
			n, err := strconv.Atoi(string(o.buf))
			if err != nil || n <= 0 {
				// Malformed length: discard the token and resync (the next
				// digits+space becomes the length). Counts the error (DoD #33).
				o.buf = o.buf[:0]
				octetStats("octetFramingError", 1)
				return "", false
			}
			if n > o.maxSize {
				o.broken = true
				octetStats("octetTruncated", 1)
				return "", false
			}
			o.msgLen = n
			o.haveLen = true
			o.buf = o.buf[:0]
			return "", false
		}
		if b < '0' || b > '9' {
			// Non-digit while in length mode: this frame is malformed. Count
			// it and resync by dropping the byte (recovery, DoD #33).
			o.buf = o.buf[:0]
			octetStats("octetFramingError", 1)
			return "", false
		}
		o.buf = append(o.buf, b)
		return "", false
	}
	// Message body: consume exactly msgLen octets.
	o.buf = append(o.buf, b)
	if len(o.buf) == o.msgLen {
		msg := string(o.buf)
		o.buf = o.buf[:0]
		o.haveLen = false
		o.msgLen = 0
		return msg, true
	}
	return "", false
}

// flushRemainder drops any incomplete frame: an unterminated octet message
// cannot be validated, and silently emitting it would fabricate data.
func (o *octetDelimiter) flushRemainder() (string, bool) {
	o.buf = nil
	o.haveLen = false
	o.msgLen = 0
	return "", false
}

// autoDelimiter decides the framing mode on the first byte: a digit starts an
// octet-counted frame, anything else is treated as LF-delimited syslog. The
// decision is sticky per stream (RFC 6587 clients use one framing).
type autoDelimiter struct {
	octet *octetDelimiter
	lf    *SyslogDelimiter
	mode  int // 0 = undecided, 1 = octet, 2 = lf
}

func (a *autoDelimiter) Push(b byte) (string, bool) {
	if a.mode == 0 {
		if b >= '0' && b <= '9' {
			a.mode = 1
			return a.octet.Push(b)
		}
		a.mode = 2
	}
	if a.mode == 1 {
		return a.octet.Push(b)
	}
	return a.lf.Push(b)
}

func (a *autoDelimiter) flushRemainder() (string, bool) {
	if a.mode == 1 {
		return a.octet.flushRemainder()
	}
	if a.mode == 2 {
		return a.lf.flushRemainder()
	}
	return "", false
}

// newFrameDelimiter returns the delimiter for the requested framing mode.
// framing is one of "auto", "lf", or "octet" (case-insensitive); unknown
// values fall back to "auto" and are counted as a config error.
func newFrameDelimiter(framing string, maxSize int) frameDelimiter {
	switch strings.ToLower(framing) {
	case "lf":
		return NewSyslogDelimiter(maxSize)
	case "octet":
		d := NewOctetDelimiter(maxSize)
		d.SetStats(stats.Add)
		return d
	default:
		return &autoDelimiter{octet: &octetDelimiter{maxSize: maxSize}, lf: NewSyslogDelimiter(maxSize)}
	}
}
