package weather

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func weatherProviderCacheKeyGo(id string, cfg Config) string {
	key := weatherProviderKeyGo(id, cfg)
	fp := ""
	if key != "" {
		sum := sha256.Sum256([]byte(key))
		fp = hex.EncodeToString(sum[:])[:12]
	}
	parts := map[string]any{"provider": weatherNormalizeProviderIDGo(id), "lat": cfg.Lat, "lon": cfg.Lon, "tempUnit": cfg.TempUnit, "windUnit": cfg.WindUnit, "days": cfg.Days, "wxApi": cfg.WxAPI, "keyFingerprint": fp}
	b, _ := json.Marshal(parts)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func (s *Service) weatherProviderCachePathGo(id string) string {
	return filepath.Join(s.cacheDir, "weather-provider-"+weatherNormalizeProviderIDGo(id)+".json")
}

func (s *Service) readWeatherProviderCacheGo(id, cacheKey string, allowStale bool) (map[string]any, int64, bool) {
	path := s.weatherProviderCachePathGo(id)
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		return nil, 0, false
	}
	age := int64(time.Since(st.ModTime()).Seconds())
	freshTTL := int64(s.weatherCacheTTL().Seconds())
	staleTTL := int64(s.weatherProviderStaleTTLGo(id).Seconds())
	if !allowStale && age > freshTTL {
		return nil, age, false
	}
	if allowStale && age > staleTTL {
		return nil, age, false
	}
	raw := s.readJSONDefault(path, nil)
	m := anyMap(raw)
	if jsonutil.StringValue(m["cacheKey"]) != cacheKey {
		return nil, age, false
	}
	src := anyMap(m["source"])
	if len(src) == 0 {
		return nil, age, false
	}
	// A cache written shortly before midnight can still be younger than the
	// normal refresh TTL after the local day rolls over. It is not a fresh
	// forecast anymore; force one live attempt before allowing stale fallback.
	if !allowStale && !weatherPayloadHasLocalDayGo(src, time.Now()) {
		return nil, age, false
	}
	return src, age, true
}

func (s *Service) writeWeatherProviderCacheGo(id, cacheKey string, src map[string]any) error {
	if src == nil {
		return nil
	}
	payload := map[string]any{"provider": weatherNormalizeProviderIDGo(id), "cacheKey": cacheKey, "storedAt": time.Now().Unix(), "source": src}
	return fileio.WriteJSON(s.weatherProviderCachePathGo(id), payload)
}

func (s *Service) weatherProviderStaleTTLGo(id string) time.Duration {
	// The dashboard keeps provider data usable as a stale safety net when live APIs are
	// down/rate-limited. Keep the Go stale window long enough to keep the kiosk
	// boring, but not so long that stale weather silently lingers for days.
	base := 18 * time.Hour
	if id == "nws" || id == "openmeteo" || id == "openmeteo-custom" {
		base = 12 * time.Hour
	}
	return base
}

func weatherMarkProviderCacheHitGo(src map[string]any, ageSeconds int64) {
	if src == nil {
		return
	}
	src["_providerCacheHit"] = true
	src["_providerCacheAgeSeconds"] = ageSeconds
	src["_stale"] = false
}

func weatherMarkProviderStaleGo(src map[string]any, ageSeconds int64, reason string) {
	if src == nil {
		return
	}
	src["_providerCacheHit"] = true
	src["_providerCacheAgeSeconds"] = ageSeconds
	src["_stale"] = true
	src["_staleReason"] = reason
}

func (s *Service) weatherRateStatePathGo() string {
	return filepath.Join(s.cacheDir, "weather-provider-rate-state.json")
}

func (s *Service) readWeatherRateStateGo() map[string]any {
	return anyMap(s.readJSONDefault(s.weatherRateStatePathGo(), map[string]any{}))
}

func (s *Service) writeWeatherRateStateGo(st map[string]any) {
	_ = fileio.WriteJSON(s.weatherRateStatePathGo(), st)
}

