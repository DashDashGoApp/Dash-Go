package maps

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// One shared transport keeps TLS sessions and idle connections warm across
// tile fetches so a render does not pay a fresh handshake per tile. Per-call
// deadlines are supplied through the request context, never the client.
var mapHTTPClient = &http.Client{
	Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 3 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		MaxIdleConns:          8,
		MaxIdleConnsPerHost:   4,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 4 * time.Second,
		ForceAttemptHTTP2:     true,
	},
}

// mapRenderDeadline bounds one complete render attempt (all tiles of all
// layers) so a dead provider can no longer stack per-tile timeouts.
const mapRenderDeadline = 6 * time.Second

// mapTileConcurrency caps parallel tile requests. Three keeps the Pi's
// network stack and the upstream tile servers comfortable while removing the
// strictly sequential fetch latency.
const mapTileConcurrency = 3

func fetchMapURLContext(ctx context.Context, rawURL string, timeout time.Duration, maxBytes int64) ([]byte, string, error) {
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(callCtx, "GET", rawURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "Dash-Go/1.5.2 local-kiosk map preview (+local cache)")
	resp, err := mapHTTPClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	ctype := strings.ToLower(resp.Header.Get("Content-Type"))
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, "", err
	}
	if int64(len(b)) > maxBytes {
		return nil, "", fmt.Errorf("response too large")
	}
	if !strings.Contains(ctype, "image") && !strings.Contains(ctype, "octet-stream") {
		return nil, "", fmt.Errorf("not an image")
	}
	if len(b) < 50 {
		return nil, "", fmt.Errorf("image response too small")
	}
	if ctype == "" {
		ctype = "image/png"
	}
	return b, ctype, nil
}

func fetchMapURL(rawURL string, timeout time.Duration, maxBytes int64) ([]byte, string, error) {
	return fetchMapURLContext(context.Background(), rawURL, timeout, maxBytes)
}

func (s *Service) fetchTile(ctx context.Context, p mapProviderGo, rawURL string, z, x, y int, layer string) ([]byte, string, error) {
	if b, mime, ok := s.cachedTile(p.Name, z, x, y, layer); ok {
		return b, mime, nil
	}
	b, ctype, err := fetchMapURLContext(ctx, rawURL, 2*time.Second, 2*1024*1024)
	if err != nil {
		return nil, "", err
	}
	ext, mime := imageExtFromMime(ctype)
	base := tileCacheBase(p.Name, z, x, y, layer)
	if base != "" {
		_ = os.MkdirAll(s.mapTileDir(), 0755)
		final := filepath.Join(s.mapTileDir(), base+ext)
		tmp := final + ".tmp"
		if os.WriteFile(tmp, b, 0644) == nil {
			_ = os.Rename(tmp, final)
		} else {
			_ = os.Remove(tmp)
		}
	}
	return b, mime, nil
}

// tileRequest is one tile position with its provider template fallbacks.
type tileRequest struct {
	ux, uy    int
	templates []string
	z         int
	layer     string
}

type tileResult struct {
	b    []byte
	mime string
	err  error
}

// fetchTilesParallel runs tile requests under the shared concurrency cap.
// When abortOnFirstFailure is set, workers observe the first error and skip
// their network fetch. Results stay index-aligned with requests so draw order
// is preserved.
func (s *Service) fetchTilesParallel(ctx context.Context, p mapProviderGo, reqs []tileRequest, abortOnFirstFailure bool) []tileResult {
	out := make([]tileResult, len(reqs))
	var aborted atomic.Bool
	sem := make(chan struct{}, mapTileConcurrency)
	var wg sync.WaitGroup
	for i, req := range reqs {
		wg.Add(1)
		go func(slot int, r tileRequest) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if abortOnFirstFailure && aborted.Load() {
				out[slot] = tileResult{err: fmt.Errorf("skipped after earlier tile failure")}
				return
			}
			select {
			case <-ctx.Done():
				out[slot] = tileResult{err: ctx.Err()}
				return
			default:
			}
			var b []byte
			var mime string
			var err error
			for offset := 0; offset < len(r.templates); offset++ {
				tpl := r.templates[(r.ux+r.uy+offset)%len(r.templates)]
				b, mime, err = s.fetchTile(ctx, p, tileURLTemplate(tpl, r.z, r.ux, r.uy), r.z, r.ux, r.uy, r.layer)
				if err == nil {
					break
				}
			}
			out[slot] = tileResult{b: b, mime: mime, err: err}
			if abortOnFirstFailure && err != nil {
				aborted.Store(true)
			}
		}(i, req)
	}
	wg.Wait()
	return out
}
