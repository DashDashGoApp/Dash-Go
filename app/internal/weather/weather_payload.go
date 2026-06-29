package weather

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

type Config struct {
	Lat          float64
	Lon          float64
	TempUnit     string
	WindUnit     string
	Days         int
	WxAPI        string
	APIKey       string
	Providers    []string
	ProviderKeys map[string]string
}

type weatherFetchJobGo struct {
	Index int
	ID    string
}

type weatherFetchResultGo struct {
	Index  int
	ID     string
	Source map[string]any
	Status map[string]any
}

func (s *Service) weatherPayload() any {
	cachePath := filepath.Join(s.cacheDir, "weather-cache.json")
	cfg := s.Config()
	cacheKey := weatherCacheKeyGo(cfg)
	force := os.Getenv("DASH_WEATHER_FORCE_REFRESH") == "1"
	if !force {
		if cached, ok := s.readWeatherCache(cachePath, cacheKey); ok {
			return cached
		}
	}
	payload, err := s.fetchGoWeatherWithConfig(context.Background(), cfg)
	if err == nil {
		priorSuccess := weatherLastSuccessMillis(cachePath)
		weatherMarkCacheGo(payload, cfg, false, "")
		if !weatherPayloadHasFreshSuccessGo(payload) && priorSuccess > 0 {
			cache := anyMap(payload["cache"])
			cache["lastSuccessAt"] = priorSuccess
			payload["cache"] = cache
		}
		_ = fileio.WriteJSON(cachePath, payload)
		return payload
	}
	if cached, ok := s.readWeatherCacheAny(cachePath); ok {
		if m, okm := cached.(map[string]any); okm {
			m["cache"] = mapMerge(anyMap(m["cache"]), map[string]any{"hit": true, "stale": true, "reason": err.Error(), "cacheKey": cacheKey, "ttlSeconds": int(s.weatherCacheTTL().Seconds())})
			return m
		}
		return cached
	}
	status := []any{
		map[string]any{"id": "openmeteo", "label": "Open-Meteo", "ok": false, "error": err.Error(), "reason": err.Error(), "status": "provider_error", "freshness": "unknown", "generator": "go"},
	}
	return map[string]any{
		"current":            nil,
		"daily":              []any{},
		"hourly":             []any{},
		"alerts":             []any{},
		"selected":           cfg.Providers,
		"sources":            []any{},
		"status":             status,
		"sourceHealth":       status,
		"location":           map[string]any{"lat": cfg.Lat, "lon": cfg.Lon},
		"keyStore":           filepath.Join(s.home, ".dashboard-weather.env"),
		"keysInServedConfig": len(cfg.ProviderKeys) > 0 || strings.TrimSpace(cfg.APIKey) != "",
		"cache":              map[string]any{"hit": false, "ttlSeconds": int(s.weatherCacheTTL().Seconds()), "cacheKey": cacheKey},
		"error":              err.Error(),
		"source":             "go-weather-unavailable",
	}
}

func (s *Service) readWeatherCache(path, cacheKey string) (any, bool) {
	st, err := os.Stat(path)
	if err != nil || st.IsDir() || time.Since(st.ModTime()) > s.weatherCacheTTL() {
		return nil, false
	}
	raw, ok := s.readWeatherCacheAny(path)
	if !ok {
		return nil, false
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, false
	}
	if !weatherPayloadHasLocalDayGo(m, time.Now()) {
		return nil, false
	}
	cache := anyMap(m["cache"])
	if jsonutil.StringValue(cache["cacheKey"]) != cacheKey {
		return nil, false
	}
	cache["hit"] = true
	cache["stale"] = false
	cache["ttlSeconds"] = int(s.weatherCacheTTL().Seconds())
	m["cache"] = cache
	return m, true
}

func (s *Service) readWeatherCacheAny(path string) (any, bool) {
	raw := s.readJSONDefault(path, nil)
	if raw == nil {
		return nil, false
	}
	return raw, true
}

func weatherPayloadHasLocalDayGo(payload any, now time.Time) bool {
	daily := anyMap(anyMap(payload)["daily"])
	today := now.In(time.Local).Format("2006-01-02")
	switch dates := daily["time"].(type) {
	case []any:
		for _, raw := range dates {
			if jsonutil.StringValue(raw) == today {
				return true
			}
		}
	case []string:
		for _, raw := range dates {
			if strings.TrimSpace(raw) == today {
				return true
			}
		}
	}
	return false
}

func (s *Service) weatherCacheTTL() time.Duration {
	return time.Duration(s.weatherRefreshMinutes()) * time.Minute
}

func weatherPayloadHasFreshSuccessGo(payload map[string]any) bool {
	for _, raw := range jsonutil.List(payload["status"]) {
		status := anyMap(raw)
		if status["ok"] != true {
			continue
		}
		freshness := strings.ToLower(jsonutil.StringValue(status["freshness"]))
		if freshness == "" || freshness == "fresh" {
			return true
		}
	}
	return false
}
func weatherMarkCacheGo(payload map[string]any, cfg Config, hit bool, reason string) {
	if payload == nil {
		return
	}
	cache := anyMap(payload["cache"])
	cache["hit"] = hit
	cache["stale"] = false
	cache["cacheKey"] = weatherCacheKeyGo(cfg)
	cache["generatedAt"] = time.Now().Unix()
	if weatherPayloadHasFreshSuccessGo(payload) {
		cache["lastSuccessAt"] = time.Now().UnixMilli()
	}
	if reason != "" {
		cache["reason"] = reason
	}
	payload["cache"] = cache
}

func weatherCacheKeyGo(cfg Config) string {
	keys := slices.Sorted(maps.Keys(cfg.ProviderKeys))
	fps := []string{}
	for _, k := range keys {
		v := strings.TrimSpace(cfg.ProviderKeys[k])
		if v == "" {
			continue
		}
		sum := sha256.Sum256([]byte(v))
		fps = append(fps, k+":"+hex.EncodeToString(sum[:])[:12])
	}
	parts := map[string]any{"lat": cfg.Lat, "lon": cfg.Lon, "tempUnit": cfg.TempUnit, "windUnit": cfg.WindUnit, "days": cfg.Days, "wxApi": cfg.WxAPI, "providers": cfg.Providers, "keyFingerprints": fps}
	b, _ := json.Marshal(parts)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
