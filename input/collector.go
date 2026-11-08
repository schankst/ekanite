package input

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"expvar"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync/atomic"
	"time"
)

var sequenceNumber int64
var stats = expvar.NewMap("input")

func init() {
	sequenceNumber = time.Now().UnixNano()
}

const (
	newlineTimeout = time.Duration(1000 * time.Millisecond)
	// msgBufSize is the UDP read buffer. RFC 5426 mandates supporting at least
	// 2048 octets; 4096 covers typical datagrams without silently truncating
	// larger legitimate payloads. Datagrams that still exceed it are counted in
	// udpTruncated rather than silently dropped.
	msgBufSize = 4096
	// udpReadBufSize is one byte larger than msgBufSize so a datagram that
	// exceeds the accepted size reads back udpReadBufSize (== msgBufSize+1)
	// bytes, making truncation detectable (ReadFromUDP clips to the buffer).
	udpReadBufSize = msgBufSize + 1
)

// newEvent builds an Event from a parsed log line. If the parser failed to
// parse the line (Parse returned false), the raw line is still indexed as a
// fallback event so no message is silently lost (Issue #16). Such events carry
// Parsed = {"raw": <line>, "unparsed": true} and a nil timestamp, so
// ReferenceTime falls back to ReceptionTime.
func newEvent(text string, parsed map[string]any, source string) *Event {
	if parsed == nil {
		parsed = map[string]any{"raw": text, "unparsed": true}
	}
	return &Event{
		Text:          text,
		Parsed:        parsed,
		ReceptionTime: time.Now().UTC(),
		Sequence:      atomic.AddInt64(&sequenceNumber, 1),
		SourceIP:      source,
	}
}

// Collector specifies the interface all network collectors must implement.
type Collector interface {
	Start(chan<- *Event) error
	Addr() net.Addr
}

// TCPCollector represents a network collector that accepts and handler TCP connections.
type TCPCollector struct {
	iface   string
	format  string
	framing string

	addr      net.Addr
	tlsConfig *tls.Config
}

// UDPCollector represents a network collector that accepts UDP packets.
type UDPCollector struct {
	format string
	addr   *net.UDPAddr
}

// NewCollector returns a network collector of the specified type, that will bind
// to the given inteface on Start(). If config is non-nil, a secure Collector will
// be returned. Secure Collectors require the protocol be TCP. framing selects
// the TCP message framing: "auto" (default), "lf", or "octet" (RFC 6587).
func NewCollector(proto, iface, format, framing string, tlsConfig *tls.Config) (Collector, error) {
	// Verify that a parser can be instantiated. The actual parser that is used will
	// be created by the connection handler.
	_, err := NewParser(format)
	if err != nil {
		return nil, err
	}

	if strings.ToLower(proto) == "tcp" {
		return &TCPCollector{
			iface:     iface,
			format:    format,
			framing:   framing,
			tlsConfig: tlsConfig,
		}, nil
	} else if strings.ToLower(proto) == "udp" {
		addr, err := net.ResolveUDPAddr("udp", iface)
		if err != nil {
			return nil, err
		}

		return &UDPCollector{addr: addr, format: format}, nil
	}
	return nil, fmt.Errorf("unsupport collector protocol")
}

// Start instructs the TCPCollector to bind to the interface and accept connections.
func (s *TCPCollector) Start(c chan<- *Event) error {
	var ln net.Listener
	var err error
	if s.tlsConfig == nil {
		ln, err = net.Listen("tcp", s.iface)
	} else {
		ln, err = tls.Listen("tcp", s.iface, s.tlsConfig)
	}
	if err != nil {
		return err
	}
	s.addr = ln.Addr()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				continue
			}
			go s.handleConnection(conn, c)
		}
	}()
	return nil
}

// Addr returns the net.Addr that the Collector is bound to, in a race-say manner.
func (s *TCPCollector) Addr() net.Addr {
	return s.addr
}

