package input

import (
	"strconv"
	"testing"
)

// feed pushes every byte and collects complete messages.
func feed(d frameDelimiter, s string) []string {
	var out []string
	for i := 0; i < len(s); i++ {
		if msg, ok := d.Push(s[i]); ok {
			out = append(out, msg)
		}
	}
	return out
}

// #33: RFC 6587 octet-counting framing consumes exactly MSG-LEN octets.
func TestOctetDelimiter_Basic(t *testing.T) {
	msg1 := `<134>1 2003-08-24T05:14:15.000003-07:00 ubuntu sshd 1999 - octet frame`
	msg2 := `<22>1 short`
	d := NewOctetDelimiter(4096)
	got := feed(d, strconv.Itoa(len(msg1))+" "+msg1+strconv.Itoa(len(msg2))+" "+msg2)
	if len(got) != 2 || got[0] != msg1 || got[1] != msg2 {
		t.Fatalf("octet framing wrong: %#v", got)
	}
}

// #33: Messages may contain embedded LF (length-precise, not newline-split).
func TestOctetDelimiter_EmbeddedLF(t *testing.T) {
	msg := "<134>1 2024-01-01T00:00:00Z h a 1 - multi\nline body"
	d := NewOctetDelimiter(4096)
	got := feed(d, strconv.Itoa(len(msg))+" "+msg)
	if len(got) != 1 || got[0] != msg {
		t.Fatalf("embedded LF not preserved: %#v", got)
	}
}

// #33: Malformed length prefixes are rejected and counted, not silently
// dropped: the stream resyncs and subsequent valid frames still arrive.
func TestOctetDelimiter_BadPrefix(t *testing.T) {
	good := `<134>1 2024-01-01T00:00:00Z h a 1 - ok`
	d := NewOctetDelimiter(4096)
	// "0 " is an invalid (zero) length; a stray '<' inside length mode is
	// malformed. Both must resync: the "1 x" frame is a genuine resync find,
	// and the final valid frame must arrive after the errors.
	got := feed(d, "0 <134>1 x"+strconv.Itoa(len(good))+" "+good)
	if len(got) != 2 || got[0] != "x" || got[1] != good {
		t.Fatalf("recovery after bad prefix failed: %#v", got)
	}
}

// #33: Over-length frames are counted as truncated instead of unbounded buffer.
func TestOctetDelimiter_Overlong(t *testing.T) {
	d := NewOctetDelimiter(8) // tiny cap
	got := feed(d, "100 <134>1 x")
	if len(got) != 0 {
		t.Fatalf("overlong frame should not emit: %#v", got)
	}
}

// #33: auto mode detects octet and LF framing per stream on first byte.
func TestAutoDelimiter(t *testing.T) {
	// Octet stream (starts with a digit).
	msg := "<134>1 2003-08-24T05:14:15.000003-07:00 ubuntu sshd 1999 - frame one"
	a1 := newFrameDelimiter("auto", 4096).(*autoDelimiter)
	got := feed(a1, strconv.Itoa(len(msg))+" "+msg)
	if len(got) != 1 || got[0] != msg {
		t.Fatalf("auto octet detection wrong: %#v", got)
	}
	if a1.mode != 1 {
		t.Fatalf("expected octet mode, got %d", a1.mode)
	}

	// LF stream (starts with '<').
	a2 := newFrameDelimiter("auto", 4096).(*autoDelimiter)
	got = feed(a2, "<134>1 one\n<22>1 two\n")
	if len(got) != 1 || got[0] != "<134>1 one" {
		t.Fatalf("auto lf detection wrong: %#v", got)
	}
	if a2.mode != 2 {
		t.Fatalf("expected lf mode, got %d", a2.mode)
	}
}

// #33: explicit lf mode stays on the syslog delimiter.
func TestAutoDelimiter_ExplicitLF(t *testing.T) {
	d := newFrameDelimiter("lf", 4096)
	got := feed(d, "<12>1 sshd is down\n<145>1 sshd is up")
	if len(got) != 1 || got[0] != "<12>1 sshd is down" {
		t.Fatalf("lf framing wrong: %#v", got)
	}
	// flushRemainder releases the buffered final message.
	if msg, ok := d.flushRemainder(); !ok || msg != "<145>1 sshd is up" {
		t.Fatalf("flushRemainder wrong: %#v %v", msg, ok)
	}
}

// #35: LogHandler counts unparsed lines and forwards raw-line fallback events.
func TestLogHandler_UnparsedCounter(t *testing.T) {
	p, err := NewParser("syslog")
	if err != nil {
		t.Fatal(err)
	}
	// Valid line parses; garbage does not.
	if !p.Parse([]byte(`<134>1 2003-08-24T05:14:15.000003-07:00 ubuntu sshd 1999 - ok`)) {
		t.Fatal("valid line rejected")
	}
	if p.Parse([]byte("garbage line")) {
		t.Fatal("garbage accepted")
	}
	// The unparsed counter must have been incremented on the input stats map.
	v := stats.Get("unparsed")
	if v == nil {
		t.Fatal("unparsed counter not present on input stats")
	}
	if v.String() == "0" {
		t.Fatal("unparsed counter not incremented")
	}
}
