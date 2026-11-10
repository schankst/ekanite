package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"runtime"

	"github.com/schankst/ekanite"
	"github.com/schankst/ekanite/input"
	"github.com/schankst/ekanite/status"
)

func startTCPCollector(iface, format, framing string, tls *tls.Config, batcher *ekanite.Batcher) (input.Collector, error) {
	collector, err := input.NewCollector("tcp", iface, format, framing, tls)
	if err != nil {
		return nil, fmt.Errorf("failed to create TCP collector: %w", err)
	}
	if err := collector.Start(batcher.C()); err != nil {
		return nil, fmt.Errorf("failed to start TCP collector: %w", err)
	}

	return collector, nil
}

func startUDPCollector(iface, format string, batcher *ekanite.Batcher) (input.Collector, error) {
	collector, err := input.NewCollector("udp", iface, format, "", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create UDP collector: %w", err)
	}
	if err := collector.Start(batcher.C()); err != nil {
		return nil, fmt.Errorf("failed to start UDP collector: %w", err)
	}

	return collector, nil
}

func startQueryServer(iface string, engine *ekanite.Engine) *ekanite.Server {
	server := ekanite.NewServer(iface, engine)
	if server == nil {
		log.Fatal("failed to create query server")
	}
	if err := server.Start(); err != nil {
		log.Fatalf("failed to start query server: %s", err.Error())
	}
	log.Printf("query server listening on %s", iface)
	return server
}

func startHTTPQueryServer(iface string, engine *ekanite.Engine) *ekanite.HTTPServer {
	server := ekanite.NewHTTPServer(iface, engine)
	if server == nil {
		log.Fatal("failed to create HTTP query server")
	}
	if err := server.Start(); err != nil {
		log.Fatalf("failed to start HTTP query server: %s", err.Error())
	}
	log.Printf("HTTP query server listening on %s", iface)
	return server
}

func startDiagServer(iface string) *status.Service {
	diagServer := status.NewService(iface)
	diagServer.BuildInfo = map[string]any{
		"version":    version,
		"commit":     commit,
		"go_version": runtime.Version(),
	}
	if err := diagServer.Start(); err != nil {
		log.Fatalf("failed to start status server on %s: %s", iface, err.Error())
	}
	log.Printf("diagnostic server listening on %s", iface)
	return diagServer
}

// closers collects started components so shutdown can close them in order
// (Issue #49). Close is idempotent per component.
type closers struct {
	collectors []input.Collector
	diag       *status.Service
	query      *ekanite.Server
	queryHTTP  *ekanite.HTTPServer
}

// stop shuts down in dependency order: acceptors/listeners first so no new
// work arrives; the batcher and engine are stopped by the caller after this
// (see shutdown in main).
func (c *closers) stop() {
	if c.diag != nil {
		c.diag.Close()
	}
	if c.queryHTTP != nil {
		_ = c.queryHTTP.Close()
	}
	if c.query != nil {
		_ = c.query.Close()
	}
	for _, col := range c.collectors {
		_ = col.Close()
	}
}

// drainLog drains errors from the channel and simply logs them.
func drainLog(msg string, errChan <-chan error) {
	for err := range errChan {
		if err != nil {
			log.Printf("%s: %s", msg, err.Error())
		}
	}
}
