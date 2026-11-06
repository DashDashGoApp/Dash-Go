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
	Lat               float64
	Lon               float64
	TempUnit          string
	WindUnit          string
	Days              int
	WxAPI             string
	AQAPI             string
	APIKey            string
	Providers         []string
	ProviderKeys      map[string]string
	AllowPrivateWxAPI bool
	CacheDir          string
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
	s.weatherFetchMu.Lock()
	defer s.weatherFetchMu.Unlock()
	cachePath := filepath.Join(s.cacheDir, "weather-cache.json")
	cfg := s.Config()
	cacheKey := weatherCacheKeyGo(cfg)
	force := os.Getenv("DASH_WEATHER_FORCE_REFRESH") == "1"
	if !force {
		if cached, ok := s.readWeatherCache(cachePath, cacheKey); ok {
			return cached
		}
	}
	payload, err := s.fetchGoWeatherWithConfig(context.Background(), cfg, false)
	if err == nil {
		priorSuccess := weatherLastSuccessMillis(cachePath)
		weatherMarkCacheGo(payload, cfg, false, "")
		if !weatherPayloadHasFreshSuccessGo(payload) && priorSuccess > 0 {
			cache := anyMap(payload["cache"])
			cache["lastSuccessAt"] = priorSuccess
			payload["cache"] = cache
		}
		_ = fileio.WriteJSON(cachePath, payload)
		s.invalidateWeatherCacheParse()
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

func (s *Service) refreshWeatherLive(ctx context.Context) (map[string]any, error) {
	s.weatherFetchMu.Lock()
	defer s.weatherFetchMu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cfg := s.Config()
	payload, err := s.fetchGoWeatherWithConfig(ctx, cfg, true)
	if err != nil {
		return nil, err
	}
	cachePath := filepath.Join(s.cacheDir, "weather-cache.json")
	priorSuccess := weatherLastSuccessMillis(cachePath)
	weatherMarkCacheGo(payload, cfg, false, "manual live refresh")
	if !weatherPayloadHasFreshSuccessGo(payload) && priorSuccess > 0 {
		cache := anyMap(payload["cache"])
		cache["lastSuccessAt"] = priorSuccess
		payload["cache"] = cache
	}
	if err := fileio.WriteJSON(cachePath, payload); err != nil {
		return nil, err
	}
	s.invalidateWeatherCacheParse()
	return payload, nil
}

func (s *Service) readWeatherCache(path, cacheKey string) (any, bool) {
	st, err := os.Stat(path)
	if err != nil || st.IsDir() || time.Since(st.ModTime()) > s.weatherCacheTTL() {
		return nil, false
	}
	m, ok := s.readWeatherCacheParsedMap(path, st)
	if !ok {
		return nil, false
	}
	if !weatherPayloadHasLocalDayGo(m, time.Now()) {
		return nil, false
	}
	oldCache := anyMap(m["cache"])
	if jsonutil.StringValue(oldCache["cacheKey"]) != cacheKey {
		return nil, false
	}
	// m is a per-call private copy, so the hit markers below can be applied
	// in place without touching the shared parsed document.
	cache := make(map[string]any, len(oldCache)+3)
	for k, v := range oldCache {
		cache[k] = v
	}
	cache["hit"] = true
	cache["stale"] = false
	cache["ttlSeconds"] = int(s.weatherCacheTTL().Seconds())
	m["cache"] = cache
	return m, true
}

func (s *Service) readWeatherCacheAny(path string) (any, bool) {
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		return nil, false
	}
	return s.readWeatherCacheParsed(path, st)
}

// readWeatherCacheParsed returns a private copy of the parsed weather-cache
// document. The parsed document itself is cached and shared, keyed by file
// mtime+size, so warm calls skip the file read and JSON decode entirely.
// Every caller receives its own top-level map plus a copied cache sub-map and
// may annotate them without affecting any other call or the shared document.
func (s *Service) readWeatherCacheParsed(path string, st os.FileInfo) (any, bool) {
	var value any
	s.weatherParseMu.Lock()
	if s.weatherParsePath == path && s.weatherParseValid && st.ModTime().Equal(s.weatherParseMod) && st.Size() == s.weatherParseSize {
		value = s.weatherParseValue
		s.weatherParseMu.Unlock()
	} else {
		value = s.readJSONDefault(path, nil)
		if value == nil {
			s.weatherParseValid = false
			s.weatherParseValue = nil
			s.weatherParseMu.Unlock()
			return nil, false
		}
		s.weatherParsePath = path
		s.weatherParseMod = st.ModTime()
		s.weatherParseSize = st.Size()
		s.weatherParseValue = value
		s.weatherParseValid = true
		s.weatherParseMu.Unlock()
	}
	m, isMap := value.(map[string]any)
	if !isMap {
		return value, true
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	if old, okc := m["cache"].(map[string]any); okc && old != nil {
		cache := make(map[string]any, len(old)+2)
		for k, v := range old {
			cache[k] = v
		}
		out["cache"] = cache
	}
	return out, true
}

// readWeatherCacheParsedMap is the map-shaped view of readWeatherCacheParsed
// used by the cache-hit path, which only accepts document objects.
func (s *Service) readWeatherCacheParsedMap(path string, st os.FileInfo) (map[string]any, bool) {
	value, ok := s.readWeatherCacheParsed(path, st)
	if !ok {
		return nil, false
	}
	m, isMap := value.(map[string]any)
	return m, isMap
}

// invalidateWeatherCacheParse drops the shared parsed-document cache after a
// refresh or write so the next read reflects the rewritten file even when its
// mtime+size happen to be unchanged.
func (s *Service) invalidateWeatherCacheParse() {
	s.weatherParseMu.Lock()
	s.weatherParseValid = false
	s.weatherParseValue = nil
	s.weatherParseMu.Unlock()
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
	fingerprints := []string{}
	for _, key := range keys {
		value := strings.TrimSpace(cfg.ProviderKeys[key])
		if value == "" {
			continue
		}
		// Keep provider secrets exclusively inside the keyed HMAC helper. The
		// ordinary dashboard-cache digest below contains only non-secret
		// configuration, and HMAC markers are appended afterward.
		fingerprints = append(fingerprints, key+":"+weatherProviderKeyFingerprintGo(value))
	}
	parts := struct {
		Lat       float64  `json:"lat"`
		Lon       float64  `json:"lon"`
		TempUnit  string   `json:"tempUnit"`
		WindUnit  string   `json:"windUnit"`
		Days      int      `json:"days"`
		WxAPI     string   `json:"wxApi"`
		Providers []string `json:"providers"`
	}{cfg.Lat, cfg.Lon, cfg.TempUnit, cfg.WindUnit, cfg.Days, cfg.WxAPI, cfg.Providers}
	b, _ := json.Marshal(parts)
	sum := sha256.Sum256(b)
	base := hex.EncodeToString(sum[:])
	if len(fingerprints) == 0 {
		return base
	}
	return base + "-" + strings.Join(fingerprints, ".")
}
