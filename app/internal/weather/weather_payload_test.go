package weather

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func writeWeatherCacheDoc(t *testing.T, path string, doc map[string]any) {
	t.Helper()
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0644); err != nil {
		t.Fatal(err)
	}
}

func clearForceRefreshForTest(t *testing.T) {
	t.Helper()
	prev := os.Getenv("DASH_WEATHER_FORCE_REFRESH")
	os.Unsetenv("DASH_WEATHER_FORCE_REFRESH")
	t.Cleanup(func() { os.Setenv("DASH_WEATHER_FORCE_REFRESH", prev) })
}

func weatherCacheDocFor(t *testing.T, service *Service, current any) map[string]any {
	t.Helper()
	cfg := service.Config()
	return map[string]any{
		"current":  map[string]any{"temperature_2m": current, "weather_code": 1},
		"daily":    map[string]any{"time": []string{time.Now().Format("2006-01-02")}},
		"hourly":   map[string]any{"time": []string{time.Now().Format("2006-01-02") + "T00:00"}},
		"selected": cfg.Providers,
		"sources":  []any{},
		"status":   []any{map[string]any{"id": "openmeteo", "ok": true, "freshness": "fresh"}},
		"cache":    map[string]any{"cacheKey": weatherCacheKeyGo(cfg), "generatedAt": float64(1234)},
	}
}

func currentTemperature(t *testing.T, payload any) float64 {
	t.Helper()
	v, _ := anyMap(anyMap(payload)["current"])["temperature_2m"].(float64)
	return v
}

// TestWeatherPayloadBytesIdenticalAcrossWarmCalls asserts the O5 gate: with an
// unchanged cache file, repeated /api/weather payloads are byte-identical,
// served from the mtime/size-keyed parse cache without re-reading or
// re-parsing the file.
func TestWeatherPayloadBytesIdenticalAcrossWarmCalls(t *testing.T) {
	clearForceRefreshForTest(t)
	service := testService(t, map[string]any{})
	cachePath := filepath.Join(service.cacheDir, "weather-cache.json")
	writeWeatherCacheDoc(t, cachePath, weatherCacheDocFor(t, service, float64(71.5)))

	first, err := json.Marshal(service.Payload())
	if err != nil {
		t.Fatal(err)
	}
	second, err := json.Marshal(service.Payload())
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("payload bytes differ across warm calls:\n%s\n%s", first, second)
	}
	var got map[string]any
	if err := json.Unmarshal(first, &got); err != nil {
		t.Fatal(err)
	}
	if !jsonutil.Truthy(anyMap(got["cache"])["hit"]) {
		t.Fatalf("warm payload must report a cache hit: %#v", got["cache"])
	}
}

// TestWeatherParseCacheInvalidatesWhenCacheFileChanges asserts the O5
// invalidation contract: when the cache file is rewritten on disk (new
// mtime/size), the next payload reflects the new content.
func TestWeatherParseCacheInvalidatesWhenCacheFileChanges(t *testing.T) {
	clearForceRefreshForTest(t)
	service := testService(t, map[string]any{})
	cachePath := filepath.Join(service.cacheDir, "weather-cache.json")
	writeWeatherCacheDoc(t, cachePath, weatherCacheDocFor(t, service, float64(70)))
	if got := currentTemperature(t, service.Payload()); got != 70 {
		t.Fatalf("first payload temperature=%v", got)
	}
	// 72.25 serializes with a different byte length than 70, so the rewrite
	// changes both mtime and size.
	writeWeatherCacheDoc(t, cachePath, weatherCacheDocFor(t, service, float64(72.25)))
	if got := currentTemperature(t, service.Payload()); got != 72.25 {
		t.Fatalf("stale parse cache served old payload, temperature=%v", got)
	}
}
