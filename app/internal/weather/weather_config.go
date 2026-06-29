package weather

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func (s *Service) Config() Config {
	settings := s.loadSettings()
	cfg := Config{
		Lat:          41.8781,
		Lon:          -87.6298,
		TempUnit:     "fahrenheit",
		WindUnit:     "mph",
		Days:         16,
		WxAPI:        "https://api.open-meteo.com",
		APIKey:       "",
		Providers:    []string{"openmeteo"},
		ProviderKeys: map[string]string{},
	}
	if b, err := os.ReadFile(filepath.Join(s.dash, "ui", "js", "config-defaults.js")); err == nil {
		txt := string(b)
		cfg.Lat = jsFloat(txt, "lat", cfg.Lat)
		cfg.Lon = jsFloat(txt, "lon", cfg.Lon)
		cfg.TempUnit = jsString(txt, "tempUnit", cfg.TempUnit)
		cfg.WindUnit = jsString(txt, "windUnit", cfg.WindUnit)
		cfg.WxAPI = jsString(txt, "wxApi", cfg.WxAPI)
		cfg.APIKey = jsString(txt, "apiKey", cfg.APIKey)
		mergeStringMap(cfg.ProviderKeys, jsStringMap(txt, "weatherProviderKeys"))
		cfg.Providers = jsStringArray(txt, "weatherProviders", cfg.Providers)
	}
	if b, err := os.ReadFile(s.configLocal); err == nil {
		txt := string(b)
		cfg.Lat = jsFloat(txt, "lat", cfg.Lat)
		cfg.Lon = jsFloat(txt, "lon", cfg.Lon)
		cfg.TempUnit = jsString(txt, "tempUnit", cfg.TempUnit)
		cfg.WindUnit = jsString(txt, "windUnit", cfg.WindUnit)
		cfg.WxAPI = jsString(txt, "wxApi", cfg.WxAPI)
		cfg.APIKey = jsString(txt, "apiKey", cfg.APIKey)
		mergeStringMap(cfg.ProviderKeys, jsStringMap(txt, "weatherProviderKeys"))
		cfg.Providers = jsStringArray(txt, "weatherProviders", cfg.Providers)
	}
	cfg.Lat = anyFloatDefault(settings["lat"], cfg.Lat)
	cfg.Lon = anyFloatDefault(settings["lon"], cfg.Lon)
	cfg.TempUnit = normalizeTempUnit(strOr(settings["tempUnit"], cfg.TempUnit))
	cfg.WindUnit = normalizeWindUnit(strOr(settings["windUnit"], cfg.WindUnit))
	// Forecast horizon is source-owned: providers return their maximum available
	// daily range and blending retains the furthest supplied date, up to 16 days.
	cfg.Days = 16
	if k := strings.TrimSpace(strOr(settings["apiKey"], "")); k != "" {
		cfg.APIKey = k
	}
	if legacy, ok := settings["weatherProviderKeys"].(map[string]any); ok {
		for k, v := range legacy {
			if val := jsonutil.StringValue(v); val != "" {
				cfg.ProviderKeys[weatherNormalizeProviderIDGo(k)] = val
			}
		}
	}
	for _, id := range []string{"weatherapi", "openweather", "googleweather", "tomorrow", "visualcrossing", "weatherbit", "pirateweather", "accuweather", "xweather", "openmeteo-custom"} {
		for _, k := range weatherSettingsKeyCandidatesGo(id) {
			if val := strings.TrimSpace(strOr(settings[k], "")); val != "" {
				cfg.ProviderKeys[id] = val
			}
		}
	}
	if s := strings.TrimSpace(strOr(settings["wxApi"], "")); s != "" {
		cfg.WxAPI = s
	}
	if arr, ok := settings["weatherProviders"].([]any); ok && len(arr) > 0 {
		cfg.Providers = []string{}
		for _, v := range arr {
			if s := strings.ToLower(jsonutil.StringValue(v)); s != "" {
				cfg.Providers = append(cfg.Providers, s)
			}
		}
	}
	cfg.Providers = normalizeWeatherProviderListGo(cfg.Providers)
	if len(cfg.Providers) == 0 {
		cfg.Providers = []string{"openmeteo"}
	}
	return cfg
}

