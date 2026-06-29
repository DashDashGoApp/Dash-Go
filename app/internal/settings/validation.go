package settings

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// ValidateShape intentionally checks only known safety-critical shapes.
// Unknown keys remain valid so newer releases can add preferences without
// making an older last-good snapshot unusable.
func ValidateShape(values map[string]any, validateRadar RadarValidator) error {
	if values == nil {
		return fmt.Errorf("settings must be a JSON object")
	}
	numberRanges := map[string][2]float64{
		"agendaDays": {1, 90}, "weatherDays": {1, 30}, "weeksBelow": {0, 26}, "weeksAbove": {0, 26},
		"maxEventsPerCell": {1, 24}, "refreshMinutes": {1, 1440}, "calendarRefreshMins": {1, 1440},
	}
	for key, bounds := range numberRanges {
		raw, ok := values[key]
		if !ok || raw == nil {
			continue
		}
		n, ok := NumberValue(raw)
		if !ok || n < bounds[0] || n > bounds[1] {
			return fmt.Errorf("settings.%s must be a number between %g and %g", key, bounds[0], bounds[1])
		}
	}
	for _, key := range []string{"showEventMaps", "showInteractiveMaps", "showWeekNumbers", "enableMoon", "enableWeather"} {
		raw, ok := values[key]
		if !ok || raw == nil {
			continue
		}
		if _, ok := raw.(bool); !ok {
			return fmt.Errorf("settings.%s must be true or false", key)
		}
	}
	if raw, ok := values["weatherDetailMode"]; ok && raw != nil {
		value, ok := raw.(string)
		value = strings.ToLower(strings.TrimSpace(value))
		if !ok || (value != "standard" && value != "expanded") {
			return fmt.Errorf("settings.weatherDetailMode must be standard or expanded")
		}
	}
	if validateRadar != nil {
		if err := validateRadar(values); err != nil {
			return err
		}
	}
	for key, bounds := range map[string][2]float64{"radarLiteFrames": {1, 5}, "radarLiteMaxPx": {384, 768}} {
		raw, ok := values[key]
		if !ok || raw == nil {
			continue
		}
		value, ok := NumberValue(raw)
		if !ok || value < bounds[0] || value > bounds[1] || value != float64(int(value)) {
			return fmt.Errorf("settings.%s must be a whole number between %g and %g", key, bounds[0], bounds[1])
		}
	}
	if raw, ok := values["weatherAlerts"]; ok && raw != nil {
		if _, err := ClampProfileWeatherAlerts(raw); err != nil {
			return err
		}
	}
	if raw, ok := values["profile"]; ok && raw != nil {
		s, ok := raw.(string)
		if !ok || NormalizeProfileName(s) == "" {
			return fmt.Errorf("settings.profile must be a supported profile name")
		}
	}
	return ValidateDashboardTypographySettings(values)
}

// ValidateDashboardTypographySettings keeps the target-scoped Dashboard
// Display typography preferences bounded while preserving general forward
// compatibility for unknown settings keys.
func ValidateDashboardTypographySettings(values map[string]any) error {
	numeric := map[string]map[float64]bool{
		"calendarTextSize":   {-1: true, -0.5: true, 0: true, 0.5: true, 1: true},
		"calendarTextWeight": {400: true, 600: true, 700: true, 800: true, 900: true},
		"clockTextSize":      {-2: true, -1: true, 0: true, 1: true, 2: true},
		"clockTextWeight":    {400: true, 500: true, 600: true, 700: true, 800: true},
		"weatherTextSize":    {-2: true, -1: true, 0: true, 1: true, 2: true},
		"weatherTextWeight":  {400: true, 500: true, 600: true, 700: true, 800: true},
		"messageTextSize":    {-2: true, -1: true, 0: true, 1: true, 2: true},
		"messageTextWeight":  {600: true, 700: true, 800: true, 850: true, 900: true},
	}
	for key, allowed := range numeric {
		raw, ok := values[key]
		if !ok || raw == nil {
			continue
		}
		n, ok := NumberValue(raw)
		if !ok || !allowed[n] {
			return fmt.Errorf("settings.%s must be one of the Dashboard Display choices", key)
		}
	}
	fonts := map[string]bool{"system": true, "rounded": true, "default": true, "readable": true, "mono": true}
	for _, key := range []string{"calendarTextFont", "clockTextFont", "weatherTextFont", "messageTextFont"} {
		raw, ok := values[key]
		if !ok || raw == nil {
			continue
		}
		value, ok := raw.(string)
		if !ok || !fonts[value] {
			return fmt.Errorf("settings.%s must be one of the Dashboard Display font choices", key)
		}
	}
	return nil
}

// NumberValue accepts the decoded numeric forms that have always been valid
// in settings/profile data. It deliberately rejects strings and non-scalars.
func NumberValue(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func finiteWhole(n float64) bool { return !math.IsNaN(n) && !math.IsInf(n, 0) && math.Trunc(n) == n }
