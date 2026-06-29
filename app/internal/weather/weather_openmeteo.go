package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	req.Header.Set("User-Agent", "Dash-Go/1.3.5-beta.47")
	res, err := weatherHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("Open-Meteo HTTP %d", res.StatusCode)
	}
	var payload map[string]any
	if err := json.NewDecoder(io.LimitReader(res.Body, weatherJSONResponseLimit)).Decode(&payload); err != nil {
		return nil, err
	}
	payload["_source"] = id
	payload["_sourceLabel"] = weatherProviderLabel(id)
	payload["_fetchedAt"] = time.Now().Unix()
	return payload, nil
}
