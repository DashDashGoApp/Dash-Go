package weather

import (
	"regexp"
	"strings"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

var (
	reDefaultCalendarKey = regexp.MustCompile(`^[A-Z0-9_]+$`)
	reLocationName       = regexp.MustCompile(`locationName:\s*"([^"]*)"`)
	reSkyLatitude        = regexp.MustCompile(`\blat:\s*([-0-9.]+)`)
	reSkyLongitude       = regexp.MustCompile(`\blon:\s*([-0-9.]+)`)
	reJSStringArrayItem  = regexp.MustCompile(`["']([^"']+)["']`)
	reJSStringMapItem    = regexp.MustCompile(`(?m)["']?([A-Za-z0-9_-]+)["']?\s*:\s*["']([^"']*)["']`)
	reWeatherSnow        = regexp.MustCompile(`snow|sleet|ice|flurr`)
	reWeatherRain        = regexp.MustCompile(`rain|shower|drizzle`)
	reWeatherFog         = regexp.MustCompile(`fog|mist|haze`)
	reFirstNumber        = regexp.MustCompile(`[\d.]+`)
)

func strOr(v any, def string) string {
	if value := jsonutil.TextValue(v); value != "" {
		return value
	}
	return def
}

func clamp(value, lower, upper int) int {
	return min(max(value, lower), upper)
}

func compareBoolTrueFirst(left, right bool) int {
	if left == right {
		return 0
	}
	if left {
		return -1
	}
	return 1
}

// weatherLastSuccessMillis reads the same aggregate-cache success marker used
// by platform health. Keeping the read local avoids a dependency back into core.
func weatherLastSuccessMillis(path string) int64 {
	raw := readJSONDefault(path, map[string]any{})
	cache := anyMap(anyMap(raw)["cache"])
	value, ok := toFloatGo(cache["lastSuccessAt"])
	if !ok || value <= 0 {
		return 0
	}
	millis := int64(value)
	if millis < 100000000000 {
		millis *= 1000
	}
	return millis
}

func readJSONDefault(path string, def any) any {
	// A temporary Service is not needed: this helper is intentionally pure path
	// decoding for weather cache metadata.
	var service Service
	return service.readJSONDefault(path, def)
}

func normalizeProfileName(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "lite", "zero2", "low", "low-power":
		return "lite"
	case "enhanced", "maximum", "x86", "x86_64", "amd64":
		return "enhanced"
	case "balanced":
		return "balanced"
	default:
		return ""
	}
}
