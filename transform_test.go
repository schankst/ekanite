package ekanite

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/ekanite/ekanite/input"
)

func transformEvent() *Event {
	return &Event{&input.Event{
		Text: `<165>1 2003-08-24T05:14:15.000003-07:00 10.1.2.3 myproc 8710 - [exampleSDID@32473 iut="3"] raw secret line`,
		Parsed: map[string]interface{}{
			"priority":   165,
			"facility":   20,
			"severity":   5,
			"version":    1,
			"timestamp":  "2003-08-24T05:14:15.000003-07:00",
			"host":       "10.1.2.3",
			"app":        "myproc",
			"pid":        8710,
			"message_id": nil,
			"message":    "raw secret line",
		},
		ReceptionTime: time.Date(2003, 8, 24, 12, 0, 0, 0, time.UTC),
		Sequence:      1,
		SourceIP:      "10.1.2.3",
	}}
}

// #93: inet masks the last IPv4 octet.
func TestMaskIP_IPv4(t *testing.T) {
	if got := maskIP("10.1.2.3"); got != "10.1.2.0" {
		t.Errorf("maskIP(10.1.2.3) = %q, want 10.1.2.0", got)
	}
	if got := maskIP("192.168.1.100"); got != "192.168.1.0" {
		t.Errorf("maskIP(192.168.1.100) = %q", got)
	}
}

// #93: inet masks the last IPv6 hextet to zero.
func TestMaskIP_IPv6(t *testing.T) {
	if got := maskIP("2001:db8::1"); got != "2001:db8::" {
		t.Errorf("maskIP(2001:db8::1) = %q, want 2001:db8::", got)
	}
	if got := maskIP("fe80::1"); got != "fe80::" {
		t.Errorf("maskIP(fe80::1) = %q, want fe80::", got)
	}
}

// #93: non-IP input is returned unchanged (no silent alteration).
func TestMaskIP_NonIP(t *testing.T) {
	if got := maskIP("myproc"); got != "myproc" {
		t.Errorf("maskIP(non-ip) = %q", got)
	}
}

// #93: hmac is deterministic for the same key and differs by key.
func TestHmacHex(t *testing.T) {
	key := []byte("sekret")
	a := hmacHex("10.1.2.3", key)
	b := hmacHex("10.1.2.3", key)
	if a == "" || a != b {
		t.Fatalf("hmac not deterministic: %q vs %q", a, b)
	}
	if a == "10.1.2.3" {
		t.Fatal("hmac must not equal the plaintext")
	}
	if c := hmacHex("10.1.2.3", []byte("other")); c == a {
		t.Fatal("hmac must differ across keys")
	}
	// Empty key -> empty result (no unkeyed hash).
	if got := hmacHex("x", nil); got != "" {
		t.Fatalf("hmac with empty key = %q, want empty", got)
	}
}

// #93: Data() anonymizes configured fields; original values absent.
func TestData_Anonymize(t *testing.T) {
	SetTransform(TransformConfig{
		Anonymize: map[string]string{"source_ip": "inet", "host": "hmac"},
		AnonKey:   []byte("k"),
	})
	defer SetTransform(TransformConfig{})

	e := transformEvent()
	doc, ok := e.Data().(map[string]interface{})
	if !ok {
		t.Fatal("Data() must return a map")
	}
	// source_ip masked.
	if got := doc["source_ip"]; got != "10.1.2.0" {
		t.Errorf("source_ip = %v, want 10.1.2.0", got)
	}
	// host hashed (hex, != plaintext).
	host, ok := doc["host"].(string)
	if !ok || host == "10.1.2.3" || len(host) != 64 {
		t.Errorf("host = %v, want 64-char hmac", doc["host"])
	}
	// Raw line excluded (dropRaw due to anonymization).
	if _, has := doc["Message"]; has {
		t.Error("Message must be excluded when anonymizing")
	}
}

// #93+#95: Source() must not contain the raw line (with its PRI/timestamp
// header) nor the original IP when anonymization is active; the parsed
// message field legitimately stays.
func TestSource_AnonymizeNoLeak(t *testing.T) {
	SetTransform(TransformConfig{
		Anonymize: map[string]string{"source_ip": "inet", "host": "inet"},
		AnonKey:   []byte("k"),
	})
	defer SetTransform(TransformConfig{})

	src := transformEvent().Source()
	if len(src) == 0 {
		t.Fatal("source must contain the serialized fields")
	}
	var m map[string]interface{}
	if err := json.Unmarshal(src, &m); err != nil {
		t.Fatalf("source not valid JSON: %v", err)
	}
	if m["source_ip"] != "10.1.2.0" {
		t.Errorf("source source_ip = %v", m["source_ip"])
	}
	raw := string(src)
	if contains(raw, "<165>1 2003-08-24") {
		t.Error("raw line header leaked into source")
	}
	if contains(raw, "10.1.2.3") {
		t.Error("original IP leaked into source")
	}
}

// #95: -raw drop excludes Message and returns serialized fields in Source.
func TestData_DropRaw(t *testing.T) {
	SetTransform(TransformConfig{RawMode: "drop"})
	defer SetTransform(TransformConfig{})

	e := transformEvent()
	doc, ok := e.Data().(map[string]interface{})
	if !ok {
		t.Fatal("Data() must return a map")
	}
	if _, has := doc["Message"]; has {
		t.Error("Message must be excluded in drop mode")
	}
	if _, has := doc["app"]; !has {
		t.Error("parsed fields must remain in drop mode")
	}
	// Source serializes fields, no raw line header.
	src := e.Source()
	if contains(string(src), "<165>1 2003-08-24") {
		t.Error("raw line header leaked in drop mode")
	}
	var m map[string]interface{}
	if err := json.Unmarshal(src, &m); err != nil && len(src) > 0 {
		t.Fatalf("source not JSON: %v", err)
	}
}

// #95: keep mode (default) preserves Message and raw source.
func TestData_KeepRaw(t *testing.T) {
	SetTransform(TransformConfig{RawMode: "keep"})
	defer SetTransform(TransformConfig{})

	e := transformEvent()
	doc, ok := e.Data().(map[string]interface{})
	if !ok {
		t.Fatal("Data() must return a map")
	}
	if doc["Message"] != e.Text {
		t.Errorf("Message = %v, want raw line", doc["Message"])
	}
	if string(e.Source()) != e.Text {
		t.Error("Source must be the raw line in keep mode")
	}
}

// #93: fallback events (raw key) never leak the raw line through Source.
func TestSource_FallbackNoLeak(t *testing.T) {
	SetTransform(TransformConfig{RawMode: "drop"})
	defer SetTransform(TransformConfig{})

	e := &Event{&input.Event{
		Text:          "unparsable raw text",
		Parsed:        map[string]interface{}{"raw": "unparsable raw text", "unparsed": true},
		ReceptionTime: time.Now().UTC(),
		Sequence:      1,
	}}
	src := e.Source()
	if contains(string(src), "unparsable raw text") {
		t.Fatalf("fallback raw leaked: %s", src)
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
