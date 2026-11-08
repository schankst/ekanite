package input

import (
	"expvar"
	"net"
	"testing"
	"time"
)

// TestTCPCollector_ParserError_KeepsServing verifies Issue #51: a NewParser
// failure inside the connection handler must close the connection and leave
// the process running (no panic), counted in tcpParserErrors.
func TestTCPCollector_ParserError_KeepsServing(t *testing.T) {
	// Construct the collector directly with an unsupported format, bypassing
	// NewCollector's upfront validation.
	c := &TCPCollector{iface: "127.0.0.1:0", format: "no-such-format", framing: "lf"}

	client, server := net.Pipe()
	defer client.Close()

	events := make(chan *Event, 8)
	done := make(chan struct{})
	go func() {
		c.handleConnection(server, events)
		close(done)
	}()

	// Write a line so the handler reads something; then it should hit the
	// parser-error path and return.
	go func() {
		client.Write([]byte("hello\n"))
		client.Close()
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("handleConnection did not return after parser error")
	}
	// Server side must be closed by the handler's deferred close.
	if _, err := server.Read(make([]byte, 1)); err == nil {
		t.Fatal("server connection not closed after parser error")
	}
	if got := stats.Get("tcpParserErrors").(*expvar.Int).Value(); got < 1 {
		t.Fatalf("tcpParserErrors = %d, want >= 1", got)
	}
}
