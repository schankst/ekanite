package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

// waitForSignals handles runtime signals until shutdown: SIGHUP triggers a TLS
// certificate reload (Issue #74), SIGINT/SIGTERM initiate shutdown. reload may
// be nil when TLS is not configured.
func waitForSignals(reload func() error) {
	// Set up signal handling.
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	// Block until a shutdown signal is received; SIGHUP only reloads.
	for sig := range signalCh {
		if sig == syscall.SIGHUP {
			if reload == nil {
				log.Println("SIGHUP received, no TLS configured")
				continue
			}
			if err := reload(); err != nil {
				log.Printf("TLS reload failed: %s", err.Error())
			} else {
				log.Println("TLS certificates reloaded")
			}
			continue
		}
		log.Println("signal received, shutting down...")
		return
	}
}
