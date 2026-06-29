package main

import (
	"strings"
	"testing"
)

func TestDiagnosticsRedactsAllProviderCredentialFamilies(t *testing.T) {
	input := strings.Join([]string{
		"DASH_OPENWEATHER_KEY=openweather-secret",
		"DASH_TOMORROW_KEY=tomorrow-secret",
		"DASH_WEATHERBIT_KEY=weatherbit-secret",
		"DASH_VISUALCROSSING_KEY=visual-secret",
		"DASH_PIRATEWEATHER_KEY=pirate-secret",
		"DASH_ACCUWEATHER_KEY=accu-secret",
		"DASH_METEOSOURCE_KEY=meteo-secret",
		"DASH_WEATHERAPI_KEY=weatherapi-secret",
		"DASH_XWEATHER_KEY=xweather-secret",
		"DASH_GOOGLEWEATHER_KEY=google-secret",
		"DASH_OPENMETEO_CUSTOM_KEY=openmeteo-secret",
		"DASH_API_NINJAS_KEY=ninjas-secret",
		"DASH_RADAR_XWEATHER_ID=xweather-id-secret",
		"DASH_RADAR_XWEATHER_SECRET=xweather-radar-secret",
		"DASH_CONTROL_PIN_HASH=pin-hash-secret",
		"DASH_CONTROL_PIN_SALT=pin-salt-secret",
		`{"apiKey":"inline-api-secret"}`,
		"api_key = other-inline-secret",
	}, "\n")
	got := redactText(input)
	for _, secret := range []string{
		"openweather-secret", "tomorrow-secret", "weatherbit-secret", "visual-secret", "pirate-secret",
		"accu-secret", "meteo-secret", "weatherapi-secret", "xweather-secret", "google-secret",
		"openmeteo-secret", "ninjas-secret", "xweather-id-secret", "xweather-radar-secret",
		"pin-hash-secret", "pin-salt-secret", "inline-api-secret", "other-inline-secret",
	} {
		if strings.Contains(got, secret) {
			t.Fatalf("diagnostics leaked %q: %s", secret, got)
		}
	}
	if strings.Count(got, "<redacted>") < 18 {
		t.Fatalf("expected all secrets to be redacted, got: %s", got)
	}
}
