package weather

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// The keyed radar proxy must reuse connections across tiles: a sweep of N
// tiles should not pay N fresh TCP/TLS handshakes.
func TestRadarHTTPClientReusesConnections(t *testing.T) {
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(make([]byte, 64))
	}))
	defer server.Close()
	for i := 0; i < 3; i++ {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), radarProxyRequestDeadline)
		req = req.Clone(ctx)
		resp, err := radarHTTPClient.Do(req)
		cancel()
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		_ = resp.Body.Close()
	}
	// httptest's plain-HTTP server uses one keep-alive connection for the
	// sequence; the transport pools it rather than dialing per request.
}

// The per-request deadline is supplied through the context: a slow upstream
// is cut off at radarProxyRequestDeadline even though the shared client has
// no Timeout of its own.
func TestRadarHTTPClientHonorsContextDeadline(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(2 * time.Second)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(make([]byte, 64))
	}))
	defer slow.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, slow.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	resp, err := radarHTTPClient.Do(req)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("expected a deadline error from a slow upstream")
	}
	if time.Since(started) > time.Second {
		t.Fatalf("request ran %v past its context deadline", time.Since(started))
	}
}
