package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGoWeatherKeyedProviderMissingKeyStatus(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := weatherConfig{Lat: 41.8781, Lon: -87.6298, TempUnit: "fahrenheit", WindUnit: "mph", Days: 3}
	item := weatherHealthErrorGo("weatherapi", cfg, "Missing API key; source ignored until a key is saved", true)
	if item["status"] != "missing_key" || item["keyRequired"] != true || item["hasKey"] != false {
		t.Fatalf("unexpected missing-key health: %#v", item)
	}
}

func TestGoWeatherEnvKeyLookup(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.WriteFile(filepath.Join(home, ".dashboard-weather.env"), []byte("DASH_WEATHERAPI_KEY='abc123'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := weatherProviderKeyGo("weatherapi", weatherConfig{}); got != "abc123" {
		t.Fatalf("expected env key, got %q", got)
	}
}

func TestNWSCoverageGate(t *testing.T) {
	if !insideNWSCoverageGo(41.8781, -87.6298) {
		t.Fatalf("expected Chicago coordinates to be inside NWS coverage")
	}
	if insideNWSCoverageGo(51.5, -0.1) {
		t.Fatalf("expected London coordinates outside NWS coverage")
	}
}

func TestGoWeatherProviderKeyLookupPrefersConfigAndAliases(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := weatherConfig{ProviderKeys: map[string]string{"weatherapi": "from-config", "googleweather": "google-config"}, APIKey: "custom-key"}
	if got := weatherProviderKeyGo("weather-api", cfg); got != "from-config" {
		t.Fatalf("expected config weatherapi key, got %q", got)
	}
	if got := weatherProviderKeyGo("google-weather", cfg); got != "google-config" {
		t.Fatalf("expected google alias key, got %q", got)
	}
	if got := weatherProviderKeyGo("openmeteo-custom", cfg); got != "custom-key" {
		t.Fatalf("expected openmeteo custom apiKey, got %q", got)
	}
}

func TestGoWeatherProviderDayCaps(t *testing.T) {
	cases := map[string]int{"weatherapi": 3, "openweather": 8, "tomorrow": 5, "pirateweather": 8, "nws": 7, "openmeteo": 16}
	for id, want := range cases {
		cfg := weatherProviderFetchConfigGo(id, weatherConfig{Days: 30})
		if cfg.Days != want {
			t.Fatalf("%s day cap got %d want %d", id, cfg.Days, want)
		}
	}
}
