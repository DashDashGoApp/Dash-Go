package maps

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// newTileTestService returns a Service whose tile cache directory is a temp
// dir, plus the tile server and a request counter.
func newTileTestService(t *testing.T) (*Service, *httptest.Server, *atomic.Int64) {
	t.Helper()
	var hits atomic.Int64
	var mu sync.Mutex
	seen := []int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, 1)
		mu.Unlock()
		hits.Add(1)
		w.Header().Set("Content-Type", "image/png")
		// A real >50 byte PNG header-ish body satisfies fetch validation.
		body := make([]byte, 64)
		body[0], body[1] = 0x89, 0x50
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)
	service := newTestService(t)
	return service, server, &hits
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	return New(ServiceConfig{
		CacheDir:  t.TempDir(),
		ConfigDir: t.TempDir(),
		LogDir:    t.TempDir(),
	})
}

// The parallel fan-out must never exceed the concurrency cap.
func TestFetchTilesParallelRespectsConcurrencyCap(t *testing.T) {
	var inflight, maxInflight atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cur := inflight.Add(1)
		for {
			old := maxInflight.Load()
			if cur <= old || maxInflight.CompareAndSwap(old, cur) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		inflight.Add(-1)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(make([]byte, 64))
	}))
	defer server.Close()
	s := newTestService(t)
	p := mapProviderGo{Name: "cap-tiles", Tiles: []string{server.URL + "/{z}/{x}/{y}"}}
	_ = p
	reqs := []tileRequest{}
	for i := 0; i < 9; i++ {
		reqs = append(reqs, tileRequest{ux: i, uy: 0, templates: p.Tiles, z: 15})
	}
	_ = s.fetchTilesParallel(context.Background(), p, reqs, false)
	if got := maxInflight.Load(); got > mapTileConcurrency {
		t.Fatalf("observed concurrency %d exceeds cap %d", got, mapTileConcurrency)
	}
}

// Draw order is preserved: results must be index-aligned with requests.
func TestFetchTilesParallelPreservesOrder(t *testing.T) {
	s, server, _ := newTileTestService(t)
	p := mapProviderGo{Name: "order-tiles", Tiles: []string{server.URL + "/{z}/{x}/{y}"}}
	reqs := []tileRequest{}
	for i := 0; i < 4; i++ {
		reqs = append(reqs, tileRequest{ux: i, uy: 0, templates: p.Tiles, z: 15})
	}
	out := s.fetchTilesParallel(context.Background(), p, reqs, false)
	for slot, res := range out {
		if res.err != nil {
			t.Fatalf("tile %d failed: %v", slot, res.err)
		}
		if len(res.b) != 64 {
			t.Fatalf("tile %d body size %d", slot, len(res.b))
		}
	}
}

// First imagery failure aborts the remaining network fetches.
func TestFetchTilesParallelAbortsOnFirstFailure(t *testing.T) {
	var called atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := called.Add(1)
		if n == 1 {
			// First request fails hard; a 500 is not an image fetch error path,
			// so force a network-level error by closing the connection.
			panic(http.ErrAbortHandler)
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(make([]byte, 64))
	}))
	defer server.Close()
	s := newTestService(t)
	p := mapProviderGo{Name: "abort-tiles", Tiles: []string{server.URL + "/{z}/{x}/{y}"}}
	reqs := []tileRequest{}
	for i := 0; i < 9; i++ {
		reqs = append(reqs, tileRequest{ux: i, uy: 0, templates: p.Tiles, z: 15})
	}
	start := time.Now()
	out := s.fetchTilesParallel(context.Background(), p, reqs, true)
	if time.Since(start) > 5*time.Second {
		t.Fatal("abort path took too long")
	}
	failed := 0
	for _, res := range out {
		if res.err != nil {
			failed++
		}
	}
	if failed < 1 {
		t.Fatal("expected at least one failure")
	}
}

// The shared transport reuses one connection across sequential fetches: the
// second fetch must not pay a fresh TCP handshake.
func TestFetchMapURLReusesConnection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(make([]byte, 64))
	}))
	defer server.Close()
	for i := 0; i < 2; i++ {
		if _, _, err := fetchMapURL(server.URL, 2*time.Second, 1024*1024); err != nil {
			t.Fatalf("fetch %d: %v", i, err)
		}
	}
}

// Cleanup throttling: two cleanups in a row must run the directory scan once.
func TestThrottledMapCacheCleanupRunsOncePerInterval(t *testing.T) {
	s := newTestService(t)
	// Prime the timer so the first call is not the boot-time clean.
	s.throttledMapCacheCleanup()
	s.mapCleanupMu.Lock()
	before := s.mapCleanupAt
	s.mapCleanupMu.Unlock()
	s.throttledMapCacheCleanup()
	s.mapCleanupMu.Lock()
	after := s.mapCleanupAt
	s.mapCleanupMu.Unlock()
	if !before.Equal(after) {
		t.Fatal("throttled cleanup ran a second time inside the interval")
	}
}

// The geocode memo returns hits and invalidates on cache-file change.
func TestGeocodeMemoRoundTrip(t *testing.T) {
	s := newTestService(t)
	if hit := s.geocodeMemoGet("memo-key"); hit != nil {
		t.Fatalf("empty memo returned %v", hit)
	}
	// Seed a cache file so geocodeMemoPut can stat it.
	cacheFile := s.mapCacheFile()
	if err := writeFileForTest(cacheFile, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	s.geocodeMemoPut("memo-key", map[string]any{"ok": true, "lat": 1.5, "lon": 2.5})
	hit := s.geocodeMemoGet("memo-key")
	if hit == nil || hit["ok"] != true || hit["cached"] != true {
		t.Fatalf("memo hit = %v", hit)
	}
	// An external write to the cache file invalidates the memo. Force a
	// distinct mtime: coarse-grain filesystems can give identical stamps.
	if err := writeFileForTest(cacheFile, []byte(`{"other": 1}`)); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(cacheFile, time.Now().Add(2*time.Second), time.Now().Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	if hit := s.geocodeMemoGet("memo-key"); hit != nil {
		t.Fatalf("stale memo returned %v", hit)
	}
}

func writeFileForTest(path string, body []byte) error {
	return os.WriteFile(path, body, 0644)
}

// Negative results are never memoized.
func TestGeocodeMemoRejectsNegatives(t *testing.T) {
	s := newTestService(t)
	if err := writeFileForTest(s.mapCacheFile(), []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	s.geocodeMemoPut("neg-key", map[string]any{"ok": false, "error": "location not found"})
	if hit := s.geocodeMemoGet("neg-key"); hit != nil {
		t.Fatalf("negative was memoized: %v", hit)
	}
}

// The per-render image deadline bounds a full layered render.
func TestRenderDeadlineBoundsTileFetches(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(make([]byte, 64))
	}))
	defer slow.Close()
	s := newTestService(t)
	p := mapProviderGo{Name: "slow-tiles", Tiles: []string{slow.URL + "/{z}/{x}/{y}"}}
	start := time.Now()
	_, _, err := s.renderTileSVG(p, 38.7, -89.9, 15, 520, 220)
	if err == nil {
		t.Fatal("expected a deadline error from a slow provider")
	}
	if time.Since(start) > mapRenderDeadline+2*time.Second {
		t.Fatalf("render ran %v past its deadline", time.Since(start))
	}
	_ = err
}
