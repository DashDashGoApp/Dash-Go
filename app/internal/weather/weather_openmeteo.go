package weather

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func fetchOpenMeteoGo(ctx context.Context, id string, cfg Config) (map[string]any, error) {
	base := strings.TrimRight(cfg.WxAPI, "/")
	if base == "" {
		base = "https://api.open-meteo.com"
	}
	u, err := url.Parse(base + "/v1/forecast")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("latitude", trimFloat(cfg.Lat))
	q.Set("longitude", trimFloat(cfg.Lon))
	q.Set("temperature_unit", cfg.TempUnit)
	q.Set("wind_speed_unit", cfg.WindUnit)
	q.Set("precipitation_unit", "mm")
	q.Set("timezone", "auto")
	q.Set("forecast_days", strconv.Itoa(clamp(cfg.Days, 1, 16)))
	q.Set("current", "temperature_2m,apparent_temperature,weather_code,wind_speed_10m,relative_humidity_2m")
	q.Set("daily", "weather_code,temperature_2m_max,temperature_2m_min,apparent_temperature_max,precipitation_sum,precipitation_probability_max,wind_speed_10m_max,uv_index_max,sunrise,sunset")
	q.Set("hourly", "temperature_2m,weather_code,precipitation_probability")
	if id == "openmeteo-custom" {
		if k := weatherProviderKeyGo(id, cfg); k != "" {
			q.Set("apikey", k)
		}
	}
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", weatherOutboundUserAgent)
	client := weatherHTTPClient
	if id == "openmeteo-custom" {
		client, err = customWeatherHTTPClient(u, cfg.AllowPrivateWxAPI)
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
	var payload map[string]any
	if len(body) > 0 {
		_ = json.Unmarshal(body, &payload)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, &weatherHTTPError{StatusCode: res.StatusCode, Message: firstErrGo(payload), RetryAfter: parseRetryAfter(res.Header.Get("Retry-After"), time.Now())}
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	payload["_source"] = id
	payload["_sourceLabel"] = weatherProviderLabel(id)
	payload["_fetchedAt"] = time.Now().Unix()
	trimOpenMeteoHourlyGo(payload)
	return payload, nil
}
