package weather

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func TestProviderUnitCorrections(t *testing.T) {
	if got := maxProbabilityGo(20, 65); got != float64(65) && got != 65 {
		t.Fatalf("max probability=%#v", got)
	}
	if visualCrossingWindUnitGo("metric") != "kmh" || visualCrossingWindUnitGo("us") != "mph" {
		t.Fatal("Visual Crossing source wind units are not mapped correctly")
	}
}

func TestPirateHourlyLiquidTotalsUseLocationDayAndIntensity(t *testing.T) {
	payload := map[string]any{
		"offset": float64(-5),
		"hourly": map[string]any{"data": []any{
			map[string]any{"time": float64(1783760400), "precipIntensity": 0.1},
			map[string]any{"time": float64(1783764000), "precipIntensity": 0.2},
		}},
	}
	totals := pirateHourlyLiquidTotalsGo(payload, "us")
	if len(totals) != 1 {
		t.Fatalf("totals=%#v", totals)
	}
	for _, total := range totals {
		if math.Abs(total-7.62) > 0.001 { // 0.3 inches liquid equivalent
			t.Fatalf("total=%v want 7.62mm", total)
		}
	}
}

func TestPirateLocalDateUsesTimezoneThenHourOffset(t *testing.T) {
	instant := time.Date(2026, 7, 11, 2, 0, 0, 0, time.UTC).Unix()
	if got := pirateLocalDateGo(map[string]any{"timezone": "America/Chicago", "offset": -5}, instant); got != "2026-07-10" {
		t.Fatalf("timezone local date=%q", got)
	}
	if got := pirateLocalDateGo(map[string]any{"offset": -5}, instant); got != "2026-07-10" {
		t.Fatalf("offset local date=%q", got)
	}
}

func TestWeatherHTTPMetricsAndRetryAfter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"message":"slow down"}`))
	}))
	defer server.Close()
	ctx, metrics := withWeatherRequestMetrics(context.Background())
	_, err := fetchJSONGo(ctx, server.URL)
	httpErr, ok := err.(*weatherHTTPError)
	if !ok || httpErr.StatusCode != 429 || httpErr.RetryAfter != 2*time.Minute {
		t.Fatalf("error=%#v", err)
	}
	calls, bytes, status := metrics.Snapshot()
	if calls != 1 || bytes == 0 || status != 429 {
		t.Fatalf("metrics calls=%d bytes=%d status=%d", calls, bytes, status)
	}
}

func TestProviderFreshTTLsRemainIndependent(t *testing.T) {
	service := testService(t, map[string]any{})
	if got := service.weatherProviderFreshTTLGo("openmeteo"); got != 30*time.Minute {
		t.Fatalf("Open-Meteo TTL=%v", got)
	}
	if got := service.weatherProviderFreshTTLGo("weatherbit"); got != 90*time.Minute {
		t.Fatalf("Weatherbit TTL=%v", got)
	}
	if got := service.weatherRefreshMinutes(); got != 30 {
		t.Fatalf("aggregate refresh=%d, want profile cadence 30", got)
	}
}

func TestManualRefreshBypassesFreshProviderCache(t *testing.T) {
	today := time.Now().Format("2006-01-02")
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = w.Write([]byte(`{"current":{"temperature_2m":72,"apparent_temperature":72,"weather_code":0,"wind_speed_10m":5,"relative_humidity_2m":50},"daily":{"time":["` + today + `"],"weather_code":[0],"temperature_2m_max":[80],"temperature_2m_min":[60]},"hourly":{"time":[],"temperature_2m":[],"weather_code":[],"precipitation_probability":[]}}`))
	}))
	defer server.Close()
	service := testService(t, map[string]any{"lat": 41.8, "lon": -87.6, "wxApi": server.URL, "weatherProviders": []any{"openmeteo"}})
	if _, err := service.Fetch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Fetch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if hits != 1 {
		t.Fatalf("fresh provider cache was not reused: hits=%d", hits)
	}
	payload, err := service.RefreshLive(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if hits != 2 {
		t.Fatalf("manual refresh did not force a live provider request: hits=%d", hits)
	}
	status := jsonutil.Map(jsonutil.List(payload["status"])[0])
	if !jsonutil.Truthy(status["liveAttempted"]) || jsonutil.Int(status["networkCalls"], 0) != 1 {
		t.Fatalf("manual refresh telemetry=%#v", status)
	}
}

func TestWeatherHealthCarriesProviderCacheMetadata(t *testing.T) {
	status := weatherHealthOKGo("openmeteo", Config{}, map[string]any{
		"_providerCacheHit":        true,
		"_providerCacheAgeSeconds": 95,
		"_stale":                   true,
		"_staleReason":             "live provider request failed",
	})
	if !jsonutil.Truthy(status["cacheHit"]) || jsonutil.Int(status["providerCacheAgeSeconds"], -1) != 95 {
		t.Fatalf("cache metadata=%#v", status)
	}
	if !jsonutil.Truthy(status["stale"]) || jsonutil.StringValue(status["freshness"]) != "stale" {
		t.Fatalf("stale metadata=%#v", status)
	}
	if jsonutil.StringValue(status["staleReason"]) != "live provider request failed" {
		t.Fatalf("stale reason=%#v", status)
	}
}
