package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

const weatherJSONResponseLimit = 4 << 20

var weatherHTTPClient = &http.Client{Timeout: 20 * time.Second}

func fetchKeyedWeatherGo(ctx context.Context, id string, cfg Config) (map[string]any, error) {
	switch id {
	case "weatherapi":
		return fetchWeatherAPIGo(ctx, cfg)
	case "openweather":
		return fetchOpenWeatherGo(ctx, cfg)
	case "googleweather":
		return fetchGoogleWeatherGo(ctx, cfg)
	case "tomorrow":
		return fetchTomorrowGo(ctx, cfg)
	case "visualcrossing":
		return fetchVisualCrossingGo(ctx, cfg)
	case "weatherbit":
		return fetchWeatherbitGo(ctx, cfg)
	case "pirateweather":
		return fetchPirateWeatherGo(ctx, cfg)
	case "accuweather":
		return fetchAccuWeatherGo(ctx, cfg)
	case "xweather":
		return fetchXWeatherGo(ctx, cfg)
	default:
		return nil, fmt.Errorf("unknown weather provider")
	}
}

func fetchJSONGo(ctx context.Context, rawURL string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Dash-Go/1.3.5-beta.47 (+local kiosk)")
	req.Header.Set("Accept", "application/json")
	res, err := weatherHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	var doc map[string]any
	if err := json.NewDecoder(io.LimitReader(res.Body, weatherJSONResponseLimit)).Decode(&doc); err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", res.StatusCode, firstErrGo(doc))
	}
	return doc, nil
}

func firstErrGo(doc map[string]any) string {
	for _, k := range []string{"error", "errors", "detail", "message"} {
		if v, ok := doc[k]; ok {
			if text := jsonutil.TextValue(v); text != "" {
				return text
			}
		}
	}
	return "provider error"
}

func weatherURLValues(vals map[string]string) string {
	q := url.Values{}
	for k, v := range vals {
		if v != "" {
			q.Set(k, v)
		}
	}
	return q.Encode()
}

func fetchJSONAnyGo(ctx context.Context, rawURL string) (any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Dash-Go/1.3.5-beta.47 (+local kiosk)")
	req.Header.Set("Accept", "application/json")
	res, err := weatherHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	var doc any
	if err := json.NewDecoder(io.LimitReader(res.Body, weatherJSONResponseLimit)).Decode(&doc); err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d", res.StatusCode)
	}
	return doc, nil
}
