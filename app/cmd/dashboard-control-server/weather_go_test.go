package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func TestGoWeatherOpenMeteoRefreshAndCache(t *testing.T) {
	today := time.Now().Format("2006-01-02")
	hits := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.URL.Path != "/v1/forecast" {
			t.Fatalf("unexpected weather path %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("latitude") != "41.8781" || q.Get("longitude") != "-87.6298" {
			t.Fatalf("wrong coordinates in query: %s", r.URL.RawQuery)
		}
		if q.Get("forecast_days") != "16" {
			t.Fatalf("wrong source-owned forecast_days: %s", q.Get("forecast_days"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"current": map[string]any{"temperature_2m": 72.0, "apparent_temperature": 75.0, "weather_code": 1, "wind_speed_10m": 6.0, "relative_humidity_2m": 55},
			"daily":   map[string]any{"time": []string{today}, "weather_code": []int{1}, "temperature_2m_max": []float64{78}, "temperature_2m_min": []float64{65}},
			"hourly":  map[string]any{"time": []string{today + "T00:00"}, "temperature_2m": []float64{68}, "weather_code": []int{1}, "precipitation_probability": []int{0}},
		})
	}))
	defer ts.Close()

	dir := t.TempDir()
	a := &app{dash: dir, configDir: filepath.Join(dir, "config"), cacheDir: filepath.Join(dir, "cache"), logDir: filepath.Join(dir, "logs"), configLocal: filepath.Join(dir, "config", "config.local.js"), settingsFile: filepath.Join(dir, "config", "settings.json")}
	a.ensureDirs()
	if err := os.WriteFile(a.settingsFile, []byte(`{"lat":41.8781,"lon":-87.6298,"wxApi":"`+ts.URL+`","weatherProviders":["openmeteo"]}`), 0644); err != nil {
		t.Fatal(err)
	}

	payload, err := a.fetchGoWeather(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if payload["generator"] != "go" {
		t.Fatalf("missing go generator: %#v", payload["generator"])
	}
	sources := jsonutil.List(payload["sources"])
	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}
	if src := sources[0].(map[string]any); src["_source"] != "openmeteo" {
		t.Fatalf("wrong source metadata: %#v", src["_source"])
	}
	if hits != 1 {
		t.Fatalf("expected 1 fetch hit, got %d", hits)
	}
}

func TestGoWeatherLeavesBrowserFallbackAvailableWhenNoSourceAnswered(t *testing.T) {
	dir := t.TempDir()
	a := &app{dash: dir, configDir: filepath.Join(dir, "config"), cacheDir: filepath.Join(dir, "cache"), logDir: filepath.Join(dir, "logs"), settingsFile: filepath.Join(dir, "config", "settings.json")}
	a.ensureDirs()
	if err := os.WriteFile(a.settingsFile, []byte(`{"weatherProviders":["weatherapi"]}`), 0644); err != nil {
		t.Fatal(err)
	}
	payload := a.weatherPayload().(map[string]any)
	if sources := jsonutil.List(payload["sources"]); len(sources) != 0 {
		t.Fatalf("expected no source payloads, got %#v", sources)
	}
	if _, ok := payload["sourceHealth"]; !ok {
		t.Fatalf("expected stable-compatible sourceHealth field")
	}
	status := jsonutil.List(payload["status"])
	if len(status) == 0 || !strings.Contains(strings.ToLower(strOr(status[0].(map[string]any)["error"], "")), "no go weather provider") {
		t.Fatalf("unexpected status: %#v", status)
	}
}