func jsFloat(txt, key string, def float64) float64 {
	re := regexp.MustCompile(`(?m)(?:"` + regexp.QuoteMeta(key) + `"|` + regexp.QuoteMeta(key) + `)\s*:\s*(-?\d+(?:\.\d+)?)`)
	m := re.FindStringSubmatch(txt)
	if len(m) < 2 {
		return def
	}
	f, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return def
	}
	return f
}

func jsString(txt, key, def string) string {
	re := regexp.MustCompile(`(?m)(?:"` + regexp.QuoteMeta(key) + `"|` + regexp.QuoteMeta(key) + `)\s*:\s*["']([^"']*)["']`)
	m := re.FindStringSubmatch(txt)
	if len(m) < 2 || strings.TrimSpace(m[1]) == "" {
		return def
	}
	return strings.TrimSpace(m[1])
}

func jsStringArray(txt, key string, def []string) []string {
	re := regexp.MustCompile(`(?ms)(?:"` + regexp.QuoteMeta(key) + `"|` + regexp.QuoteMeta(key) + `)\s*:\s*\[([^\]]*)\]`)
	m := re.FindStringSubmatch(txt)
	if len(m) < 2 {
		return def
	}
	itemRe := reJSStringArrayItem
	out := []string{}
	for _, im := range itemRe.FindAllStringSubmatch(m[1], -1) {
		s := strings.ToLower(strings.TrimSpace(im[1]))
		if s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return def
	}
	return out
}

func jsStringMap(txt, key string) map[string]string {
	out := map[string]string{}
	re := regexp.MustCompile(`(?ms)(?:"` + regexp.QuoteMeta(key) + `"|` + regexp.QuoteMeta(key) + `)\s*:\s*\{(.*?)\}`)
	m := re.FindStringSubmatch(txt)
	if len(m) < 2 {
		return out
	}
	itemRe := reJSStringMapItem
	for _, im := range itemRe.FindAllStringSubmatch(m[1], -1) {
		k := weatherNormalizeProviderIDGo(im[1])
		v := strings.TrimSpace(im[2])
		if k != "" && v != "" {
			out[k] = v
		}
	}
	return out
}

func mergeStringMap(dst, src map[string]string) {
	for k, v := range src {
		if strings.TrimSpace(v) != "" {
			dst[weatherNormalizeProviderIDGo(k)] = strings.TrimSpace(v)
		}
	}
}

func anyFloatDefault(v any, def float64) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case json.Number:
		if f, err := x.Float64(); err == nil {
			return f
		}
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(x), 64); err == nil {
			return f
		}
	}
	return def
}

func normalizeTempUnit(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "c", "celsius", "metric":
		return "celsius"
	default:
		return "fahrenheit"
	}
}

func normalizeWindUnit(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "kmh", "km/h", "kph":
		return "kmh"
	case "ms", "m/s":
		return "ms"
	default:
		return "mph"
	}
}

func trimFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func weatherProviderLabel(id string) string {
	switch strings.ToLower(id) {
	case "openmeteo", "openmeteo-custom":
		return "Open-Meteo"
	case "nws":
		return "NWS / NOAA"
	case "weatherapi":
		return "WeatherAPI.com"
	case "openweather":
		return "OpenWeather"
	case "googleweather":
		return "Google Weather"
	case "tomorrow":
		return "Tomorrow.io"
	case "visualcrossing":
		return "Visual Crossing"
	case "weatherbit":
		return "Weatherbit"
	case "pirateweather":
		return "Pirate Weather"
	case "accuweather":
		return "AccuWeather"
	case "xweather":
		return "Xweather"
	default:
		if id == "" {
			return "Weather source"
		}
		return id
	}
}
