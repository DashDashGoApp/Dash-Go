package weather

import (
	"errors"
	"strings"
	"testing"
)

func TestWeatherProviderErrorsNeverPersistCredentials(t *testing.T) {
	cases := []struct {
		name       string
		message    string
		secrets    []string
		mustRemain []string
	}{
		{
			name:       "visual crossing timeout quotes the key",
			message:    `Get "https://weather.visualcrossing.com/VisualCrossingWebServices/rest/services/timeline/41.8,-87.6?include=current%2Cdays&key=ABC123SECRET&unitGroup=metric": context deadline exceeded`,
			secrets:    []string{"ABC123SECRET"},
			mustRemain: []string{"context deadline exceeded", "weather.visualcrossing.com", "unitGroup=metric"},
		},
		{
			name:       "openweather appid",
			message:    `Get "https://api.openweathermap.org/data/3.0/onecall?lat=41.8&lon=-87.6&appid=OWMSECRET9&units=metric": dial tcp: i/o timeout`,
			secrets:    []string{"OWMSECRET9"},
			mustRemain: []string{"i/o timeout", "api.openweathermap.org", "lat=41.8"},
		},
		{
			name:       "weatherapi key",
			message:    `Get "https://api.weatherapi.com/v1/forecast.json?key=WAPISECRET&q=41.8,-87.6&days=3": EOF`,
			secrets:    []string{"WAPISECRET"},
			mustRemain: []string{"EOF", "api.weatherapi.com"},
		},
		{
			name:       "xweather client secret",
			message:    `Get "https://api.xweather.com/forecast?client_id=CID&client_secret=XWSECRET&filter=1hr": unexpected EOF`,
			secrets:    []string{"XWSECRET"},
			mustRemain: []string{"unexpected EOF", "api.xweather.com", "client_id=CID"},
		},
		{
			name:       "subscription key is not matched as a bare key",
			message:    `Get "https://atlas.microsoft.com/weather?subscription-key=AZSECRET": timeout`,
			secrets:    []string{"AZSECRET"},
			mustRemain: []string{"atlas.microsoft.com"},
		},
	}
	for _, tc := range cases {
		got := redactWeatherErrorGo(tc.message)
		for _, secret := range tc.secrets {
			if strings.Contains(got, secret) {
				t.Errorf("%s: credential survived redaction: %q", tc.name, got)
			}
		}
		if !strings.Contains(got, "[redacted]") {
			t.Errorf("%s: expected a redaction marker, got %q", tc.name, got)
		}
		for _, keep := range tc.mustRemain {
			if !strings.Contains(got, keep) {
				t.Errorf("%s: redaction dropped diagnostic detail %q from %q", tc.name, keep, got)
			}
		}
	}

	if got := redactWeatherErrorGo(""); got != "" {
		t.Errorf("an empty message must stay empty, got %q", got)
	}
	plain := "dial tcp: no such host"
	if got := redactWeatherErrorGo(plain); got != plain {
		t.Errorf("a message with no credential must be untouched, got %q", got)
	}
	// The wrapper returns the original error when nothing needed redacting, so
	// error classification downstream sees exactly the same value as before.
	plainErr := errors.New("dial tcp: connection refused")
	if got := redactWeatherError(plainErr); got != plainErr {
		t.Errorf("an error without a credential must be returned as-is")
	}
	if redactWeatherError(nil) != nil {
		t.Errorf("a nil error must stay nil")
	}
}
