package parser

import (
	"reflect"
	"testing"
)

func parseRFC(raw string) (map[string]interface{}, bool) {
	p := &RFC5424{}
	p.Init()
	res := map[string]interface{}{}
	p.Parse([]byte(raw), &res)
	return res, len(res) > 0
}

// #17: PRI-Bereich 0-191 wird validiert; Außerhalb wird abgelehnt (Parse miss).
func TestRFC5424_PRI_ValidRange(t *testing.T) {
	valid := []string{
		`<0>1 2024-01-01T00:00:00Z h a 1 - m`,
		`<191>1 2024-01-01T00:00:00Z h a 1 - m`,
	}
	for _, raw := range valid {
		res, ok := parseRFC(raw)
		if !ok {
			t.Fatalf("valid PRI line rejected: %q", raw)
		}
		if res["facility"] != res["priority"].(int)/8 || res["severity"] != res["priority"].(int)%8 {
			t.Errorf("facility/severity mismatch for %q: %v", raw, res)
		}
	}
	invalid := []string{
		`<192>1 2024-01-01T00:00:00Z h a 1 - m`,
		`<999>1 2024-01-01T00:00:00Z h a 1 - m`,
	}
	for _, raw := range invalid {
		if _, ok := parseRFC(raw); ok {
			t.Errorf("invalid PRI line accepted: %q", raw)
		}
	}
}

// #17: facility/severity werden aus PRI abgeleitet.
func TestRFC5424_FacilitySeverity(t *testing.T) {
	res, ok := parseRFC(`<134>1 2024-01-01T00:00:00Z host app 1 - msg`)
	if !ok {
		t.Fatal("parse failed")
	}
	if res["facility"] != 16 || res["severity"] != 6 {
		t.Errorf("expected facility=16 severity=6, got %v/%v", res["facility"], res["severity"])
	}
}

// #17: NILVALUE "-" wird auf nil normalisiert; BOM wird gestrippt.
func TestRFC5424_NilValueAndBOM(t *testing.T) {
	res, ok := parseRFC("\xef\xbb\xbf<14>1 2024-01-01T00:00:00Z host app - - msg")
	if !ok {
		t.Fatal("parse failed")
	}
	if res["pid"] != nil || res["message_id"] != nil {
		t.Errorf("NILVALUE pid/msgid not normalized: %v", res)
	}
	if res["message"] != "msg" {
		t.Errorf("BOM not stripped from message: %v", res)
	}
	if res["host"] != "host" {
		t.Errorf("host should not be nil: %v", res)
	}
}

// #17: nil timestamp bei NILVALUE, nicht "-".
func TestRFC5424_NilTimestamp(t *testing.T) {
	res, ok := parseRFC(`<14>1 - host app 1 - msg`)
	if !ok {
		t.Fatal("parse failed")
	}
	if res["timestamp"] != nil {
		t.Errorf("NILVALUE timestamp not nil: %v", res)
	}
}

// #15: Structured Data wird extrahiert; message enthält nur noch MSG.
func TestRFC5424_StructuredData(t *testing.T) {
	res, ok := parseRFC(`<165>1 2003-08-24T05:14:15.000003-07:00 192.0.2.1 myproc 8710 - [exampleSDID@32473 iut="3" eventSource="App"] An application event log entry...`)
	if !ok {
		t.Fatal("parse failed")
	}
	if res["message"] != "An application event log entry..." {
		t.Errorf("message not cleaned of SD: %q", res["message"])
	}
	sd, ok := res["structured_data"].([]map[string]interface{})
	if !ok || len(sd) != 1 {
		t.Fatalf("structured_data missing/incorrect: %v", res["structured_data"])
	}
	elem := sd[0]["exampleSDID@32473"].(map[string]interface{})
	if elem["iut"] != "3" || elem["eventSource"] != "App" {
		t.Errorf("SD params not parsed: %v", elem)
	}
}

