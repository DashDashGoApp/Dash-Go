package weather

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// radarProviderSpec is intentionally small and server-owned. The browser may
// display the same metadata, but only these known IDs can ever reach the
// keyed tile proxy. That keeps the proxy from becoming an arbitrary URL fetcher.
type radarProviderSpec struct {
	ID          string
	Label       string
	Tier        string
	Kind        string
	KeyRequired bool
}

var radarProviderSpecs = map[string]radarProviderSpec{
	"rainviewer": {"rainviewer", "RainViewer", "free · no key · global · observed history", "frame-index", false},
	"nws":        {"nws", "NWS / NOAA (US)", "free · no key · US-only · latest radar frame", "wms", false},
	"tomorrow":   {"tomorrow", "Tomorrow.io", "key required · metered map tiles", "xyz-keyed", true},
	"weatherbit": {"weatherbit", "Weatherbit Maps", "key required · Maps plan", "xyz-keyed", true},
	"xweather":   {"xweather", "Xweather Maps", "key required · metered map tiles", "xyz-keyed", true},
	"custom_xyz": {"custom_xyz", "Custom XYZ/WMS", "advanced · browser-direct endpoint", "xyz", false},
}

var radarProviderOrder = []string{"rainviewer", "nws", "tomorrow", "weatherbit", "xweather", "custom_xyz"}

type radarConfig struct {
	Provider    string
	Selection   string
	Fallback    string
	Lat         float64
	Lon         float64
	Profile     string
	CustomTiles string
	CustomWMS   string
	Env         map[string]string
}

func radarNormalizeProviderID(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	id = strings.ReplaceAll(id, "-", "_")
	id = strings.ReplaceAll(id, " ", "_")
	if id == "aeris" || id == "aerisweather" || id == "xweather_maps" {
		return "xweather"
	}
	return id
}

func radarKnownProvider(id string) bool {
	_, ok := radarProviderSpecs[radarNormalizeProviderID(id)]
	return ok
}

func radarProfileTier(profile string) string {
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case "lite", "zero2", "low", "low-power", "ultra-lite":
		return "lite"
	case "enhanced", "maximum", "x86", "high":
		return "enhanced"
	default:
		return "balanced"
	}
}

// radarFrameMode describes the source contract rather than a user-selected
// workload budget. RainViewer publishes the observed timeline; other current
// providers use one current frame unless a future source integration explicitly
// adds a timeline.
func radarFrameMode(provider string) string {
	if radarNormalizeProviderID(provider) == "rainviewer" {
		return "source"
	}
	return "latest"
}

func radarAutomaticNWSFallback(lat, lon float64) bool {
	// NWS WMS is a useful current-frame fallback for North American kiosks.
	// Outside its coverage we leave RainViewer as the sole automatic source.
	return lat >= 18 && lat <= 72 && lon >= -170 && lon <= -60
}

func (s *Service) radarConfig() radarConfig {
	cfg := radarConfig{Provider: "rainviewer", Selection: "auto", Lat: 41.8781, Lon: -87.6298, Profile: "balanced", Env: s.radarEnvValues()}
	for _, path := range []string{filepath.Join(s.dash, "ui", "js", "config-defaults.js"), s.configLocal} {
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		txt := string(b)
		cfg.CustomTiles = strings.TrimSpace(jsString(txt, "radarCustomTiles", cfg.CustomTiles))
		cfg.CustomWMS = strings.TrimSpace(jsString(txt, "radarCustomWms", cfg.CustomWMS))
		cfg.Lat = jsFloat(txt, "lat", cfg.Lat)
		cfg.Lon = jsFloat(txt, "lon", cfg.Lon)
	}
	settings := s.loadSettings()
	cfg.Lat = anyFloatDefault(settings["lat"], cfg.Lat)
	cfg.Lon = anyFloatDefault(settings["lon"], cfg.Lon)
	cfg.Profile = strOr(settings["profile"], cfg.Profile)
	if s := strings.TrimSpace(strOr(settings["radarCustomTiles"], "")); s != "" {
		cfg.CustomTiles = s
	}
	if s := strings.TrimSpace(strOr(settings["radarCustomWms"], "")); s != "" {
		cfg.CustomWMS = s
	}
	if radarAutomaticNWSFallback(cfg.Lat, cfg.Lon) {
		cfg.Fallback = "nws"
	}
	return cfg
}

