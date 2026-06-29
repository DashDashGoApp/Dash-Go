package weather

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func weatherHealthOKGo(id string, cfg Config, src map[string]any) map[string]any {
	item := map[string]any{"id": id, "label": weatherProviderLabel(id), "ok": true, "enabled": true, "status": "active", "freshness": "fresh", "reason": "Source returned fresh data", "generator": "go", "keyRequired": weatherProviderNeedsKey(id), "hasKey": true}
	if d, ok := src["daily"].(map[string]any); ok {
		item["daysReturned"] = len(jsonutil.List(d["time"]))
	}
	if days := weatherProviderMaxDays(id); days > 0 {
		item["maxDays"] = days
	}
	return item
}

func weatherHealthErrorGo(id string, cfg Config, msg string, disabled bool) map[string]any {
	item := map[string]any{"id": id, "label": weatherProviderLabel(id), "ok": false, "enabled": !disabled, "disabled": disabled, "status": "provider_error", "freshness": "unknown", "error": msg, "reason": msg, "generator": "go", "keyRequired": weatherProviderNeedsKey(id), "hasKey": !weatherProviderNeedsKey(id) || weatherProviderKeyGo(id, cfg) != ""}
	if disabled && item["keyRequired"] == true && item["hasKey"] == false {
		item["status"] = "missing_key"
	} else if disabled {
		item["status"] = "disabled"
	} else {
		item["status"] = weatherClassifyProviderErrorGo(msg)
	}
	if days := weatherProviderMaxDays(id); days > 0 {
		item["maxDays"] = days
	}
	return item
}

func weatherProviderNeedsKey(id string) bool {
	switch strings.ToLower(id) {
	case "weatherapi", "openweather", "googleweather", "tomorrow", "visualcrossing", "weatherbit", "pirateweather", "accuweather", "xweather", "openmeteo-custom":
		return true
	default:
		return false
	}
}

func weatherProviderMaxDays(id string) int {
	switch strings.ToLower(strings.TrimSpace(id)) {
	case "openmeteo", "openmeteo-custom":
		return 16
	case "nws":
		return 7
	case "weatherapi":
		return 3
	case "openweather":
		return 8
	case "googleweather":
		return 10
	case "tomorrow":
		return 5
	case "visualcrossing":
		return 15
	case "weatherbit":
		return 7
	case "pirateweather":
		return 8
	case "accuweather":
		return 5
	case "xweather":
		return 15
	default:
		return 0
	}
}

func weatherProviderKeyGo(provider string, cfg Config) string {
	provider = weatherNormalizeProviderIDGo(provider)
	if cfg.ProviderKeys != nil {
		if k := strings.TrimSpace(cfg.ProviderKeys[provider]); k != "" {
			return k
		}
	}
	if provider == "openmeteo-custom" && strings.TrimSpace(cfg.APIKey) != "" {
		return strings.TrimSpace(cfg.APIKey)
	}
	env := weatherEnvValuesGo()
	for _, name := range weatherEnvKeyCandidatesGo(provider) {
		if k := strings.TrimSpace(env[name]); k != "" {
			return k
		}
	}
	settings := readJSONDefaultMap(filepath.Join(defaultHomeGo(), "dashboard", "config", "settings.json"))
	if legacy, ok := settings["weatherProviderKeys"].(map[string]any); ok {
		if k := jsonutil.StringValue(legacy[provider]); k != "" {
			return k
		}
	}
	for _, kname := range weatherSettingsKeyCandidatesGo(provider) {
		if k := jsonutil.StringValue(settings[kname]); k != "" {
			return k
		}
	}
	if provider == "openmeteo-custom" {
		if k := jsonutil.StringValue(settings["apiKey"]); k != "" {
			return k
		}
	}
	return ""
}

func weatherEnvKeyCandidatesGo(provider string) []string {
	provider = weatherNormalizeProviderIDGo(provider)
	base := map[string][]string{
		"weatherapi":       {"DASH_WEATHERAPI_KEY", "DASH_WEATHER_API_KEY", "WEATHERAPI_KEY", "WEATHER_API_KEY"},
		"openweather":      {"DASH_OPENWEATHER_KEY", "DASH_OPEN_WEATHER_KEY", "OPENWEATHER_KEY", "OPEN_WEATHER_KEY"},
		"googleweather":    {"DASH_GOOGLE_WEATHER_KEY", "DASH_GOOGLEWEATHER_KEY", "GOOGLE_WEATHER_KEY", "GOOGLEWEATHER_KEY"},
		"tomorrow":         {"DASH_TOMORROW_KEY", "TOMORROW_KEY", "TOMORROWIO_KEY", "TOMORROW_IO_KEY"},
		"visualcrossing":   {"DASH_VISUALCROSSING_KEY", "DASH_VISUAL_CROSSING_KEY", "VISUALCROSSING_KEY", "VISUAL_CROSSING_KEY"},
		"weatherbit":       {"DASH_WEATHERBIT_KEY", "DASH_METEOSOURCE_KEY", "WEATHERBIT_KEY", "METEOSOURCE_KEY"},
		"pirateweather":    {"DASH_PIRATEWEATHER_KEY", "DASH_PIRATE_WEATHER_KEY", "PIRATEWEATHER_KEY", "PIRATE_WEATHER_KEY"},
		"accuweather":      {"DASH_ACCUWEATHER_KEY", "DASH_ACCU_WEATHER_KEY", "ACCUWEATHER_KEY", "ACCU_WEATHER_KEY"},
		"xweather":         {"DASH_XWEATHER_KEY", "DASH_X_WEATHER_KEY", "XWEATHER_KEY", "X_WEATHER_KEY"},
		"openmeteo-custom": {"DASH_OPENMETEO_CUSTOM_KEY", "DASH_OPEN_METEO_CUSTOM_KEY", "OPENMETEO_CUSTOM_KEY", "OPEN_METEO_CUSTOM_KEY"},
	}
	out := append([]string{}, base[provider]...)
	generic := "DASH_WEATHER_" + strings.ToUpper(strings.ReplaceAll(provider, "-", "_")) + "_KEY"
	out = append(out, generic)
	return out
}

func weatherSettingsKeyCandidatesGo(provider string) []string {
	provider = weatherNormalizeProviderIDGo(provider)
	camel := strings.ReplaceAll(provider, "-", "")
	return []string{provider + "Key", camel + "Key", "weather_" + strings.ReplaceAll(provider, "-", "_") + "_key", "DASH_WEATHER_" + strings.ToUpper(strings.ReplaceAll(provider, "-", "_")) + "_KEY"}
}

func weatherEnvValuesGo() map[string]string {
	path := os.Getenv("DASH_WEATHER_ENV")
	if path == "" {
		path = filepath.Join(defaultHomeGo(), ".dashboard-weather.env")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return map[string]string{}
	}
	out := map[string]string{}
	for line := range strings.SplitSeq(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		out[strings.TrimSpace(parts[0])] = strings.Trim(strings.TrimSpace(parts[1]), `"'`)
	}
	return out
}

func defaultHomeGo() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return "."
}

func readJSONDefaultMap(path string) map[string]any {
	b, err := os.ReadFile(path)
	if err != nil {
		return map[string]any{}
	}
	var m map[string]any
	if json.Unmarshal(b, &m) != nil {
		return map[string]any{}
	}
	return m
}
