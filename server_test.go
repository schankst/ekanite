package ekanite

import (
	"bufio"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

// fakeSearcher is a Searcher test double returning fixed results (or an error).
type fakeSearcher struct {
	results []string
	err     error
}

func (f *fakeSearcher) Search(string) (<-chan string, error) {
	if f.err != nil {
		return nil, f.err
	}
	c := make(chan string, len(f.results))
	for _, r := range f.results {
		c <- r
	}
	close(c)
	return c, nil
}

// TestServer_TelnetQuery exercises the line-based query server end to end: a
// query is sent, results are returned line by line, terminated by a blank line
// (Issue #87: the telnet handleConnection path had no coverage).
func TestServer_TelnetQuery(t *testing.T) {
	s := NewServer("127.0.0.1:0", &fakeSearcher{results: []string{"line-one", "line-two"}})
	if err := s.Start(); err != nil {
		t.Fatalf("failed to start server: %s", err.Error())
	}
	defer s.Close()

	conn, err := net.Dial("tcp", s.Addr().String())
	if err != nil {
		t.Fatalf("dial: %s", err.Error())
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))

	if _, err := conn.Write([]byte("login\n")); err != nil {
		t.Fatalf("write query: %s", err.Error())
	}

	var lines []string
	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatalf("read: %s", err.Error())
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break // end-of-results marker (blank line)
		}
		lines = append(lines, line)
	}

	if len(lines) != 2 || lines[0] != "line-one" || lines[1] != "line-two" {
		t.Fatalf("unexpected results: %v", lines)
	}
}