func (s *Service) radarEnvValues() map[string]string {
	path := strings.TrimSpace(os.Getenv("DASH_RADAR_ENV"))
	if path == "" {
		path = filepath.Join(s.home, ".dashboard-radar.env")
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

func radarKeyNames(provider string) []string {
	switch radarNormalizeProviderID(provider) {
	case "tomorrow":
		return []string{"DASH_RADAR_TOMORROW_KEY"}
	case "weatherbit":
		return []string{"DASH_RADAR_WEATHERBIT_KEY"}
	case "xweather":
		return []string{"DASH_RADAR_XWEATHER_ID", "DASH_RADAR_XWEATHER_SECRET"}
	default:
		return nil
	}
}

func (s *Service) radarHasKey(provider string) bool {
	names := radarKeyNames(provider)
	if len(names) == 0 {
		return true
	}
	vals := s.radarEnvValues()
	for _, name := range names {
		if strings.TrimSpace(vals[name]) == "" {
			return false
		}
	}
	return true
}

func (s *Service) radarKey(provider, name string) string {
	return strings.TrimSpace(s.radarEnvValues()[name])
}

func (s *Service) radarStatus() map[string]any {
	cfg := s.radarConfig()
	providers := make([]map[string]any, 0, len(radarProviderOrder))
	for _, id := range radarProviderOrder {
		spec := radarProviderSpecs[id]
		item := map[string]any{
			"id": id, "label": spec.Label, "tier": spec.Tier, "kind": spec.Kind,
			"keyRequired": spec.KeyRequired, "hasKey": s.radarHasKey(id), "active": id == cfg.Provider,
			"animated": radarFrameMode(id) == "source", "frameMode": radarFrameMode(id),
		}
		if until, reason, failures, active := s.providerBackoffActive("radar-" + id); active {
			item["backoff"] = map[string]any{"until": until.Unix(), "reason": reason, "failures": failures}
		}
		providers = append(providers, item)
	}
	slices.SortStableFunc(providers, func(left, right map[string]any) int {
		return compareBoolTrueFirst(left["id"] == cfg.Provider, right["id"] == cfg.Provider)
	})
	return map[string]any{
		"provider": cfg.Provider, "selection": cfg.Selection, "automatic": true, "fallbackProvider": cfg.Fallback,
		"lat": cfg.Lat, "lon": cfg.Lon, "profile": radarProfileTier(cfg.Profile), "providers": providers,
		"customTilesConfigured": cfg.CustomTiles != "", "customWmsConfigured": cfg.CustomWMS != "",
	}
}

func validateRadarSettings(body map[string]any) error {
	if raw, ok := body["radarProvider"]; ok {
		id := radarNormalizeProviderID(fmt.Sprint(raw))
		if id != "auto" && !radarKnownProvider(id) {
			return fmt.Errorf("settings.radarProvider is not supported")
		}
	}
	for _, key := range []string{"radarCustomTiles", "radarCustomWms"} {
		raw, ok := body[key]
		if !ok || raw == nil {
			continue
		}
		s, ok := raw.(string)
		if !ok || len(strings.TrimSpace(s)) > 2048 {
			return fmt.Errorf("settings.%s must be a short string", key)
		}
		if strings.TrimSpace(s) != "" && !strings.HasPrefix(strings.ToLower(strings.TrimSpace(s)), "https://") {
			return fmt.Errorf("settings.%s must use https://", key)
		}
	}
	return nil
}