func TestGoWeatherProviderCacheServesFreshAndStale(t *testing.T) {
	today := time.Now().Format("2006-01-02")
	calls := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"current": map[string]any{"temperature_2m": 70.0, "apparent_temperature": 71.0, "weather_code": 1, "wind_speed_10m": 5.0, "relative_humidity_2m": 50},
				"daily":   map[string]any{"time": []string{today}, "weather_code": []int{1}, "temperature_2m_max": []float64{80}, "temperature_2m_min": []float64{60}},
			})
			return
		}
		http.Error(w, `{"error":"quota"}`, http.StatusTooManyRequests)
	}))
	defer ts.Close()

	dir := t.TempDir()
	a := &app{dash: dir, configDir: filepath.Join(dir, "config"), cacheDir: filepath.Join(dir, "cache"), logDir: filepath.Join(dir, "logs"), settingsFile: filepath.Join(dir, "config", "settings.json")}
	a.ensureDirs()
	if err := os.WriteFile(a.settingsFile, []byte(`{"lat":41.8781,"lon":-87.6298,"wxApi":"`+ts.URL+`","weatherProviders":["openmeteo"]}`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := a.fetchGoWeather(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("first fetch calls = %d", calls)
	}
	if _, err := a.fetchGoWeather(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("fresh provider cache should avoid live call; calls = %d", calls)
	}
	path := a.weatherProviderCachePathGo("openmeteo")
	oldTime := time.Now().Add(-31 * time.Minute)
	if err := os.Chtimes(path, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	payload, err := a.fetchGoWeather(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("expected stale refresh live attempt; calls = %d", calls)
	}
	status := jsonutil.List(payload["status"])
	if len(status) == 0 {
		t.Fatalf("missing status")
	}
	if got := jsonutil.Map(status[0])["status"]; got != "stale_cache" {
		t.Fatalf("expected stale_cache status, got %#v (%#v)", got, status[0])
	}
	if _, _, ok := a.weatherProviderCooldownGo("openmeteo"); !ok {
		t.Fatalf("expected provider cooldown after 429")
	}
}

func TestGoWeatherCachesExpireAtLocalDayRollover(t *testing.T) {
	dir := t.TempDir()
	a := &app{dash: dir, configDir: filepath.Join(dir, "config"), cacheDir: filepath.Join(dir, "cache"), logDir: filepath.Join(dir, "logs"), settingsFile: filepath.Join(dir, "config", "settings.json")}
	a.ensureDirs()
	if err := os.WriteFile(a.settingsFile, []byte(`{"lat":41.8781,"lon":-87.6298,"weatherProviders":["openmeteo"]}`), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := a.weatherConfig()
	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	oldSource := map[string]any{"daily": map[string]any{"time": []any{yesterday}}}
	currentSource := map[string]any{"daily": map[string]any{"time": []any{today}}}

	cachePath := filepath.Join(a.cacheDir, "weather-cache.json")
	oldPayload := map[string]any{"daily": map[string]any{"time": []any{yesterday}}}
	weatherMarkCacheGo(oldPayload, cfg, false, "")
	if err := fileio.WriteJSON(cachePath, oldPayload); err != nil {
		t.Fatal(err)
	}
	if _, ok := a.readWeatherCache(cachePath, weatherCacheKeyGo(cfg)); ok {
		t.Fatal("yesterday's aggregate cache was accepted as fresh after local midnight")
	}
	currentPayload := map[string]any{"daily": map[string]any{"time": []any{today}}}
	weatherMarkCacheGo(currentPayload, cfg, false, "")
	if err := fileio.WriteJSON(cachePath, currentPayload); err != nil {
		t.Fatal(err)
	}
	if _, ok := a.readWeatherCache(cachePath, weatherCacheKeyGo(cfg)); !ok {
		t.Fatal("current-day aggregate cache was rejected")
	}

	providerKey := weatherProviderCacheKeyGo("openmeteo", cfg)
	if err := a.writeWeatherProviderCacheGo("openmeteo", providerKey, oldSource); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := a.readWeatherProviderCacheGo("openmeteo", providerKey, false); ok {
		t.Fatal("yesterday's provider cache was accepted as fresh after local midnight")
	}
	if _, _, ok := a.readWeatherProviderCacheGo("openmeteo", providerKey, true); !ok {
		t.Fatal("yesterday's provider cache should remain available only as stale fallback")
	}
	if err := a.writeWeatherProviderCacheGo("openmeteo", providerKey, currentSource); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := a.readWeatherProviderCacheGo("openmeteo", providerKey, false); !ok {
		t.Fatal("current-day provider cache was rejected")
	}
}

func TestFetchJSONGoRejectsOversizedResponse(t *testing.T) {
	large := strings.Repeat("x", weatherJSONResponseLimit)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"blob":"` + large + `"}`))
	}))
	defer ts.Close()
	if _, err := fetchJSONGo(context.Background(), ts.URL); err == nil {
		t.Fatal("oversized weather response unexpectedly decoded")
	}
}

func TestWeatherRefreshGuardrailsUseConfiguredProviderBudget(t *testing.T) {
	if got, _ := weatherRefreshMinimumForProviders([]string{"openmeteo"}); got != weatherRefreshHardMinimumMinutes {
		t.Fatalf("openmeteo minimum=%d", got)
	}
	if got, _ := weatherRefreshMinimumForProviders([]string{"weatherapi"}); got != weatherRefreshAPIKeyMinimumMinutes {
		t.Fatalf("weatherapi minimum=%d", got)
	}
	if got, _ := weatherRefreshMinimumForProviders([]string{"weatherbit"}); got != weatherRefreshLowQuotaMinimumMinutes {
		t.Fatalf("weatherbit minimum=%d", got)
	}
	a := testProfileApp(t)
	// Legacy cadence is retained in settings for upgrade compatibility but cannot
	// change the automatic profile/provider policy.
	if err := fileio.WriteJSON(a.settingsFile, map[string]any{"weatherProviders": []any{"weatherbit"}, "refreshWxMinutes": 5, "profile": "balanced"}); err != nil {
		t.Fatal(err)
	}
	if got := a.weatherCacheTTL(); got != time.Duration(weatherRefreshLowQuotaMinimumMinutes)*time.Minute {
		t.Fatalf("weatherbit effective cache ttl=%s", got)
	}
	if got := a.weatherRefreshMinutes(); got != weatherRefreshLowQuotaMinimumMinutes {
		t.Fatalf("weatherbit automatic cadence=%d", got)
	}
	if _, err := a.updateProfileValues(map[string]any{"refreshWxMinutes": 90}); err == nil {
		t.Fatal("retired weather cadence must not return to Profile editing")
	}
	if err := fileio.WriteJSON(a.settingsFile, map[string]any{"weatherProviders": []any{"openmeteo"}, "profile": "lite"}); err != nil {
		t.Fatal(err)
	}
	if got := a.weatherRefreshMinutes(); got != 45 {
		t.Fatalf("lite automatic weather cadence=%d, want 45", got)
	}
	if err := fileio.WriteJSON(a.settingsFile, map[string]any{"weatherProviders": []any{"openmeteo"}, "profile": "balanced"}); err != nil {
		t.Fatal(err)
	}
	if got := a.weatherRefreshMinutes(); got != 30 {
		t.Fatalf("balanced automatic weather cadence=%d, want 30", got)
	}
}
