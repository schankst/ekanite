package main

import (
	"runtime"
	"strings"
	"testing"
)

func TestVersionString(t *testing.T) {
	oldVersion, oldCommit := version, commit
	version = "1.4.0"
	commit = "abc123"
	defer func() {
		version = oldVersion
		commit = oldCommit
	}()

	got := versionString()
	want := "ekanite 1.4.0 (abc123) " + runtime.Version()
	if got != want {
		t.Fatalf("unexpected version string: got %q, want %q", got, want)
	}
	if !strings.HasPrefix(got, "ekanite ") {
		t.Fatalf("version string missing prefix: %q", got)
	}
	if !strings.Contains(got, "(abc123)") {
		t.Fatalf("version string missing commit: %q", got)
	}
	if !strings.Contains(got, runtime.Version()) {
		t.Fatalf("version string missing go version: %q", got)
	}
}