func (s *TCPCollector) handleConnection(conn net.Conn, c chan<- *Event) {
	stats.Add("tcpConnections", 1)
	defer func() {
		stats.Add("tcpConnections", -1)
		conn.Close()
	}()
	// Last-resort guard: a panic inside this goroutine must not crash the whole
	// process (Issue #51). Recovery keeps the collector serving other
	// connections; the failure is counted and logged.
	defer func() {
		if r := recover(); r != nil {
			log.Printf("panic in TCP connection handler recovered: %v", r)
			stats.Add("tcpPanicsRecovered", 1)
		}
	}()

	parser, err := NewParser(s.format)
	if err != nil {
		// Parser configuration is validated at collector start; a failure here is
		// unexpected but must not crash the process (Issue #51). Drop this
		// connection and let the collector keep serving others.
		log.Printf("failed to create TCP connection parser: %s", err.Error())
		stats.Add("tcpParserErrors", 1)
		return
	}

	delimiter := newFrameDelimiter(s.framing, msgBufSize)
	reader := bufio.NewReader(conn)
	var log string
	var match bool

	for {
		if err := conn.SetReadDeadline(time.Now().Add(newlineTimeout)); err != nil {
			stats.Add("tcpConnUnrecoverError", 1)
			return
		}
		b, err := reader.ReadByte()
		if err != nil {
			stats.Add("tcpConnReadError", 1)
			if neterr, ok := err.(net.Error); ok && neterr.Timeout() {
				stats.Add("tcpConnReadTimeout", 1)
			} else if err == io.EOF {
				stats.Add("tcpConnReadEOF", 1)
			} else {
				stats.Add("tcpConnUnrecoverError", 1)
				return
			}

			log, match = delimiter.flushRemainder()
		} else {
			stats.Add("tcpBytesRead", 1)
			log, match = delimiter.Push(b)
		}

		// Log line available?
		if match {
			stats.Add("tcpEventsRx", 1)
			var parsed map[string]any
			if parser.Parse(bytes.NewBufferString(log).Bytes()) {
				parsed = parser.Result
			}
			c <- newEvent(string(parser.Raw), parsed, conn.RemoteAddr().String())
		}

		// Was the connection closed?
		if err == io.EOF {
			return
		}
	}
}

// Start instructs the UDPCollector to start reading packets from the interface.
func (s *UDPCollector) Start(c chan<- *Event) error {
	conn, err := net.ListenUDP("udp", s.addr)
	if err != nil {
		return err
	}

	parser, err := NewParser(s.format)
	if err != nil {
		return fmt.Errorf("failed to create UDP parser: %w", err)
	}

	go func() {
		// Last-resort guard: a panic in the UDP read loop must not crash the
		// process (Issue #51). Recovery keeps the collector reading.
		defer func() {
			if r := recover(); r != nil {
				log.Printf("panic in UDP collector recovered: %v", r)
				stats.Add("udpPanicsRecovered", 1)
			}
		}()
		buf := make([]byte, udpReadBufSize)
		for {
			n, addr, err := conn.ReadFromUDP(buf)
			stats.Add("udpBytesRead", int64(n))
			if err != nil {
				continue
			}
			if n > msgBufSize {
				// Datagram exceeded the accepted size: the remainder was dropped.
				// Count it so silent loss is observable (Issue #18).
				stats.Add("udpTruncated", 1)
				continue
			}
			log := strings.Trim(string(buf[:n]), "\r\n")
			var parsed map[string]any
			if parser.Parse(bytes.NewBufferString(log).Bytes()) {
				parsed = parser.Result
			}
			c <- newEvent(log, parsed, addr.String())
			stats.Add("udpEventsRx", 1)
		}
	}()
	return nil
}

// Addr returns the net.Addr to which the UDP collector is bound.
func (s *UDPCollector) Addr() net.Addr {
	return s.addr
}
