package weather

import (
	"testing"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func TestWeatherSourcesPayloadGoLeavesBlendingToBrowser(t *testing.T) {
	sources := []any{
		map[string]any{"_source": "first", "_sourceLabel": "First source", "current": map[string]any{"temperature_2m": 70.0}, "daily": map[string]any{"time": []any{"2026-01-01"}}, "hourly": map[string]any{"time": []any{"2026-01-01T00:00"}}},
		map[string]any{"_source": "second", "_sourceLabel": "Second source", "current": map[string]any{"temperature_2m": 72.0}},
	}
	got := weatherSourcesPayloadGo(sources, nil, []string{"first", "second"})
	if got["current"].(map[string]any)["temperature_2m"] != 70.0 {
		t.Fatalf("top-level compatibility current must mirror first source, got %#v", got["current"])
	}
	if got["_source"] != "first" {
		t.Fatalf("source marker = %#v, want first", got["_source"])
	}
	blend := jsonutil.Map(got["weatherBlend"])
	if blend["method"] != "browser-authoritative normalized sources" {
		t.Fatalf("unexpected blend contract: %#v", blend)
	}
	if len(got["sources"].([]any)) != 2 {
		t.Fatalf("raw sources were not retained: %#v", got["sources"])
	}
}

func TestPrecipitationMMGoCanonicalizesProviderUnits(t *testing.T) {
	cases := []struct {
		value any
		unit  string
		want  float64
	}{
		{1, "in", 25.4}, {2, "cm", 20}, {3.5, "mm", 3.5},
	}
	for _, tc := range cases {
		got, ok := precipitationMMGo(tc.value, tc.unit).(float64)
		if !ok || got != tc.want {
			t.Fatalf("precipitationMMGo(%v, %q) = %#v, want %v", tc.value, tc.unit, precipitationMMGo(tc.value, tc.unit), tc.want)
		}
	}
	if got := precipitationSumMMGo(1.5, 2.25); got != 3.75 {
		t.Fatalf("precipitationSumMMGo = %#v, want 3.75", got)
	}
	if got := precipitationMMGo(-1, "mm"); got != nil {
		t.Fatalf("negative precipitation must be rejected, got %#v", got)
	}
}
