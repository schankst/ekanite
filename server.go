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

// Server serves query client connections.
type Server struct {
	iface    string
	Searcher Searcher

	addr net.Addr
	ln   net.Listener

	mu    sync.Mutex
	conns map[net.Conn]struct{}

	Logger *log.Logger
}

// NewServer returns a new Server instance.
func NewServer(iface string, searcher Searcher) *Server {
	return &Server{
		iface:    iface,
		Searcher: searcher,
		conns:    make(map[net.Conn]struct{}),
		Logger:   log.New(os.Stderr, "[server] ", log.LstdFlags),
	}
}

// Start instructs the Server to bind to the interface and accept connections.
func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.iface)
	if err != nil {
		return err
	}

	s.ln = ln
	s.addr = ln.Addr()

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
	if s.ln != nil {
		if err := s.ln.Close(); err != nil {
			return err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for conn := range s.conns {
		conn.Close()
	}
	return nil
}

// Addr returns the address to which the Server is bound.
func (s *Server) Addr() net.Addr {
	return s.addr
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
			for s := range c {
				if _, werr := conn.Write([]byte(s + "\n")); werr != nil {
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
