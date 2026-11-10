package input

import (
	"bufio"
	"crypto/tls"
	"errors"
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
	// maxTCPConns caps the number of concurrent TCP connections so a flood of
	// half-open/idle connections cannot exhaust goroutines and file descriptors
	// (Issue #14). Connections beyond the cap are rejected immediately.
	maxTCPConns = 1000
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

// parsedEvent parses one framed log line and builds the event to forward. The
// parser's raw bytes are the input line, so the event text is the line itself;
// a failed parse still yields an event (raw/unparsed fallback, Issue #16).
// Shared by the TCP and UDP collectors so parse+construct lives in one place
// (Issue #5).
func parsedEvent(parser *Parser, line, source string) *Event {
	var parsed map[string]any
	if parser.Parse([]byte(line)) {
		parsed = parser.Result
	}
	return newEvent(line, parsed, source)
}

// Collector specifies the interface all network collectors must implement.
type Collector interface {
	Start(chan<- *Event) error
	Addr() net.Addr
	// Close stops the collector so its accept/read loop returns (Issue #49).
	Close() error
}

// TCPCollector represents a network collector that accepts and handler TCP connections.
type TCPCollector struct {
	iface   string
	format  string
	framing string

	addr      net.Addr
	ln        net.Listener
	tlsConfig *tls.Config

	// readTimeout is the per-byte read deadline; an idle connection is closed
	// once it elapses instead of looping forever (Issue #14). Defaults to
	// newlineTimeout when zero.
	readTimeout time.Duration
	// maxConns caps concurrent connections; excess connections are rejected
	// (Issue #14). Defaults to maxTCPConns when zero.
	maxConns int

	// sem bounds concurrent connections (size maxConns); created in Start.
	sem chan struct{}
}

// UDPCollector represents a network collector that accepts UDP packets.
type UDPCollector struct {
	format string
	addr   *net.UDPAddr
	conn   *net.UDPConn
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
			iface:       iface,
			format:      format,
			framing:     framing,
			tlsConfig:   tlsConfig,
			readTimeout: newlineTimeout,
			maxConns:    maxTCPConns,
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
	s.ln = ln
	s.addr = ln.Addr()

	limit := s.maxConns
	if limit <= 0 {
		limit = maxTCPConns
	}
	s.sem = make(chan struct{}, limit)

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				// Listener closed (shutdown, Issue #49) or transient failure.
				if errors.Is(err, net.ErrClosed) {
					return
				}
				continue
			}
			// Bound concurrent connections (Issue #14): reject beyond the cap
			// so a flood of half-open connections cannot exhaust resources.
			select {
			case s.sem <- struct{}{}:
				go func() {
					defer func() { <-s.sem }()
					s.handleConnection(conn, c)
				}()
			default:
				stats.Add("tcpConnsRejected", 1)
				conn.Close()
			}
		}
	}()
	return nil
}

// Close closes the TCP listener; the accept loop returns and active
// connections drain naturally (Issue #49).
func (s *TCPCollector) Close() error {
	if s.ln != nil {
		return s.ln.Close()
	}
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
	readTimeout := s.readTimeout
	if readTimeout <= 0 {
		readTimeout = newlineTimeout
	}

	for {
		if err := conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
			stats.Add("tcpConnUnrecoverError", 1)
			return
		}
		b, err := reader.ReadByte()
		if err != nil {
			stats.Add("tcpConnReadError", 1)
			if neterr, ok := err.(net.Error); ok && neterr.Timeout() {
				// Idle sender: the deadline ends the connection instead of
				// re-arming forever (Issue #14).
				stats.Add("tcpConnReadTimeout", 1)
			} else if errors.Is(err, io.EOF) {
				stats.Add("tcpConnReadEOF", 1)
			} else {
				stats.Add("tcpConnUnrecoverError", 1)
				return
			}

			// Flush any buffered complete line, then close the connection.
			if log, match := delimiter.flushRemainder(); match {
				stats.Add("tcpEventsRx", 1)
				c <- parsedEvent(parser, log, conn.RemoteAddr().String())
			}
			return
		}

		stats.Add("tcpBytesRead", 1)
		if log, match := delimiter.Push(b); match {
			stats.Add("tcpEventsRx", 1)
			c <- parsedEvent(parser, log, conn.RemoteAddr().String())
		}
	}
}

// Start instructs the UDPCollector to start reading packets from the interface.
func (s *UDPCollector) Start(c chan<- *Event) error {
	conn, err := net.ListenUDP("udp", s.addr)
	if err != nil {
		return err
	}
	s.conn = conn

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
				// Connection closed (shutdown, Issue #49) or transient failure.
				if errors.Is(err, net.ErrClosed) {
					return
				}
				continue
			}
			if n > msgBufSize {
				// Datagram exceeded the accepted size: the remainder was dropped.
				// Count it so silent loss is observable (Issue #18).
				stats.Add("udpTruncated", 1)
				continue
			}
			log := strings.Trim(string(buf[:n]), "\r\n")
			c <- parsedEvent(parser, log, addr.String())
			stats.Add("udpEventsRx", 1)
		}
	}()
	return nil
}

// Close closes the UDP socket; the read loop returns (Issue #49).
func (s *UDPCollector) Close() error {
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

// Addr returns the net.Addr to which the UDP collector is bound.
func (s *UDPCollector) Addr() net.Addr {
	return s.addr
}
