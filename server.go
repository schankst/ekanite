package ekanite

import (
	"bufio"
	"errors"
	"log"
	"net"
	"os"
	"strings"
	"sync"
)

// Searcher is the interface any object that perform searches should implement.
type Searcher interface {
	Search(query string) (<-chan string, error)
}

// serverBase holds the state and lifecycle shared by Ekanite's TCP query
// servers (telnet and HTTP): the bind interface, the bound listener/address,
// the searcher and the logger. Server and HTTPServer embed it and add only
// their connection handling (Issue #10).
type serverBase struct {
	iface    string
	Searcher Searcher

	addr net.Addr
	ln   net.Listener

	Logger *log.Logger
}

// listen binds the TCP listener and records the bound address.
func (b *serverBase) listen() error {
	ln, err := net.Listen("tcp", b.iface)
	if err != nil {
		return err
	}
	b.ln = ln
	b.addr = ln.Addr()
	return nil
}

// closeListener closes the listener, if one was bound.
func (b *serverBase) closeListener() error {
	if b.ln != nil {
		return b.ln.Close()
	}
	return nil
}

// Addr returns the address to which the server is bound.
func (b *serverBase) Addr() net.Addr {
	return b.addr
}

// Server serves query client connections over a line-based (telnet) protocol.
type Server struct {
	serverBase

	mu    sync.Mutex
	conns map[net.Conn]struct{}
}

// NewServer returns a new Server instance.
func NewServer(iface string, searcher Searcher) *Server {
	return &Server{
		serverBase: serverBase{
			iface:    iface,
			Searcher: searcher,
			Logger:   log.New(os.Stderr, "[server] ", log.LstdFlags),
		},
		conns: make(map[net.Conn]struct{}),
	}
}

// Start instructs the Server to bind to the interface and accept connections.
func (s *Server) Start() error {
	if err := s.listen(); err != nil {
		return err
	}

	go func() {
		for {
			conn, err := s.ln.Accept()
			if err != nil {
				// Listener closed (shutdown, Issue #49) or transient failure.
				if errors.Is(err, net.ErrClosed) {
					return
				}
				continue
			}
			s.mu.Lock()
			s.conns[conn] = struct{}{}
			s.mu.Unlock()
			go s.handleConnection(conn)
		}
	}()
	return nil
}

// Close stops accepting new connections and closes all active ones, so the
// server drains promptly on shutdown (Issue #49).
func (s *Server) Close() error {
	if err := s.closeListener(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for conn := range s.conns {
		conn.Close()
	}
	return nil
}

func (s *Server) handleConnection(conn net.Conn) {
	defer func() {
		s.mu.Lock()
		delete(s.conns, conn)
		s.mu.Unlock()
		conn.Close()
		s.Logger.Printf("connection from %s closed", conn.RemoteAddr())
	}()
	s.Logger.Printf("new connection from %s", conn.RemoteAddr())

	reader := bufio.NewReader(conn)
	for {
		b, err := reader.ReadString('\n')
		if err != nil {
			conn.Close()
			return
		}

		query := strings.Trim(b, "\r\n")
		if query == "" {
			continue
		}

		s.Logger.Printf("executing query '%s'", query)
		c, err := s.Searcher.Search(query)
		if err != nil {
			if _, werr := conn.Write([]byte(err.Error())); werr != nil {
				conn.Close()
				return
			}
		} else {
			for res := range c {
				if _, werr := conn.Write([]byte(res + "\n")); werr != nil {
					conn.Close()
					// Drain any remaining results so the search goroutine can
					// finish and release the engine lock (Issue #13); otherwise
					// an abandoned search would block retention enforcement.
					for range c {
					}
					return
				}
			}
		}
		// Send two newlines to indicate end-of-results.
		if _, werr := conn.Write([]byte("\n\n")); werr != nil {
			conn.Close()
			return
		}
	}
}
