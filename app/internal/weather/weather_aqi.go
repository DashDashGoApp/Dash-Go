package weather

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

const weatherAQICacheTTL = 30 * time.Minute

// weatherAQIFailureCooldown bounds how often a failing air-quality endpoint is
// re-attempted. The success cache alone left a dead endpoint being called on every
// weather refresh, and once per browser retry on top of that, so a short failure
// marker is persisted and honoured before any live attempt. It stays short on
// purpose: the first refresh after it expires probes again.
const weatherAQIFailureCooldown = 10 * time.Minute

type weatherAQIFailureState struct {
	CacheKey string `json:"cacheKey"`
	FailedAt int64  `json:"failedAt"`
}

func weatherAQIFailurePath(cacheDir string) string {
	return filepath.Join(cacheDir, "weather-aqi-failure.json")
}

// weatherAQIFailureUntil reports when the cooldown for this cache key ends, or the
// zero time when it is not cooling down (never failed, different key, or expired).
func (s *Service) weatherAQIFailureUntil(key string) time.Time {
	raw := anyMap(s.readJSONDefault(weatherAQIFailurePath(s.cacheDir), nil))
	if len(raw) == 0 || jsonutil.StringValue(raw["cacheKey"]) != key {
		return time.Time{}
	}
	failedAt := int64(jsonutil.Int(raw["failedAt"], 0))
	if failedAt == 0 {
		return time.Time{}
	}
	until := time.Unix(failedAt, 0).Add(weatherAQIFailureCooldown)
	if !time.Now().Before(until) {
		return time.Time{}
	}
	return until
}

func (s *Service) weatherAQINoteFailure(key string) {
	_ = fileio.WriteJSON(weatherAQIFailurePath(s.cacheDir), weatherAQIFailureState{CacheKey: key, FailedAt: time.Now().Unix()})
}

// weatherAQINoteSuccess clears the marker, so a recovered endpoint is cached
// normally instead of being held back by an old failure.
func (s *Service) weatherAQINoteSuccess(key string) {
	path := weatherAQIFailurePath(s.cacheDir)
	if fileio.Exists(path) {
		_ = fileio.RemoveDurable(path)
	}
}

func weatherAQICacheKey(cfg Config) string {
	// Display units are intentionally absent: AQI is canonical and survives a
	// Fahrenheit/Celsius toggle without another provider request.
	body, _ := json.Marshal(struct {
		Lat   float64 `json:"lat"`
		Lon   float64 `json:"lon"`
		AQAPI string  `json:"aqApi"`
	}{cfg.Lat, cfg.Lon, strings.TrimRight(cfg.AQAPI, "/")})
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func (s *Service) aqiPayload(ctx context.Context) map[string]any {
	s.aqiMu.Lock()
	defer s.aqiMu.Unlock()
	cfg := s.Config()
	path := filepath.Join(s.cacheDir, "weather-aqi-cache.json")
	key := weatherAQICacheKey(cfg)
	if cached, ok := s.readAQICache(path, key, false); ok {
		return cached
	}
	// A recent failure is not re-attempted on every refresh. A stale cache is still
	// served when it exists, so the household keeps a number while the endpoint is
	// down rather than watching the pill flap.
	if until := s.weatherAQIFailureUntil(key); !until.IsZero() {
		if cached, ok := s.readAQICache(path, key, true); ok {
			cache := anyMap(cached["cache"])
			cache["hit"] = true
			cache["stale"] = true
			cache["cooldownUntil"] = until.Unix()
			cache["reason"] = "Air quality is cooling down after a recent failure"
			cached["cache"] = cache
			return cached
		}
		return map[string]any{"current": nil, "error": "Air quality is cooling down after a recent failure", "cache": map[string]any{"hit": false, "stale": false, "cooldownUntil": until.Unix(), "cacheKey": key}}
	}
	payload, err := fetchOpenMeteoAQI(ctx, cfg)
	if err == nil {
		s.weatherAQINoteSuccess(key)
		payload["cache"] = map[string]any{"hit": false, "stale": false, "cacheKey": key, "savedAt": time.Now().UnixMilli()}
		_ = fileio.WriteJSON(path, payload)
		return payload
	}
	s.weatherAQINoteFailure(key)
	if cached, ok := s.readAQICache(path, key, true); ok {
		cache := anyMap(cached["cache"])
		cache["hit"] = true
		cache["stale"] = true
		cache["reason"] = err.Error()
		cached["cache"] = cache
		return cached
	}
	return map[string]any{"current": nil, "error": err.Error(), "cache": map[string]any{"hit": false, "stale": false, "cacheKey": key}}
}

func (s *Service) readAQICache(path, key string, allowStale bool) (map[string]any, bool) {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || (!allowStale && time.Since(info.ModTime()) > weatherAQICacheTTL) {
		return nil, false
	}
	payload := anyMap(s.readJSONDefault(path, nil))
	if len(payload) == 0 || jsonutil.StringValue(anyMap(payload["cache"])["cacheKey"]) != key {
		return nil, false
	}
	cache := anyMap(payload["cache"])
	cache["hit"] = true
	cache["ageSeconds"] = max(int(time.Since(info.ModTime()).Seconds()), 0)
	payload["cache"] = cache
	return payload, true
}

func fetchOpenMeteoAQI(ctx context.Context, cfg Config) (map[string]any, error) {
	base := strings.TrimRight(strings.TrimSpace(cfg.AQAPI), "/")
	if base == "" {
		base = "https://air-quality-api.open-meteo.com"
	}
	target, err := url.Parse(base + "/v1/air-quality")
	if err != nil {
		return nil, err
	}
	q := target.Query()
	q.Set("latitude", trimFloat(cfg.Lat))
	q.Set("longitude", trimFloat(cfg.Lon))
	q.Set("current", "us_aqi")
	q.Set("timezone", "auto")
	target.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", weatherOutboundUserAgent)
	client := weatherHTTPClient
	if !strings.EqualFold(target.Hostname(), "air-quality-api.open-meteo.com") {
		client, err = customWeatherHTTPClient(target, cfg.AllowPrivateWxAPI)
		if err != nil {
			return nil, err
		}
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := readWeatherResponse(ctx, res)
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	_ = json.Unmarshal(body, &doc)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, &weatherHTTPError{StatusCode: res.StatusCode, Message: firstErrGo(doc), RetryAfter: parseRetryAfter(res.Header.Get("Retry-After"), time.Now())}
	}
	current := anyMap(doc["current"])
	value, ok := toFloatGo(current["us_aqi"])
	if !ok || value < 0 || value > 500 {
		return nil, fmt.Errorf("air-quality provider returned no usable US AQI")
	}
	return map[string]any{
		"current":   map[string]any{"us_aqi": value},
		"source":    "openmeteo-aqi",
		"fetchedAt": time.Now().Unix(),
	}, nil
}
