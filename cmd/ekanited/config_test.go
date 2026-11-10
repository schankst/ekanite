package main

import (
	"strings"
	"testing"
	"time"
)

// TestParseConfig_FlagWinsOverEnv verifies that an explicit CLI flag overrides
// the environment-derived default (flag-wins-over-env).
func TestParseConfig_FlagWinsOverEnv(t *testing.T) {
	t.Setenv("EKANITE_DATADIR", "/from/env")

	cfg, err := parseConfig([]string{"-datadir", "/from/flag"})
	if err != nil {
		t.Fatalf("parseConfig: %s", err.Error())
	}
	if cfg.DataDir != "/from/flag" {
		t.Fatalf("DataDir = %q, want /from/flag", cfg.DataDir)
	}
}

// TestParseConfig_EnvUsed verifies that EKANITE_<FLAGNAME> provides the default.
func TestParseConfig_EnvUsed(t *testing.T) {
	t.Setenv("EKANITE_BATCHSIZE", "123")

	cfg, err := parseConfig(nil)
	if err != nil {
		t.Fatalf("parseConfig: %s", err.Error())
	}
	if cfg.BatchSize != 123 {
		t.Fatalf("BatchSize = %d, want 123", cfg.BatchSize)
	}
}

// TestParseConfig_Durations verifies duration parsing (incl. the "d" suffix)
// and the millisecond->duration batch timeout.
func TestParseConfig_Durations(t *testing.T) {
	cfg, err := parseConfig([]string{"-retention", "7d", "-batchtime", "1500"})
	if err != nil {
		t.Fatalf("parseConfig: %s", err.Error())
	}
	if cfg.Retention != 7*24*time.Hour {
		t.Fatalf("Retention = %s, want 168h", cfg.Retention)
	}
	if cfg.BatchTimeout != 1500*time.Millisecond {
		t.Fatalf("BatchTimeout = %s, want 1.5s", cfg.BatchTimeout)
	}
}

// TestParseConfig_Anonymize verifies parsing of the -anonymize spec.
func TestParseConfig_Anonymize(t *testing.T) {
	cfg, err := parseConfig([]string{"-anonymize", "source_ip=inet, host=hmac"})
	if err != nil {
		t.Fatalf("parseConfig: %s", err.Error())
	}
	if len(cfg.Anonymize) != 2 || cfg.Anonymize["source_ip"] != "inet" || cfg.Anonymize["host"] != "hmac" {
		t.Fatalf("Anonymize = %v, want source_ip=inet,host=hmac", cfg.Anonymize)
	}
}

// TestParseConfig_Invalid verifies that bad values are reported as errors
// (instead of panicking), so the parsing logic is unit-testable.
func TestParseConfig_Invalid(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"bad duration", []string{"-retention", "zzz"}, "retention"},
		{"bad anonymize algo", []string{"-anonymize", "x=bogus"}, "algorithm"},
		{"bad anonymize entry", []string{"-anonymize", "noequals"}, "entry"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseConfig(tc.args)
			if err == nil {
				t.Fatalf("expected error for %v, got nil", tc.args)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not mention %q", err.Error(), tc.want)
			}
		})
	}
}
