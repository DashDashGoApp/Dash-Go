package weather

import (
	"testing"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// The hourly contract is a provider-local naive "2006-01-02T15:04" wall clock
// plus canonical Celsius. These tests assert the normalization directly, because
// a mismatch here is invisible: the browser unions hours by exact string
// identity and silently falls back to a single source when the strings differ.
func TestWeatherHourlyNormalizesProviderLocalWallClock(t *testing.T) {
	// 19:00Z on 2026-07-11 is 14:00 in Chicago.
	epoch := float64(time.Date(2026, 7, 11, 19, 0, 0, 0, time.UTC).Unix())
	cases := []struct {
		name    string
		payload map[string]any
		want    string
	}{
		{"iana zone", map[string]any{"timezone": "America/Chicago"}, "2026-07-11T14:00"},
		{"seconds offset", map[string]any{"timezone_offset": float64(-18000)}, "2026-07-11T14:00"},
		{"hours offset", map[string]any{"offset": float64(-5)}, "2026-07-11T14:00"},
	}
	for _, tc := range cases {
		if got := weatherHourlyLocalTime(tc.payload, epoch); got != tc.want {
			t.Errorf("%s: local hour=%q want %q", tc.name, got, tc.want)
		}
	}
	if got := weatherHourlyFromRFC3339("2026-07-11T14:00:00-05:00"); got != "2026-07-11T14:00" {
		t.Errorf("offset-bearing ISO hour=%q", got)
	}
	if got := weatherHourlyEpochFromValue("2026-07-11T19:00:00Z"); got != epoch {
		t.Errorf("ISO instant epoch=%v want %v", got, epoch)
	}
}

func TestWeatherHourlyMapsProvidersToTheCanonicalShape(t *testing.T) {
	// OpenWeather: One Call 3.0 hourly is already inside the response Dash-Go
	// fetches today, so this costs no extra request.
	openWeather := map[string]any{"timezone_offset": float64(-18000), "hourly": []any{
		map[string]any{"dt": float64(1783800000), "temp": 24.5, "pop": 0.25, "weather": []any{map[string]any{"id": float64(500)}}},
	}}
	block := jsonutil.Map(weatherHourlyForProviderGo("openweather", openWeather, Config{TempUnit: "celsius"}))
	if got := len(jsonutil.List(block["time"])); got != 1 {
		t.Fatalf("openweather hourly rows=%d", got)
	}
	if got := jsonutil.TextValue(jsonutil.List(block["temperature_2m"])[0]); got != "24.5" {
		t.Errorf("openweather hourly temperature=%q want canonical celsius", got)
	}
	if got := jsonutil.Int(jsonutil.List(block["weather_code"])[0], -1); got != 61 {
		t.Errorf("openweather hourly code=%d want rain", got)
	}
	if got := jsonutil.Int(jsonutil.List(block["precipitation_probability"])[0], -1); got != 25 {
		t.Errorf("openweather pop=%d want percent", got)
	}

	// NWS periods carry the location offset, so the hour keeps its own clock.
	nws := jsonutil.Map(nwsHourlyGo([]any{map[string]any{
		"startTime":                  "2026-07-11T14:00:00-05:00",
		"temperature":                float64(68),
		"temperatureUnit":            "F",
		"shortForecast":              "Rain Showers",
		"probabilityOfPrecipitation": map[string]any{"value": float64(80)},
	}}))
	if got := jsonutil.TextValue(jsonutil.List(nws["time"])[0]); got != "2026-07-11T14:00" {
		t.Errorf("nws hourly time=%q", got)
	}
	if got := jsonutil.Int(jsonutil.List(nws["temperature_2m"])[0], 0); got != 20 {
		t.Errorf("nws hourly temperature=%d want 20 celsius", got)
	}
	if got := jsonutil.Int(jsonutil.List(nws["precipitation_probability"])[0], -1); got != 80 {
		t.Errorf("nws hourly pop=%d", got)
	}

	// Visual Crossing publishes the hour as a time-only clock ("00:00:00") under
	// the day that carries the date. The fixture here previously invented a full
	// ISO stamp, so it validated the wrong shape: the adapter then contributed no
	// hourly rows while still reporting a healthy source, and nothing failed.
	visual := jsonutil.Map(visualCrossingHourlyGo(map[string]any{"days": []any{
		map[string]any{"datetime": "2026-07-11", "hours": []any{
			map[string]any{"datetime": "14:00:00", "temp": 25.0, "conditions": "Partially cloudy", "precipprob": float64(5)},
		}},
		map[string]any{"datetime": "2026-07-12", "hours": []any{
			map[string]any{"datetime": "00:00:00", "temp": 18.5, "conditions": "Clear", "precipprob": float64(0)},
		}},
	}}))
	visualTimes := jsonutil.List(visual["time"])
	if len(visualTimes) != 2 {
		t.Fatalf("visual crossing hourly rows=%d want 2", len(visualTimes))
	}
	if got := jsonutil.TextValue(visualTimes[0]); got != "2026-07-11T14:00" {
		t.Errorf("visual crossing hourly time=%q want the day joined to the hour clock", got)
	}
	if got := jsonutil.TextValue(visualTimes[1]); got != "2026-07-12T00:00" {
		t.Errorf("visual crossing next-day time=%q", got)
	}
	if got := jsonutil.Int(jsonutil.List(visual["temperature_2m"])[0], 0); got != 25 {
		t.Errorf("visual crossing hourly temperature=%d want canonical celsius", got)
	}
	// A full stamp is still accepted, so a future API change cannot silently
	// empty this block a second time.
	if got := visualCrossingHourlyStampGo("2026-07-11", "2026-07-11T14:00:00"); got != "2026-07-11T14:00" {
		t.Errorf("full-stamp passthrough=%q", got)
	}
	if got := visualCrossingHourlyStampGo("", "14:00:00"); got != "" {
		t.Errorf("a missing day date must not invent a stamp, got %q", got)
	}
	// Root cause, kept as documentation: the shared normalizer needs a full
	// stamp, so a bare time-only clock normalized to nothing and the whole block
	// disappeared without an error being raised anywhere.
	if got := weatherHourlyFromLocalISO("00:00:00"); got != "" {
		t.Errorf("time-only clocks must not normalize without their day, got %q", got)
	}
}

func TestWeatherHourlyIsBoundedToTheDashboardHorizon(t *testing.T) {
	rows := []any{}
	for index := 0; index < weatherHourlyMaxRows+40; index++ {
		rows = append(rows, map[string]any{
			"startTime":                  "2026-07-11T14:00:00-05:00",
			"temperature":                float64(68),
			"temperatureUnit":            "F",
			"shortForecast":              "Sunny",
			"probabilityOfPrecipitation": map[string]any{"value": float64(0)},
		})
	}
	block := jsonutil.Map(nwsHourlyGo(rows))
	if got := len(jsonutil.List(block["time"])); got != weatherHourlyMaxRows {
		t.Fatalf("nws hourly rows=%d want %d", got, weatherHourlyMaxRows)
	}
	// Open-Meteo's own payload is trimmed to the same horizon.
	openMeteo := map[string]any{"hourly": map[string]any{"time": rows}}
	trimOpenMeteoHourlyGo(openMeteo)
	if got := len(jsonutil.List(anyMap(openMeteo["hourly"])["time"])); got != weatherHourlyMaxRows {
		t.Fatalf("open-meteo hourly rows=%d want %d", got, weatherHourlyMaxRows)
	}
}

// A household that enables only providers which omit sunrise and sunset must
// still see them, because Dash-Go derives both from its configured location.
func TestWeatherSunTimesAreDerivedForProviderGaps(t *testing.T) {
	location := time.FixedZone("CDT", -5*3600)
	rise, set := weatherSunTimesForDay(41.8781, -87.6298, time.Date(2026, 10, 1, 0, 0, 0, 0, location), location)
	if rise.IsZero() || set.IsZero() {
		t.Fatalf("Chicago sunrise/sunset was not derived: rise=%v set=%v", rise, set)
	}
	// Cross-checked against the public sunrise/sunset service for this date:
	// 06:45 sunrise and 18:34 sunset local.
	for _, tc := range []struct {
		name string
		got  time.Time
		want time.Time
	}{
		{"sunrise", rise.In(location), time.Date(2026, 10, 1, 6, 45, 0, 0, location)},
		{"sunset", set.In(location), time.Date(2026, 10, 1, 18, 34, 0, 0, location)},
	} {
		if diff := tc.got.Sub(tc.want); diff > 4*time.Minute || diff < -4*time.Minute {
			t.Errorf("%s=%s want %s (±4m)", tc.name, tc.got.Format("15:04"), tc.want.Format("15:04"))
		}
	}

	// A source that publishes neither value gains both, and a source that
	// already knows them keeps its own numbers. Both daily shapes must work: a
	// constructed adapter (map[string][]any) and a decoded provider payload
	// (map[string]any).
	sources := []any{
		map[string]any{"_source": "nws", "daily": map[string][]any{"time": {"2026-10-01"}, "sunrise": {""}, "sunset": {""}}},
		map[string]any{"_source": "openmeteo", "daily": map[string]any{"time": []any{"2026-10-01"}, "sunrise": []any{"2026-10-01T06:40"}, "sunset": []any{"2026-10-01T18:30"}}},
	}
	fillDerivedSunTimesGo(sources, Config{Lat: 41.8781, Lon: -87.6298})
	constructed := anyMap(sources[0])["daily"].(map[string][]any)
	if jsonutil.TextValue(constructed["sunrise"][0]) == "" || jsonutil.TextValue(constructed["sunset"][0]) == "" {
		t.Fatalf("constructed-provider daily gap was not filled: %#v", constructed)
	}
	decoded := anyMap(anyMap(sources[1])["daily"])
	if got := jsonutil.TextValue(jsonutil.List(decoded["sunrise"])[0]); got != "2026-10-01T06:40" {
		t.Errorf("provider-supplied sunrise was overwritten: %q", got)
	}
}

// Polar night must leave the values empty rather than inventing a sunrise.
func TestWeatherSunTimesStayEmptyDuringPolarNight(t *testing.T) {
	location := time.UTC
	rise, set := weatherSunTimesForDay(78.2, 15.6, time.Date(2026, 12, 21, 0, 0, 0, 0, location), location)
	if !rise.IsZero() || !set.IsZero() {
		t.Fatalf("polar night produced rise=%v set=%v", rise, set)
	}
}
