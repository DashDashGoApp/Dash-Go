package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func radarTestApp(t *testing.T) *app {
	t.Helper()
	dash := t.TempDir()
	home := t.TempDir()
	a := &app{dash: dash, home: home, configDir: filepath.Join(dash, "config"), cacheDir: filepath.Join(dash, "cache"), settingsFile: filepath.Join(dash, "config", "settings.json"), configLocal: filepath.Join(dash, "config", "config.local.js")}
	a.ensureDirs()
	if err := os.MkdirAll(filepath.Join(dash, "ui", "js"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dash, "ui", "js", "config-defaults.js"), []byte(`const CONFIG={lat:41.8781,lon:-87.6298,radarProvider:"rainviewer"};`), 0644); err != nil {
		t.Fatal(err)
	}
	return a
}

func TestRadarStatusNeverIncludesKeys(t *testing.T) {
	a := radarTestApp(t)
	if err := os.WriteFile(filepath.Join(a.home, ".dashboard-radar.env"), []byte("DASH_RADAR_TOMORROW_KEY=secret-value\n"), 0600); err != nil {
		t.Fatal(err)
	}
	got := a.radarStatus()
	text := ""
	for _, item := range got["providers"].([]map[string]any) {
		text += item["id"].(string) + item["tier"].(string)
	}
	if strings.Contains(text, "secret-value") {
		t.Fatal("status exposed a key")
	}
	if got["provider"] != "rainviewer" {
		t.Fatalf("provider=%v", got["provider"])
	}
}

func TestRadarProxyOnlyAllowsKnownKeyedProvider(t *testing.T) {
	a := radarTestApp(t)
	for _, target := range []string{
		"/api/radar/tile?provider=custom_xyz&z=1&x=0&y=0",
		"/api/radar/tile?provider=https://example.invalid&z=1&x=0&y=0",
	} {
		w := httptest.NewRecorder()
		a.handleRadarTile(w, httptest.NewRequest(http.MethodGet, target, nil))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s status=%d", target, w.Code)
		}
	}
}

func TestRadarTileCoordinatesAndKeyedURL(t *testing.T) {
	a := radarTestApp(t)
	if err := os.WriteFile(filepath.Join(a.home, ".dashboard-radar.env"), []byte("DASH_RADAR_WEATHERBIT_KEY=secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodGet, "/api/radar/tile?provider=weatherbit&z=3&x=4&y=2", nil)
	z, x, y, err := radarTileCoordinates(r)
	if err != nil || z != 3 || x != 4 || y != 2 {
		t.Fatalf("coords %d %d %d %v", z, x, y, err)
	}
	u, err := a.radarTileURL("weatherbit", z, x, y)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(u, "maps.weatherbit.io") || !strings.Contains(u, "key=secret") {
		t.Fatalf("unexpected URL %q", u)
	}
}

func TestRadarProviderFrameModesAreSourceOwned(t *testing.T) {
	if got := radarFrameMode("rainviewer"); got != "source" {
		t.Fatalf("RainViewer frame mode=%q, want source", got)
	}
	for _, provider := range []string{"nws", "tomorrow", "weatherbit", "xweather", "custom_xyz"} {
		if got := radarFrameMode(provider); got != "latest" {
			t.Fatalf("%s frame mode=%q, want latest", provider, got)
		}
	}
}

func TestRadarStatusDoesNotExposeRetiredFrameBudget(t *testing.T) {
	a := radarTestApp(t)
	status := a.radarStatus()
	if _, present := status["frameBudget"]; present {
		t.Fatal("radar status must not expose a retired profile frame budget")
	}
	for _, item := range status["providers"].([]map[string]any) {
		if _, present := item["frameBudget"]; present {
			t.Fatal("radar provider status must use source frame mode, not profile budget")
		}
		if item["id"] == "rainviewer" {
			if item["frameMode"] != "source" {
				t.Fatalf("RainViewer frameMode=%v", item["frameMode"])
			}
			if item["animated"] != true {
				t.Fatalf("RainViewer animated=%v", item["animated"])
			}
		} else if item["animated"] != false {
			t.Fatalf("current-frame provider %v must not advertise animation", item["id"])
		}
	}
}
