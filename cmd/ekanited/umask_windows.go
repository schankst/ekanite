//go:build windows

package main

// restrictFileCreation limits newly created files to the owner only (Issue
// #90). Windows has no process umask; the equivalent is enforced by the
// explicit 0700/0600 modes applied in securePermissions (index.go) and by the
// audit log's 0600 open mode. This is intentionally a no-op so the binary
// cross-compiles for Windows (Issue #138).
func restrictFileCreation() {}
