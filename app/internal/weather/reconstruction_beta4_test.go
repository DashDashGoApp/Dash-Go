package weather

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func TestWeatherPartialSourcePreservesDailyWhenCurrentFails(t *testing.T) {
	source, err := weatherPartialSourceGo("googleweather", nil, map[string][]any{
		"time":                     {"2026-07-11"},
		"weather_code":             {float64(0)},
		"temperature_2m_max":       {float64(30)},
		"temperature_2m_min":       {float64(20)},
		"apparent_temperature_max": {float64(31)},
		"wind_speed_10m_max":       {float64(5)},
	}, nil, errors.New("current unavailable"), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !jsonutil.Truthy(source["_partial"]) {
		t.Fatalf("partial marker missing: %#v", source)
	}
	current := jsonutil.Map(source["current"])
	if got := jsonutil.Int(current["temperature_2m"], -1); got != 30 {
		t.Fatalf("daily fallback did not derive current temperature: %#v", current)
	}
	status := weatherHealthOKGo("googleweather", Config{}, source)
	if jsonutil.StringValue(status["status"]) != "partial" || !jsonutil.Truthy(status["partial"]) {
		t.Fatalf("health did not preserve usable partial status: %#v", status)
	}
}

func TestWeatherPartialSourceFailsWhenNoPartIsUsable(t *testing.T) {
	_, err := weatherPartialSourceGo("googleweather", nil, nil, nil, errors.New("current unavailable"), errors.New("daily unavailable"), errors.New("hourly unavailable"))
	if err == nil {
		t.Fatal("empty multi-call provider result was accepted")
	}
}

func TestWeatherLocationCacheIsBoundToProviderAndRoundedLocation(t *testing.T) {
	cfg := Config{CacheDir: t.TempDir(), Lat: 38.627003, Lon: -90.199402}
	weatherLocationCacheWrite(cfg, "nws", "https://example.test/grid")
	if got, ok := weatherLocationCacheRead(cfg, "nws"); !ok || got != "https://example.test/grid" {
		t.Fatalf("location cache miss: got=%q ok=%v", got, ok)
	}
	moved := cfg
	moved.Lat += 0.01
	if _, ok := weatherLocationCacheRead(moved, "nws"); ok {
		t.Fatal("location cache was reused for a different location")
	}
	if _, ok := weatherLocationCacheRead(cfg, "accuweather"); ok {
		t.Fatal("location cache was reused across providers")
	}
}

func TestProviderCacheKeyIgnoresDisplayUnits(t *testing.T) {
	base := Config{Lat: 38.627, Lon: -90.1994, Days: 7, TempUnit: "celsius", WindUnit: "ms"}
	other := base
	other.TempUnit = "fahrenheit"
	other.WindUnit = "mph"
	if left, right := weatherProviderCacheKeyGo("openmeteo", base), weatherProviderCacheKeyGo("openmeteo", other); left != right {
		t.Fatalf("display units changed canonical provider cache identity:\n%s\n%s", left, right)
	}
}

func TestWeatherDisplayConversionDoesNotMutateCanonicalCache(t *testing.T) {
	canonical := map[string]any{
		"current": map[string]any{"temperature_2m": float64(20), "apparent_temperature": float64(19), "wind_speed_10m": float64(5)},
		"daily":   map[string]any{"time": []any{"2026-07-11"}, "temperature_2m_max": []any{float64(25)}, "temperature_2m_min": []any{float64(15)}, "wind_speed_10m_max": []any{float64(7)}},
	}
	display := weatherDisplaySourceGo(canonical, Config{TempUnit: "fahrenheit", WindUnit: "mph"})
	if got := jsonutil.Int(jsonutil.Map(display["current"])["temperature_2m"], 0); got != 68 {
		t.Fatalf("temperature conversion=%d want 68", got)
	}
	if got := jsonutil.Int(jsonutil.Map(canonical["current"])["temperature_2m"], 0); got != 20 {
		t.Fatalf("canonical cache was mutated: %d", got)
	}
}

func TestWeatherLocationCacheRejectsExpiredEntry(t *testing.T) {
	cfg := Config{CacheDir: t.TempDir(), Lat: 38.627, Lon: -90.1994}
	weatherLocationCacheWrite(cfg, "nws", "https://example.test/grid")
	path := weatherLocationCachePath(cfg, "nws")
	old := weatherLocationCacheTTL + 1
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	when := info.ModTime().Add(-old)
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatal(err)
	}
	if _, ok := weatherLocationCacheRead(cfg, "nws"); ok {
		t.Fatalf("expired location cache remained valid: %s", filepath.Base(path))
	}
}
