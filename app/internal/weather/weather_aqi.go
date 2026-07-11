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
	payload, err := fetchOpenMeteoAQI(ctx, cfg)
	if err == nil {
		payload["cache"] = map[string]any{"hit": false, "stale": false, "cacheKey": key, "savedAt": time.Now().UnixMilli()}
		_ = fileio.WriteJSON(path, payload)
		return payload
	}
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