// #15: Mehrere SD-Elemente.
func TestRFC5424_MultipleSD(t *testing.T) {
	res, ok := parseRFC(`<165>1 2003-08-24T05:14:15.000003-07:00 h a 1 - [id1 k="v"][id2 k2="w"] body`)
	if !ok {
		t.Fatal("parse failed")
	}
	sd := res["structured_data"].([]map[string]interface{})
	if len(sd) != 2 {
		t.Fatalf("expected 2 SD elements, got %d", len(sd))
	}
	if res["message"] != "body" {
		t.Errorf("message after multiple SD wrong: %q", res["message"])
	}
}

// #15: Kein SD -> kein structured_data Feld, message unverändert.
func TestRFC5424_NoSD(t *testing.T) {
	res, ok := parseRFC(`<165>1 2003-08-24T05:14:15.000003-07:00 h a 1 - plain message`)
	if !ok {
		t.Fatal("parse failed")
	}
	if _, has := res["structured_data"]; has {
		t.Errorf("structured_data should not be set: %v", res)
	}
	if res["message"] != "plain message" {
		t.Errorf("message wrong: %q", res["message"])
	}
}

// #16: VERSION 1-3 Ziffern (nicht nur 1).
func TestRFC5424_VersionUpTo3(t *testing.T) {
	for _, ver := range []string{"1", "23", "321"} {
		res, ok := parseRFC(`<14>` + ver + ` 2024-01-01T00:00:00Z h a 1 - m`)
		if !ok {
			t.Fatalf("version %q rejected", ver)
		}
		want := 0
		for _, c := range ver {
			want = want*10 + int(c-'0')
		}
		if res["version"] != want {
			t.Errorf("version %q parsed as %v", ver, res["version"])
		}
	}
}

// #16: Alphanumerische PROCID (RFC PRINTUSASCII) wird akzeptiert.
func TestRFC5424_AlphaProcID(t *testing.T) {
	res, ok := parseRFC(`<14>1 2024-01-01T00:00:00Z h a web-01 - m`)
	if !ok {
		t.Fatal("alphanumeric PROCID rejected")
	}
	if res["pid"] != "web-01" {
		t.Errorf("pid = %v, want web-01", res["pid"])
	}
}

// #16: MSGID mit Punkt (RFC PRINTUSASCII) wird akzeptiert.
func TestRFC5424_MsgIDWithDot(t *testing.T) {
	res, ok := parseRFC(`<14>1 2024-01-01T00:00:00Z h a 1 msg.id.1 m`)
	if !ok {
		t.Fatal("dotted MSGID rejected")
	}
	if res["message_id"] != "msg.id.1" {
		t.Errorf("message_id = %v, want msg.id.1", res["message_id"])
	}
}

// #16: Nicht-Numerische PROCID wird als String übernommen, nicht als 0.
func TestRFC5424_NonNumericProcID(t *testing.T) {
	res, ok := parseRFC(`<14>1 2024-01-01T00:00:00Z h a systemd-123 - m`)
	if !ok {
		t.Fatal("parse failed")
	}
	if res["pid"] != "systemd-123" {
		t.Errorf("pid = %v, want systemd-123", res["pid"])
	}
}

// Parse übereinstimmt mit gofmt der Expected-Maps in parser_test.go.
func TestRFC5424_DeepEqualStable(t *testing.T) {
	res, ok := parseRFC(`<33>5 1985-04-12T23:20:50.52Z test.com cron 304 - password accepted`)
	if !ok {
		t.Fatal("parse failed")
	}
	want := map[string]interface{}{
		"priority":   33,
		"facility":   4,
		"severity":   1,
		"version":    5,
		"timestamp":  "1985-04-12T23:20:50.52Z",
		"host":       "test.com",
		"app":        "cron",
		"pid":        304,
		"message_id": nil,
		"message":    "password accepted",
	}
	if !reflect.DeepEqual(want, res) {
		t.Errorf("result mismatch:\n got %v\nwant %v", res, want)
	}
}
