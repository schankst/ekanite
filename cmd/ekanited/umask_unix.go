//go:build !windows

package main

import "syscall"

// restrictFileCreation limits newly created files to the owner only via the
// process umask (Issue #90). On Unix this enforces owner-only access even for
// files whose library uses default modes. No-op on Windows (guarded by build
// tag so cross-compilation succeeds, Issue #138).
func restrictFileCreation() {
	syscall.Umask(0077)
}
