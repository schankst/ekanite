package input

import (
	"io"
	"log"
	"strconv"
	"testing"
)

// Representative sample lines per supported input format, reused across the
// parser benchmarks so the measured cost is parsing, not fixture building.
const (
	benchRFC5424 = `<165>1 2024-01-01T00:00:00Z host1 app1 123 ID47 [exampleSDID@32473 iut="3" eventSource="Application"] an application event log entry`
	benchBSD     = `Oct 31 10:00:00 host1 app1[123]: hello world`
	benchFortiOS = `date=2023-01-01 time=12:00:01 devname="FGT80F" devid="FGT801234" logid="0100032001" type="traffic" subtype="forward" level="notice" vd="root" srcip=192.168.1.10 srcport=56789 srcintf="wan1" dstip=203.0.113.5 dstport=443 dstintf="port3" action="accept" policyid=17 sessionid=123456789 app="HTTPS"`
)

// benchFrames is the number of frames/messages fed per benchmark iteration so a
// single ns/op figure covers a realistic stream rather than a single byte.
const benchFrames = 100

// newBenchParser returns a Parser for format or fails the benchmark. NewParser
// logs the created format; that is silenced here so the benchmark output stays
// machine-readable (Issue #77).
func newBenchParser(b *testing.B, format string) *Parser {
	b.Helper()
	prev := log.Writer()
	log.SetOutput(io.Discard)
	p, err := NewParser(format)
	log.SetOutput(prev)
	if err != nil {
		b.Fatalf("NewParser(%q): %v", format, err)
	}
	return p
}

// octetStream builds n RFC 6587 octet-counted frames for msg.
func octetStream(n int, msg string) []byte {
	buf := make([]byte, 0, n*(len(msg)+4))
	for i := 0; i < n; i++ {
		buf = append(buf, strconv.Itoa(len(msg))...)
		buf = append(buf, ' ')
		buf = append(buf, msg...)
	}
	return buf
}

// lfStream builds n LF-delimited messages.
func lfStream(n int, msg string) []byte {
	buf := make([]byte, 0, n*(len(msg)+1))
	for i := 0; i < n; i++ {
		buf = append(buf, msg...)
		buf = append(buf, '\n')
	}
	return buf
}

// BenchmarkDelimiterOctet measures RFC 6587 octet-counting framing (byte at a
// time) over a stream of frame-per-message reads.
func BenchmarkDelimiterOctet(b *testing.B) {
	stream := octetStream(benchFrames, benchRFC5424)
	b.SetBytes(int64(len(stream)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d := NewOctetDelimiter(64 * 1024)
		frames := 0
		for _, c := range stream {
			if _, ok := d.Push(c); ok {
				frames++
			}
		}
		if frames != benchFrames {
			b.Fatalf("octet delimiter framed %d messages, want %d", frames, benchFrames)
		}
	}
}

// BenchmarkDelimiterSyslog measures PRI-anchored framing (the SyslogDelimiter
// used for "lf" and "auto"): a message is dispatched when the *next* message's
// `<PRI>` is seen, so the final message is only released by flushRemainder.
func BenchmarkDelimiterSyslog(b *testing.B) {
	stream := lfStream(benchFrames, benchRFC5424)
	b.SetBytes(int64(len(stream)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d := newFrameDelimiter("lf", 64*1024)
		frames := 0
		for _, c := range stream {
			if _, ok := d.Push(c); ok {
				frames++
			}
		}
		if _, ok := d.flushRemainder(); ok {
			frames++
		}
		if frames != benchFrames {
			b.Fatalf("syslog delimiter framed %d messages, want %d", frames, benchFrames)
		}
	}
}

// benchParser measures the steady-state cost of parsing one line with format.
func benchParser(b *testing.B, format, line string) {
	p := newBenchParser(b, format)
	raw := []byte(line)
	b.SetBytes(int64(len(raw)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !p.Parse(raw) {
			b.Fatalf("%s: parse failed on %q", format, line)
		}
	}
}

// BenchmarkParserRFC5424 measures RFC 5424 parsing (with structured data).
func BenchmarkParserRFC5424(b *testing.B) { benchParser(b, "syslog", benchRFC5424) }

// BenchmarkParserBSD measures RFC 3164 (BSD) parsing.
func BenchmarkParserBSD(b *testing.B) { benchParser(b, "bsd", benchBSD) }

// BenchmarkParserFortiOS measures FortiOS key=value parsing.
func BenchmarkParserFortiOS(b *testing.B) { benchParser(b, "fortios", benchFortiOS) }

// BenchmarkIngestPath measures the full per-line collector cost once framing is
// done: parse plus event construction (parsedEvent, Issue #5).
func BenchmarkIngestPath(b *testing.B) {
	p := newBenchParser(b, "syslog")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if e := parsedEvent(p, benchRFC5424, "10.0.0.1"); e == nil {
			b.Fatal("parsedEvent returned nil")
		}
	}
}
