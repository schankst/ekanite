package ekanite

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func newTestHTTPServer(t *testing.T, searcher Searcher) *HTTPServer {
	t.Helper()
	s := NewHTTPServer("127.0.0.1:0", searcher)
	if err := s.Start(); err != nil {
		t.Fatalf("failed to start HTTP server: %s", err.Error())
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// TestHTTPServer_ServeHTTP_GET verifies the index page for GET (Issue #87: the
// HTTP handler had no coverage).
func TestHTTPServer_ServeHTTP_GET(t *testing.T) {
	s := newTestHTTPServer(t, &fakeSearcher{})

	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Ekanite query interface") {
		t.Fatal("index page not rendered")
	}
}

// TestHTTPServer_ServeHTTP_POSTQuery verifies a POST query renders results.
func TestHTTPServer_ServeHTTP_POSTQuery(t *testing.T) {
	s := newTestHTTPServer(t, &fakeSearcher{results: []string{"hit-alpha", "hit-beta"}})

	form := url.Values{"query": {"login"}}
	req := httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "hit-alpha") || !strings.Contains(body, "hit-beta") {
		t.Fatalf("results not rendered: %q", body)
	}
}

// TestHTTPServer_ServeHTTP_SearchError verifies a searcher error yields HTTP 500.
func TestHTTPServer_ServeHTTP_SearchError(t *testing.T) {
	s := newTestHTTPServer(t, &fakeSearcher{err: errors.New("boom")})

	form := url.Values{"query": {"login"}}
	req := httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

// TestHTTPServer_ServeHTTP_MethodNotAllowed verifies unsupported methods are rejected.
func TestHTTPServer_ServeHTTP_MethodNotAllowed(t *testing.T) {
	s := newTestHTTPServer(t, &fakeSearcher{})

	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest("DELETE", "/", nil))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}
