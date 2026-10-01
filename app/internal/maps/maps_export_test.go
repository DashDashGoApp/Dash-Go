package maps

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// The two ArcGIS exports must be in flight at the same time: a server that
// only releases the second response after the first completes must not make
// the fetch take the sum of both latencies.
func TestFetchArcGISExportsRunsConcurrently(t *testing.T) {
	var imgHits, labelHits atomic.Int64
	imgSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		imgHits.Add(1)
		// Hold the imagery response until the labels request arrives,
		// proving the two are in flight together.
		deadline := time.After(2 * time.Second)
		for labelHits.Load() == 0 {
			select {
			case <-deadline:
				t.Error("labels request never arrived while imagery was in flight")
			default:
				time.Sleep(time.Millisecond)
			}
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(make([]byte, 64))
	}))
	defer imgSrv.Close()
	labelSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		labelHits.Add(1)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(make([]byte, 64))
	}))
	defer labelSrv.Close()

	started := time.Now()
	img, labels := fetchArcGISExports(context.Background(), imgSrv.URL, labelSrv.URL)
	elapsed := time.Since(started)
	if img.err != nil || labels.err != nil {
		t.Fatalf("img err=%v labels err=%v", img.err, labels.err)
	}
	if len(img.b) != 64 || len(labels.b) != 64 {
		t.Fatalf("unexpected bodies: img=%d labels=%d", len(img.b), len(labels.b))
	}
	if elapsed > 1500*time.Millisecond {
		t.Fatalf("exports took %v; expected concurrent, not sequential", elapsed)
	}
}

// A label export failure is tolerated and reported; imagery failure is the
// render failure.
func TestFetchArcGISExportsToleratesLabelFailure(t *testing.T) {
	imgSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(make([]byte, 64))
	}))
	defer imgSrv.Close()
	labelSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer labelSrv.Close()
	img, labels := fetchArcGISExports(context.Background(), imgSrv.URL, labelSrv.URL)
	if img.err != nil {
		t.Fatalf("imagery failed: %v", img.err)
	}
	if labels.err == nil {
		t.Fatal("expected label failure to be reported")
	}
}

// Imagery failure fails the fetch pair even when labels succeed.
func TestFetchArcGISExportsImageryFailureWins(t *testing.T) {
	imgSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer imgSrv.Close()
	labelSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(make([]byte, 64))
	}))
	defer labelSrv.Close()
	img, _ := fetchArcGISExports(context.Background(), imgSrv.URL, labelSrv.URL)
	if img.err == nil {
		t.Fatal("expected imagery failure to be reported")
	}
}
