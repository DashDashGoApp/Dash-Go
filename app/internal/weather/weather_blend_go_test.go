package weather

import (
	"testing"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func TestBlendWeatherSourcesGo(t *testing.T) {
	cfg := Config{Days: 2}
	sources := []any{
		map[string]any{"current": map[string]any{"temperature_2m": 70.0, "weather_code": 1}, "daily": map[string]any{"time": []any{"2026-01-01", "2026-01-02"}, "temperature_2m_max": []any{80.0, 81.0}, "temperature_2m_min": []any{60.0, 61.0}, "weather_code": []any{1, 2}, "precipitation_probability_max": []any{20.0, 10.0}}, "hourly": map[string]any{"time": []any{"2026-01-01T00:00"}, "temperature_2m": []any{65.0}, "weather_code": []any{1}, "precipitation_probability": []any{0.0}}},
		map[string]any{"current": map[string]any{"temperature_2m": 72.0, "weather_code": 1}, "daily": map[string]any{"time": []any{"2026-01-01", "2026-01-02"}, "temperature_2m_max": []any{82.0, 83.0}, "temperature_2m_min": []any{62.0, 63.0}, "weather_code": []any{1, 3}, "precipitation_probability_max": []any{40.0, 20.0}}, "hourly": map[string]any{"time": []any{"2026-01-01T00:00"}, "temperature_2m": []any{67.0}, "weather_code": []any{1}, "precipitation_probability": []any{10.0}}},
	}
	got := blendWeatherSourcesGo(sources, nil, []string{"a", "b"}, cfg)
	cur := jsonutil.Map(got["current"])
	if cur["temperature_2m"] != 71.0 {
		t.Fatalf("blended current temp = %#v", cur["temperature_2m"])
	}
	daily := got["daily"].(map[string][]any)
	if len(daily["time"]) != 2 {
		t.Fatalf("daily dates not blended: %#v", daily["time"])
	}
	if daily["temperature_2m_max"][0] != 81.0 {
		t.Fatalf("daily high not averaged: %#v", daily["temperature_2m_max"])
	}
	hourly := got["hourly"].(map[string][]any)
	if hourly["temperature_2m"][0] != 66.0 {
		t.Fatalf("hourly temp not averaged: %#v", hourly["temperature_2m"])
	}
	if got["_source"] != "blend" {
		t.Fatalf("expected blended source marker")
	}
}
