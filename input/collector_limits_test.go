package input

import (
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

// TestTCPCollector_IdleTimeoutClosesConn verifies Issue #14: a connection that
// goes idle (no bytes) is closed once the read deadline elapses, instead of
// being kept open forever by a re-arming loop.
func TestTCPCollector_IdleTimeoutClosesConn(t *testing.T) {
	s := &TCPCollector{
		iface:       "127.0.0.1:0",
		format:      "syslog",
		framing:     "lf",
		readTimeout: 150 * time.Millisecond,
	}
	c := make(chan *Event, 8)
	if err := s.Start(c); err != nil {
		t.Fatalf("failed to start collector: %s", err.Error())
	}
	defer s.Close()

	conn, err := net.Dial("tcp", s.Addr().String())
	if err != nil {
		t.Fatalf("dial: %s", err.Error())
	}
	defer conn.Close()

	// Send nothing; the collector must close the idle connection. A server-side
	// close surfaces as io.EOF on the client; a client-side read timeout (the
	// buggy behavior, connection still open) must not satisfy this.
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
		t.Fatalf("idle connection was not closed by the server (Issue #14): %v", err)
	}
}

// TestTCPCollector_ConnectionLimit verifies Issue #14: connections beyond the
// cap are rejected instead of spawning unbounded goroutines.
func TestTCPCollector_ConnectionLimit(t *testing.T) {
	s := &TCPCollector{
		iface:       "127.0.0.1:0",
		format:      "syslog",
		framing:     "lf",
		readTimeout: time.Hour, // keep accepted connections open
		maxConns:    2,
	}
	c := make(chan *Event, 8)
	if err := s.Start(c); err != nil {
		t.Fatalf("failed to start collector: %s", err.Error())
	}
	defer s.Close()

	addr := s.Addr().String()
	var held []net.Conn
	for i := 0; i < 2; i++ {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatalf("dial %d: %s", i, err.Error())
		}
		held = append(held, conn)
	}
	defer func() {
		for _, cn := range held {
			cn.Close()
		}
	}()

	// Give the accept loop time to consume both slots.
	time.Sleep(200 * time.Millisecond)

	// The next connection exceeds the cap and must be closed by the collector.
	over, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial over-limit: %s", err.Error())
	}
	defer over.Close()
	_ = over.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := over.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
		t.Fatalf("connection beyond the cap was not rejected (Issue #14): %v", err)
	}
}