func (s *Service) weatherProviderCooldownGo(id string) (time.Time, string, bool) {
	st := s.readWeatherRateStateGo()
	providers := anyMap(st["providers"])
	p := anyMap(providers[weatherNormalizeProviderIDGo(id)])
	untilUnix, ok := toFloatGo(p["cooldownUntil"])
	if !ok || untilUnix <= 0 {
		return time.Time{}, "", false
	}
	until := time.Unix(int64(untilUnix), 0)
	if time.Now().Before(until) {
		return until, jsonutil.TextValue(p["lastError"]), true
	}
	return time.Time{}, "", false
}

func (s *Service) noteWeatherProviderErrorGo(id string, err error) {
	if err == nil {
		return
	}
	msg := err.Error()
	status := weatherClassifyProviderErrorGo(msg)
	st := s.readWeatherRateStateGo()
	providers := anyMap(st["providers"])
	pid := weatherNormalizeProviderIDGo(id)
	p := anyMap(providers[pid])
	prev, _ := toFloatGo(p["consecutiveErrors"])
	p["consecutiveErrors"] = int(prev) + 1
	p["lastError"] = msg
	p["lastStatus"] = status
	p["lastErrorAt"] = time.Now().Unix()
	if status == "rate_limited" {
		p["cooldownUntil"] = time.Now().Add(30 * time.Minute).Unix()
	} else if status == "auth_error" {
		// Do not hammer providers when a saved key is rejected.
		p["cooldownUntil"] = time.Now().Add(10 * time.Minute).Unix()
	} else if int(prev)+1 >= 3 {
		p["cooldownUntil"] = time.Now().Add(5 * time.Minute).Unix()
	}
	providers[pid] = p
	st["providers"] = providers
	s.writeWeatherRateStateGo(st)
}

func (s *Service) clearWeatherProviderCooldownGo(id string) {
	st := s.readWeatherRateStateGo()
	providers := anyMap(st["providers"])
	pid := weatherNormalizeProviderIDGo(id)
	p := anyMap(providers[pid])
	if len(p) == 0 {
		return
	}
	p["consecutiveErrors"] = 0
	p["lastSuccessAt"] = time.Now().Unix()
	delete(p, "cooldownUntil")
	providers[pid] = p
	st["providers"] = providers
	s.writeWeatherRateStateGo(st)
}

func weatherClassifyProviderErrorGo(msg string) string {
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "http 429") || strings.Contains(lower, "rate") || strings.Contains(lower, "quota") || strings.Contains(lower, "limit"):
		return "rate_limited"
	case strings.Contains(lower, "http 401") || strings.Contains(lower, "http 403") || strings.Contains(lower, "api key") || strings.Contains(lower, "unauthorized") || strings.Contains(lower, "forbidden"):
		return "auth_error"
	case strings.Contains(lower, "timeout") || strings.Contains(lower, "deadline") || strings.Contains(lower, "temporarily"):
		return "temporary_error"
	default:
		return "provider_error"
	}
}

func weatherProviderFetchConfigGo(id string, cfg Config) Config {
	out := cfg
	if maxDays := weatherProviderMaxDays(id); maxDays > 0 {
		out.Days = clamp(out.Days, 1, maxDays)
	} else {
		out.Days = clamp(out.Days, 1, 16)
	}
	return out
}

func weatherNormalizeProviderIDGo(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	switch id {
	case "metno", "meteosource":
		return "weatherbit"
	case "google-weather", "google_weather", "google":
		return "googleweather"
	case "weather-api", "weather_api":
		return "weatherapi"
	case "open-meteo", "open_meteo":
		return "openmeteo"
	case "open-meteo-custom", "open_meteo_custom":
		return "openmeteo-custom"
	case "pirate-weather", "pirate_weather":
		return "pirateweather"
	case "visual-crossing", "visual_crossing":
		return "visualcrossing"
	}
	return id
}

func normalizeWeatherProviderListGo(xs []string) []string {
	supported := map[string]bool{"openmeteo": true, "openmeteo-custom": true, "nws": true, "weatherapi": true, "openweather": true, "googleweather": true, "tomorrow": true, "visualcrossing": true, "weatherbit": true, "pirateweather": true, "accuweather": true, "xweather": true}
	seen := map[string]bool{}
	out := []string{}
	for _, raw := range xs {
		id := weatherNormalizeProviderIDGo(raw)
		if id != "" && supported[id] && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}
