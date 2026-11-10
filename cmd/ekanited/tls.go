package main

import (
	"crypto/tls"
	"sync"
)

// certReloader holds the currently loaded TLS server certificate and swaps it
// atomically on reload, so a running server serves rotated certificates without
// restart (Issue #74).
type certReloader struct {
	mu       sync.RWMutex
	certPath string
	keyPath  string
	cert     *tls.Certificate
}

// GetCertificate implements tls.Config.GetCertificate: it is called per
// handshake, so a reload takes effect for new connections immediately.
func (r *certReloader) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cert, nil
}

// Reload re-reads the certificate and key files and swaps them in atomically.
// On error the previously loaded certificate stays in effect.
func (r *certReloader) Reload() error {
	cert, err := tls.LoadX509KeyPair(r.certPath, r.keyPath)
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.cert = &cert
	r.mu.Unlock()
	return nil
}

// newTLSConfig builds a server TLS config for the TCP ingest (RFC 5425) using
// tls.LoadX509KeyPair, which supports RSA, ECDSA and Ed25519 keys plus full
// certificate chains. TLS 1.2 is the minimum protocol version. No client
// certificates are required — the sender authenticates on the network level.
func newTLSConfig(certPath, keyPath string) (*tls.Config, *certReloader, error) {
	reloader := &certReloader{certPath: certPath, keyPath: keyPath}
	if err := reloader.Reload(); err != nil {
		return nil, nil, err
	}
	config := &tls.Config{
		GetCertificate: reloader.GetCertificate,
		MinVersion:     tls.VersionTLS12,
	}
	return config, reloader, nil
}
