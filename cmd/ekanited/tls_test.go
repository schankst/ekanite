package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// genTestCert writes a self-signed Ed25519 server certificate for "localhost"
// to certPath/keyPath in the given directory and returns the parsed certificate.
func genTestCert(t *testing.T, dir, name string) *x509.Certificate {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519 keygen: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, pub, priv)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}
	writePEM(t, filepath.Join(dir, "cert.pem"), "CERTIFICATE", der)
	keyDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	writePEM(t, filepath.Join(dir, "key.pem"), "PRIVATE KEY", keyDER)
	return cert
}

func writePEM(t *testing.T, path, blockType string, der []byte) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	if err := pem.Encode(f, &pem.Block{Type: blockType, Bytes: der}); err != nil {
		t.Fatalf("pem encode %s: %v", path, err)
	}
}

// clientPool returns a TLS client config trusting the given certificate.
func clientPool(cert *x509.Certificate) *tls.Config {
	pool := x509.NewCertPool()
	pool.AddCert(cert)
	return &tls.Config{RootCAs: pool, ServerName: "localhost", MinVersion: tls.VersionTLS12}
}

// dialTLS connects to the listener and returns an error if the handshake fails.
func dialTLS(t *testing.T, addr string, cfg *tls.Config) error {
	t.Helper()
	conn, err := tls.Dial("tcp", addr, cfg)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}

// TestNewTLSConfig_ModernKeys verifies LoadX509KeyPair-based config accepts an
// Ed25519 key and certificate (Issue #74: was PKCS#1-RSA-only before).
func TestNewTLSConfig_ModernKeys(t *testing.T) {
	dir := t.TempDir()
	genTestCert(t, dir, "server-a")
	cfg, _, err := newTLSConfig(filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem"))
	if err != nil {
		t.Fatalf("newTLSConfig with Ed25519: %v", err)
	}
	if cfg.MinVersion != tls.VersionTLS12 {
		t.Errorf("MinVersion = %x, want TLS1.2", cfg.MinVersion)
	}
	if cfg.GetCertificate == nil {
		t.Error("GetCertificate not set, reload path broken")
	}
}

// TestCertRotation_SIGHUP verifies the rotation contract of Issue #74: after
// replacing cert/key files and reloading, new connections present the new
// certificate while the listener keeps running.
func TestCertRotation_SIGHUP(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()
	genTestCert(t, dirA, "server-a")
	genTestCert(t, dirB, "server-b")

	// Server: TLS listener on an ephemeral port, cert from dirA.
	cfg, reloader, err := newTLSConfig(filepath.Join(dirA, "cert.pem"), filepath.Join(dirA, "key.pem"))
	if err != nil {
		t.Fatalf("newTLSConfig: %v", err)
	}
	ln, err := tls.Listen("tcp", "127.0.0.1:0", cfg)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				// Complete the TLS handshake so the client-side verifies the
				// served certificate (handshake is lazy in crypto/tls).
				if tc, ok := c.(*tls.Conn); ok {
					_ = tc.Handshake()
				}
				c.Close()
			}(conn)
		}
	}()

	certA, err := x509.ParseCertificate(mustPEM(t, filepath.Join(dirA, "cert.pem")))
	if err != nil {
		t.Fatal(err)
	}
	certB, err := x509.ParseCertificate(mustPEM(t, filepath.Join(dirB, "cert.pem")))
	if err != nil {
		t.Fatal(err)
	}

	// 1) Old certificate works.
	if err := dialTLS(t, ln.Addr().String(), clientPool(certA)); err != nil {
		t.Fatalf("handshake with cert A before rotation: %v", err)
	}

	// 2) Rotate: new files land at the same paths, reload = what SIGHUP calls.
	copyFile(t, filepath.Join(dirB, "cert.pem"), filepath.Join(dirA, "cert.pem"))
	copyFile(t, filepath.Join(dirB, "key.pem"), filepath.Join(dirA, "key.pem"))
	if err := reloader.Reload(); err != nil {
		t.Fatalf("reload after rotation: %v", err)
	}

	// 3) New certificate works against the same listener.
	if err := dialTLS(t, ln.Addr().String(), clientPool(certB)); err != nil {
		t.Fatalf("handshake with cert B after rotation: %v", err)
	}
	// 4) Old trust anchor now fails — proves the new cert is actually served.
	if err := dialTLS(t, ln.Addr().String(), clientPool(certA)); err == nil {
		t.Fatal("handshake with old cert A after rotation unexpectedly succeeded")
	}
}

// mustPEM returns the DER content of the first PEM block in the file.
func mustPEM(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		t.Fatalf("no PEM block in %s", path)
	}
	return block.Bytes
}

// copyFile copies src to dst byte-for-byte.
func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read %s: %v", src, err)
	}
	writeRaw(t, dst, data)
}

// writeRaw writes exact bytes (cert/key PEM) to path.
func writeRaw(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
