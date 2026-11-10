package parser

import (
	"testing"
	"time"
)

func parseBSD(raw string) (map[string]interface{}, bool) {
	p := &BSD{}
	p.Init()
	res := map[string]interface{}{}
	p.Parse([]byte(raw), &res)
	return res, len(res) > 0
}

func nowYear(t *testing.T) int {
	return time.Now().Year()
}

// #36: Grundform RFC 3164 "Mmm dd hh:mm:ss host app[pid]: msg".
func TestBSD_Basic(t *testing.T) {
	res, ok := parseBSD("Oct 31 10:00:00 host1 app1[123]: hello world")
	if !ok {
		t.Fatal("basic BSD line rejected")
	}
	if res["host"] != "host1" || res["app"] != "app1" || res["pid"] != 123 {
		t.Errorf("fields wrong: %v", res)
	}
	if res["message"] != "hello world" {
		t.Errorf("message wrong: %q", res["message"])
	}
	ts := res["timestamp"].(string)
	parsed, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		t.Fatalf("timestamp not RFC3339: %q (%v)", ts, err)
	}
	if parsed.Year() != nowYear(t) || parsed.Month() != time.October || parsed.Day() != 31 {
		t.Errorf("timestamp parts wrong: %v", parsed)
	}
	if parsed.Hour() != 10 || parsed.Minute() != 0 || parsed.Second() != 0 {
		t.Errorf("timestamp time wrong: %v", parsed)
	}
}

// #36: Ohne PID: "app: msg" — App ohne Klammern, kein pid-Feld.
func TestBSD_NoPID(t *testing.T) {
	res, ok := parseBSD("Jan  2 03:04:05 myhost su: 'su root' failed")
	if !ok {
		t.Fatal("no-pid line rejected")
	}
	if res["app"] != "su" || res["pid"] != nil {
		t.Errorf("app/pid wrong: %v", res)
	}
	if res["host"] != "myhost" {
		t.Errorf("host wrong: %v", res)
	}
	if res["message"] != "'su root' failed" {
		t.Errorf("message wrong: %q", res["message"])
	}
	// Day is space-padded (single digit with leading space).
	tstamp := res["timestamp"].(string)
	parsed, err := time.Parse(time.RFC3339, tstamp)
	if err != nil {
		t.Fatalf("timestamp not RFC3339: %q (%v)", tstamp, err)
	}
	if parsed.Day() != 2 {
		t.Errorf("day wrong: %v", parsed)
	}
}

// #36: Optional führendes PRI wird verarbeitet (facility/severity).
func TestBSD_PrefixPRI(t *testing.T) {
	res, ok := parseBSD("<34>Oct 11 22:14:15 mymachine su[123]: msg here")
	if !ok {
		t.Fatal("PRI-prefixed line rejected")
	}
	if res["priority"] != 34 || res["facility"] != 4 || res["severity"] != 2 {
		t.Errorf("PRI fields wrong: %v", res)
	}
	if res["app"] != "su" {
		t.Errorf("app wrong: %v", res)
	}
}

// #36: Monatsnamen — alle 12.
func TestBSD_Months(t *testing.T) {
	for month, num := range bsdMonths {
		res, ok := parseBSD(month + " 01 12:00:00 host app[1]: msg")
		if !ok {
			t.Fatalf("%s rejected", month)
		}
		parsed, _ := time.Parse(time.RFC3339, res["timestamp"].(string))
		if parsed.Month() != num {
			t.Errorf("%s -> month %v, want %v", month, parsed.Month(), num)
		}
	}
}

// #36: Mehrzeilige Message (MSG darf beliebig sein).
func TestBSD_MultiLine(t *testing.T) {
	res, ok := parseBSD("Oct 11 22:14:15 mymachine app[1]: first line\nsecond line")
	if !ok {
		t.Fatal("multiline rejected")
	}
	if res["message"] != "first line\nsecond line" {
		t.Errorf("multiline message wrong: %q", res["message"])
	}
}

// #36: Fehlende Message (leerer MSG-Teil nach Doppelpunkt).
func TestBSD_EmptyMessage(t *testing.T) {
	res, ok := parseBSD("Oct 11 22:14:15 mymachine app[1]:")
	if !ok {
		t.Fatal("empty-msg rejected")
	}
	if res["message"] != "" {
		t.Errorf("empty message wrong: %q", res["message"])
	}
}

// #36: Ungültige Zeile -> kein Ergebnis (Parser zählt als unparsed).
func TestBSD_Invalid(t *testing.T) {
	for _, raw := range []string{
		"",
		"hello world",
		"Oct 31 10:00:00",           // fehlt host/app
		"Octob 31 10:00:00 h a - m", // falscher Monatsname (4 Buchstaben)
	} {
		if res, ok := parseBSD(raw); ok {
			t.Errorf("invalid line accepted: %q -> %v", raw, res)
		}
	}
}

// #36: App mit Bindestrich/Punkt (kein PID).
func TestBSD_AppWithSpecialChars(t *testing.T) {
	res, ok := parseBSD("Oct 11 22:14:15 mymachine systemd-logind: session opened")
	if !ok {
		t.Fatal("app-with-dash rejected")
	}
	if res["app"] != "systemd-logind" {
		t.Errorf("app wrong: %v", res)
	}
	if res["message"] != "session opened" {
		t.Errorf("message wrong: %q", res["message"])
	}
}

// #36: PID mit mehreren Ziffern.
func TestBSD_LongPID(t *testing.T) {
	res, ok := parseBSD("Dec 25 23:59:59 srv cron[42317]: (root) CMD (ls)")
	if !ok {
		t.Fatal("long-pid rejected")
	}
	if res["pid"] != 42317 || res["app"] != "cron" {
		t.Errorf("pid/app wrong: %v", res)
	}
}

// #36: Über loghandler (Registry) — -input bsd wählbar.
func TestBSD_Registry(t *testing.T) {
	// bsdName -> BSDStandard mapping via parser package contract is
	// exercised in input/parser_test; here just assert the BSD parses what
	// rsyslog forwards by default.
	for _, raw := range []string{
		"Oct 31 10:00:00 host1 app1[123]: hello",
		"<34>Oct 31 10:00:00 host1 app1[123]: hello",
		"Oct 31 10:00:00 host1 app1: hello",
	} {
		if _, ok := parseBSD(raw); !ok {
			t.Errorf("expected success for %q", raw)
		}
	}
}
