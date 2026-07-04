package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func captureDoctorConfigLocationCheck(t *testing.T, a *app) (int, string) {
	t.Helper()
	old := os.Stdout
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = write
	code := a.runDoctorConfigCLI([]string{"--location-check"})
	if err := write.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = old
	out, err := io.ReadAll(read)
	if err != nil {
		t.Fatal(err)
	}
	if err := read.Close(); err != nil {
		t.Fatal(err)
	}
	return code, string(out)
}

func TestDoctorLocationCheckAcceptsCurrentRootWeatherCacheLocation(t *testing.T) {
	a := testApp(t)
	cache := `{"location":{"lat":38.73172,"lon":-90.08038}}`
	if err := os.WriteFile(filepath.Join(a.cacheDir, "weather-cache.json"), []byte(cache), 0644); err != nil {
		t.Fatal(err)
	}
	code, out := captureDoctorConfigLocationCheck(t, a)
	if code != 0 {
		t.Fatalf("location check exit = %d, output: %s", code, out)
	}
	if !strings.Contains(out, "LOCATION_OK:41.878100,-87.629800:Chicago") {
		t.Fatalf("missing configured location in output: %s", out)
	}
	if strings.Contains(out, "WEATHER_CACHE_ZERO_LOCATION") {
		t.Fatalf("current root weather cache produced a false zero-location finding: %s", out)
	}
}

func TestDoctorLocationCheckRecognizesOnlyPresentZeroCacheCoordinates(t *testing.T) {
	cases := []struct {
		name  string
		cache string
		want  bool
	}{
		{name: "root zero", cache: `{"location":{"lat":0,"lon":0}}`, want: true},
		{name: "legacy payload zero", cache: `{"payload":{"location":{"lat":0,"lon":0}}}`, want: true},
		{name: "missing location", cache: `{"payload":{}}`, want: false},
		{name: "partial location", cache: `{"location":{"lat":0}}`, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := testApp(t)
			if err := os.WriteFile(filepath.Join(a.cacheDir, "weather-cache.json"), []byte(tc.cache), 0644); err != nil {
				t.Fatal(err)
			}
			code, out := captureDoctorConfigLocationCheck(t, a)
			if code != 0 {
				t.Fatalf("location check exit = %d, output: %s", code, out)
			}
			got := strings.Contains(out, "WEATHER_CACHE_ZERO_LOCATION")
			if got != tc.want {
				t.Fatalf("zero finding = %v, want %v, output: %s", got, tc.want, out)
			}
		})
	}
}
