package weather

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

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func testService(t *testing.T, values map[string]any) *Service {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	for _, dir := range []string{home, filepath.Join(root, "config"), filepath.Join(root, "cache"), filepath.Join(root, "calendars"), filepath.Join(root, "ui", "js")} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "ui", "js", "config-defaults.js"), []byte(`const CONFIG={lat:41.8781,lon:-87.6298,weatherProviders:["openmeteo"]};`), 0644); err != nil {
		t.Fatal(err)
	}
	return New(ServiceConfig{
		Dash:        root,
		Home:        home,
		CacheDir:    filepath.Join(root, "cache"),
		CalendarDir: filepath.Join(root, "calendars"),
		ConfigLocal: filepath.Join(root, "config", "config.local.js"),
		LoadSettings: func() map[string]any {
			return values
		},
		ProfilePayload:         func() map[string]any { return map[string]any{"base": "balanced"} },
		ProfileBaseForSettings: func(map[string]any) string { return "balanced" },
		NetworkLikelyAvailable: func() bool { return true },
	})
}

func TestServiceFetchesAndCachesOpenMeteo(t *testing.T) {
	today := time.Now().Format("2006-01-02")
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.URL.Path != "/v1/forecast" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		if r.URL.Query().Get("forecast_days") != "16" {
			t.Fatalf("forecast_days=%q", r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"current": map[string]any{"temperature_2m": 72.0, "apparent_temperature": 72.0, "weather_code": 1, "wind_speed_10m": 6.0, "relative_humidity_2m": 55},
			"daily":   map[string]any{"time": []string{today}, "weather_code": []int{1}, "temperature_2m_max": []float64{78}, "temperature_2m_min": []float64{65}},
			"hourly":  map[string]any{"time": []string{today + "T00:00"}, "temperature_2m": []float64{68}, "weather_code": []int{1}, "precipitation_probability": []int{0}},
		})
	}))
	defer server.Close()
	service := testService(t, map[string]any{"lat": 41.8781, "lon": -87.6298, "wxApi": server.URL, "weatherProviders": []any{"openmeteo"}})
	payload, err := service.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if payload["generator"] != "go" || hits != 1 {
		t.Fatalf("payload=%#v hits=%d", payload["generator"], hits)
	}
	if sources := jsonutil.List(payload["sources"]); len(sources) != 1 || jsonutil.StringValue(jsonutil.Map(sources[0])["_source"]) != "openmeteo" {
		t.Fatalf("sources=%#v", sources)
	}
	if _, err := service.Fetch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if hits != 1 {
		t.Fatalf("fresh cache did not suppress a duplicate request: hits=%d", hits)
	}
}

func TestServiceRadarStatusNeverLeaksKey(t *testing.T) {
	service := testService(t, map[string]any{})
	if err := os.WriteFile(filepath.Join(service.home, ".dashboard-radar.env"), []byte("DASH_RADAR_TOMORROW_KEY=secret-value\n"), 0600); err != nil {
		t.Fatal(err)
	}
	status := service.RadarStatus()
	if strings.Contains(strings.ToLower(formatAny(status)), "secret-value") {
		t.Fatal("radar status leaked a secret")
	}
	if status["provider"] != "rainviewer" {
		t.Fatalf("provider=%#v", status["provider"])
	}
}

func TestServiceGeneratesMoonCalendarWithoutCoreDependency(t *testing.T) {
	service := testService(t, map[string]any{})
	if err := os.WriteFile(service.configLocal, []byte(`window.DASH_CONFIG={lat:41.8781,lon:-87.6298,locationName:"Chicago"};\n`), 0644); err != nil {
		t.Fatal(err)
	}
	result := service.GenerateMoonCalendar(true)
	if result["ok"] != true {
		t.Fatalf("result=%#v", result)
	}
	if _, err := os.Stat(filepath.Join(service.calDir, "moon.violet.ics")); err != nil {
		t.Fatal(err)
	}
}

func formatAny(value any) string {
	body, _ := json.Marshal(value)
	return string(body)
}

func TestTolerantWeatherBlendKeepsNumericCodesWithoutNilCategory(t *testing.T) {
	sources := []any{
		map[string]any{"current": map[string]any{"weather_code": nil}},
		map[string]any{"current": map[string]any{"weather_code": 2}},
	}
	if got := modalWeatherCodeGo(sources, -1, "current"); got != 2 {
		t.Fatalf("blended weather code = %#v, want numeric code 2", got)
	}
}
