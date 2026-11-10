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

// startCollector creates and starts a network collector. proto is "tcp" or
// "udp"; tls/framing are ignored by the UDP collector.
func startCollector(proto, iface, format, framing string, tls *tls.Config, batcher *ekanite.Batcher) (input.Collector, error) {
	collector, err := input.NewCollector(proto, iface, format, framing, tls)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s collector: %w", proto, err)
	}
	if err := collector.Start(batcher.C()); err != nil {
		return nil, fmt.Errorf("failed to start %s collector: %w", proto, err)
	}

	return collector, nil
}

// startQueryServers starts the telnet and/or HTTP query servers per config.
func startQueryServers(cfg *Config, engine *ekanite.Engine, comp *closers) {
	if cfg.QueryIface != "" {
		comp.query = startQueryServer(cfg.QueryIface, engine)
	}
	if cfg.QueryHTTPIface != "" {
		comp.queryHTTP = startHTTPQueryServer(cfg.QueryHTTPIface, engine)
	}
}

// startIngestCollectors starts the TCP (with optional TLS) and UDP collectors
// per config. It returns the TLS certificate reloader (nil when TLS is off) so
// the signal handler can reload certificates on SIGHUP (Issue #74).
func startIngestCollectors(cfg *Config, batcher *ekanite.Batcher, comp *closers) (*certReloader, error) {
	var certReload *certReloader

	if cfg.TCPIface != "" {
		var tlsConfig *tls.Config
		if cfg.TLSCert != "" && cfg.TLSKey != "" {
			var err error
			tlsConfig, certReload, err = newTLSConfig(cfg.TLSCert, cfg.TLSKey)
			if err != nil {
				return nil, fmt.Errorf("failed to configure TLS: %w", err)
			}
			log.Printf("TLS successfully configured (cert reload on SIGHUP)")
		}

		tcpCol, err := startCollector("tcp", cfg.TCPIface, cfg.InputFormat, cfg.Framing, tlsConfig, batcher)
		if err != nil {
			return nil, fmt.Errorf("failed to start TCP collector: %w", err)
		}
		comp.collectors = append(comp.collectors, tcpCol)
		log.Printf("TCP collector listening to %s", cfg.TCPIface)
	}

	if cfg.UDPIface != "" {
		udpCol, err := startCollector("udp", cfg.UDPIface, cfg.InputFormat, "", nil, batcher)
		if err != nil {
			return nil, fmt.Errorf("failed to start UDP collector: %w", err)
		}
		comp.collectors = append(comp.collectors, udpCol)
		log.Printf("UDP collector listening to %s", cfg.UDPIface)
	}

	return certReload, nil
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
