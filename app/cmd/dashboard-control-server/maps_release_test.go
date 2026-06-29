package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func TestGoMapPrewarmUsesCachedEventLocations(t *testing.T) {
	a := testApp(t)
	if err := os.MkdirAll(a.cacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := fileio.WriteJSON(a.mapCacheFile(), map[string]any{
		"main street park": map[string]any{"label": "Main Street Park", "lat": 40.12345, "lon": -73.98765},
	}); err != nil {
		t.Fatal(err)
	}
	if err := fileio.WriteJSON(filepath.Join(a.cacheDir, "events.cache.json"), map[string]any{
		"events": []any{map[string]any{"title": "Practice", "location": "Main Street Park"}},
	}); err != nil {
		t.Fatal(err)
	}
	res := a.prewarmEventMaps(5)
	if res["ok"] != true {
		t.Fatalf("prewarm not ok: %#v", res)
	}
	if jsonutil.Int(res["resolved"], 0) != 1 {
		t.Fatalf("resolved=%v res=%#v", res["resolved"], res)
	}
	if jsonutil.Int(res["imagesWritten"], 0) != 6 {
		t.Fatalf("imagesWritten=%v res=%#v", res["imagesWritten"], res)
	}
	files, err := filepath.Glob(filepath.Join(a.mapImageDir(), "*.svg"))
	if err != nil || len(files) != 6 {
		t.Fatalf("expected 6 generated svg files, got %d err=%v", len(files), err)
	}
}

func TestGoMapRendererReusesCachedTiles(t *testing.T) {
	t.Setenv("DASHBOARD_MAP_NETWORK_TEST", "1")
	a := testApp(t)
	lat, lon, zoom := 41.8781, -87.6298, 15
	z, _, _, n, x0, x1, y0, y1 := tileBounds(lat, lon, zoom, 520, 220)
	_ = n
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x62, 0xf8, 0xff, 0xff, 0x3f, 0x03, 0x00, 0x08, 0xfc, 0x02, 0xfe, 0xa7, 0x69, 0x81, 0x9d, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82}
	if err := os.MkdirAll(a.mapTileDir(), 0755); err != nil {
		t.Fatal(err)
	}
	for ty := y0; ty <= y1; ty++ {
		if ty < 0 || ty >= n {
			continue
		}
		for tx := x0; tx <= x1; tx++ {
			ux := tx % n
			if ux < 0 {
				ux += n
			}
			name := tileCacheBase("osm-carto", z, ux, ty, "") + ".png"
			if err := os.WriteFile(filepath.Join(a.mapTileDir(), name), png, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	path, mime := a.fetchMapImage(lat, lon, zoom, "standard", true)
	if path == "" || mime != "image/svg+xml" {
		t.Fatalf("expected rendered svg, got path=%q mime=%q", path, mime)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, "data:image/png;base64,") {
		t.Fatalf("rendered map did not embed cached png tiles: %s", text[:min(120, len(text))])
	}
	if strings.Contains(text, "Map unavailable") || strings.Contains(text, "Go map preview") {
		t.Fatalf("rendered map unexpectedly used placeholder: %s", text[:min(160, len(text))])
	}
}

func TestMapStatusSnapshotInvalidatesAfterMaintenance(t *testing.T) {
	a := testApp(t)
	if err := os.MkdirAll(a.mapImageDir(), 0755); err != nil {
		t.Fatal(err)
	}
	first := a.mapCacheStatus()
	if got := jsonutil.Int(first["imageCount"], -1); got != 0 {
		t.Fatalf("initial image count=%d", got)
	}
	if err := os.WriteFile(filepath.Join(a.mapImageDir(), "standard_z15_p41.8781_m87.6298.svg"), []byte("<svg/>"), 0644); err != nil {
		t.Fatal(err)
	}
	withinTTL := a.mapCacheStatus()
	if got := jsonutil.Int(withinTTL["imageCount"], -1); got != 0 {
		t.Fatalf("map snapshot unexpectedly walked cache within TTL: %d", got)
	}
	a.invalidateMapStatusCache()
	fresh := a.mapCacheStatus()
	if got := jsonutil.Int(fresh["imageCount"], -1); got != 1 {
		t.Fatalf("invalidated map snapshot count=%d want 1", got)
	}
}
