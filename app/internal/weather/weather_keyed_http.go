package weather

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

const weatherOutboundUserAgent = "Dash-Go (+local-kiosk)"

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

type weatherRequestMetrics struct {
	mu            sync.Mutex
	NetworkCalls  int
	ResponseBytes int64
	LastStatus    int
}

type weatherRequestMetricsKey struct{}

func withWeatherRequestMetrics(ctx context.Context) (context.Context, *weatherRequestMetrics) {
	metrics := &weatherRequestMetrics{}
	return context.WithValue(ctx, weatherRequestMetricsKey{}, metrics), metrics
}

func recordWeatherResponse(ctx context.Context, status int, size int64) {
	metrics, _ := ctx.Value(weatherRequestMetricsKey{}).(*weatherRequestMetrics)
	if metrics == nil {
		return
	}
	metrics.mu.Lock()
	metrics.NetworkCalls++
	metrics.ResponseBytes += size
	metrics.LastStatus = status
	metrics.mu.Unlock()
}

func (m *weatherRequestMetrics) Snapshot() (int, int64, int) {
	if m == nil {
		return 0, 0, 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.NetworkCalls, m.ResponseBytes, m.LastStatus
}

type weatherHTTPError struct {
	StatusCode int
	Message    string
	RetryAfter time.Duration
}

func (e *weatherHTTPError) Error() string {
	if e == nil {
		return "weather provider error"
	}
	message := strings.TrimSpace(e.Message)
	if message == "" {
		message = "provider error"
	}
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, message)
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(value); err == nil && when.After(now) {
		return when.Sub(now)
	}
	return 0
}

func readWeatherResponse(ctx context.Context, res *http.Response) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(res.Body, weatherJSONResponseLimit+1))
	if err != nil {
		return nil, err
	}
	recordWeatherResponse(ctx, res.StatusCode, int64(len(body)))
	if len(body) > weatherJSONResponseLimit {
		return nil, fmt.Errorf("weather response exceeded %d bytes", weatherJSONResponseLimit)
	}
	return body, nil
}

func fetchJSONGo(ctx context.Context, rawURL string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", weatherOutboundUserAgent)
	req.Header.Set("Accept", "application/json")
	res, err := weatherHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := readWeatherResponse(ctx, res)
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if len(bytes.TrimSpace(body)) != 0 {
		_ = json.Unmarshal(body, &doc)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, &weatherHTTPError{StatusCode: res.StatusCode, Message: firstErrGo(doc), RetryAfter: parseRetryAfter(res.Header.Get("Retry-After"), time.Now())}
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func fetchJSONAnyGo(ctx context.Context, rawURL string) (any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", weatherOutboundUserAgent)
	req.Header.Set("Accept", "application/json")
	res, err := weatherHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := readWeatherResponse(ctx, res)
	if err != nil {
		return nil, err
	}
	var doc any
	if len(bytes.TrimSpace(body)) != 0 {
		_ = json.Unmarshal(body, &doc)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		message := "provider error"
		if m, ok := doc.(map[string]any); ok {
			message = firstErrGo(m)
		}
		return nil, &weatherHTTPError{StatusCode: res.StatusCode, Message: message, RetryAfter: parseRetryAfter(res.Header.Get("Retry-After"), time.Now())}
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
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
