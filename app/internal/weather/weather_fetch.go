package weather

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func (s *Service) fetchGoWeather(parent context.Context) (map[string]any, error) {
	return s.fetchGoWeatherWithConfig(parent, s.Config())
}

func (s *Service) fetchGoWeatherWithConfig(parent context.Context, cfg Config) (map[string]any, error) {
	jobs := []weatherFetchJobGo{}
	selected := normalizeWeatherProviderListGo(cfg.Providers)
	if len(selected) == 0 {
		selected = []string{"openmeteo"}
	}
	for _, id := range selected {
		jobs = append(jobs, weatherFetchJobGo{Index: len(jobs), ID: id})
	}
	results := make([]weatherFetchResultGo, len(jobs))
	if len(jobs) > 0 {
		workers := s.weatherWorkerLimitGo(len(jobs))
		sem := make(chan struct{}, workers)
		var wg sync.WaitGroup
		for _, job := range jobs {
			wg.Go(func() {
				sem <- struct{}{}
				defer func() { <-sem }()
				ctx, cancel := context.WithTimeout(parent, weatherProviderTimeoutGo(job.ID))
				defer cancel()
				results[job.Index] = s.fetchOneWeatherProviderGo(ctx, job, cfg)
			})
		}
		wg.Wait()
	}
	sources := []any{}
	status := []any{}
	for _, r := range results {
		if r.ID == "" {
			continue
		}
		if r.Source != nil {
			sources = append(sources, r.Source)
		}
		if r.Status != nil {
			status = append(status, r.Status)
		}
	}
	if len(sources) == 0 && !stringListContains(selected, "openmeteo") {
		fcfg := weatherProviderFetchConfigGo("openmeteo", cfg)
		ctx, cancel := context.WithTimeout(parent, weatherProviderTimeoutGo("openmeteo"))
		defer cancel()
		if src, err := fetchOpenMeteoGo(ctx, "openmeteo", fcfg); err == nil {
			sources = append(sources, src)
			status = append(status, mapMerge(weatherHealthOKGo("openmeteo", fcfg, src), map[string]any{"fallback": true, "tier": "fallback · free", "reason": "Fallback source returned data"}))
		} else {
			status = append(status, mapMerge(weatherHealthErrorGo("openmeteo", fcfg, err.Error(), false), map[string]any{"tier": "fallback · free"}))
		}
	}
	if len(sources) == 0 {
		return nil, fmt.Errorf("no Go weather provider answered")
	}
	payload := blendWeatherSourcesGo(sources, status, selected, cfg)
	payload["location"] = map[string]any{"lat": cfg.Lat, "lon": cfg.Lon}
	payload["keyStore"] = filepath.Join(s.home, ".dashboard-weather.env")
	payload["keysInServedConfig"] = len(cfg.ProviderKeys) > 0 || strings.TrimSpace(cfg.APIKey) != ""
	return payload, nil
}

func (s *Service) weatherWorkerLimitGo(n int) int {
	profilePayload := s.profilePayload()
	profile := strings.ToLower(jsonutil.StringValue(profilePayload["base"]))
	if profile == "" {
		profile = strings.ToLower(jsonutil.StringValue(profilePayload["current"]))
	}
	limit := 4
	if profile == "lite" || profile == "zero2" || profile == "low" || profile == "low-power" {
		limit = 3
	}
	if n < limit {
		return max(n, 1)
	}
	return limit
}

func weatherProviderTimeoutGo(id string) time.Duration {
	switch strings.ToLower(strings.TrimSpace(id)) {
	case "googleweather", "accuweather", "xweather", "weatherbit", "nws":
		return 7 * time.Second
	default:
		return 6 * time.Second
	}
}

func (s *Service) fetchOneWeatherProviderGo(ctx context.Context, job weatherFetchJobGo, cfg Config) weatherFetchResultGo {
	id := weatherNormalizeProviderIDGo(job.ID)
	pcfg := weatherProviderFetchConfigGo(id, cfg)
	result := weatherFetchResultGo{Index: job.Index, ID: id}
	requestedDays := cfg.Days
	effectiveDays := pcfg.Days
	cacheKey := weatherProviderCacheKeyGo(id, pcfg)
	enrich := func(m map[string]any) map[string]any {
		m["requestedDays"] = requestedDays
		m["effectiveDays"] = effectiveDays
		m["timeoutSeconds"] = int(weatherProviderTimeoutGo(id).Seconds())
		m["providerCacheKey"] = cacheKey
		return m
	}

	fetchLive := func() (map[string]any, error) {
		switch id {
		case "openmeteo", "openmeteo-custom":
			return fetchOpenMeteoGo(ctx, id, pcfg)
		case "nws":
			return fetchNWSGo(ctx, pcfg)
		case "weatherapi", "openweather", "googleweather", "tomorrow", "visualcrossing", "weatherbit", "pirateweather", "accuweather", "xweather":
			return fetchKeyedWeatherGo(ctx, id, pcfg)
		default:
			return nil, fmt.Errorf("unknown weather provider")
		}
	}

	if id == "" {
		result.Status = enrich(weatherHealthErrorGo(id, pcfg, "unknown weather provider", true))
		return result
	}
	if weatherProviderNeedsKey(id) && weatherProviderKeyGo(id, pcfg) == "" {
		result.Status = enrich(weatherHealthErrorGo(id, pcfg, "Missing API key; source ignored until a key is saved", true))
		return result
	}

	if until, reason, _, ok := s.providerBackoffActive("weather-" + id); ok {
		if src, age, cacheOK := s.readWeatherProviderCacheGo(id, cacheKey, true); cacheOK {
			weatherMarkProviderStaleGo(src, age, "provider backoff: "+reason)
			result.Source = src
			result.Status = enrich(mapMerge(weatherHealthOKGo(id, pcfg, src), map[string]any{"freshness": "stale", "stale": true, "cacheHit": true, "status": "backoff", "reason": "Using stale provider cache during retry backoff", "backoffUntil": until.Unix(), "lastError": reason}))
			return result
		}
		result.Status = enrich(mapMerge(weatherHealthErrorGo(id, pcfg, "provider retry backoff: "+reason, false), map[string]any{"status": "backoff", "backoffUntil": until.Unix()}))
		return result
	}

	if until, reason, ok := s.weatherProviderCooldownGo(id); ok {
		if src, age, cacheOK := s.readWeatherProviderCacheGo(id, cacheKey, true); cacheOK {
			weatherMarkProviderStaleGo(src, age, "provider in cooldown: "+reason)
			result.Source = src
			result.Status = enrich(mapMerge(weatherHealthOKGo(id, pcfg, src), map[string]any{"freshness": "stale", "stale": true, "cacheHit": true, "status": "stale_cache", "reason": "Using stale provider cache during cooldown", "cooldownUntil": until.Unix(), "lastError": reason}))
			return result
		}
		result.Status = enrich(mapMerge(weatherHealthErrorGo(id, pcfg, "provider in cooldown: "+reason, false), map[string]any{"status": "cooldown", "cooldownUntil": until.Unix()}))
		return result
	}

	if src, age, ok := s.readWeatherProviderCacheGo(id, cacheKey, false); ok {
		weatherMarkProviderCacheHitGo(src, age)
		result.Source = src
		result.Status = enrich(mapMerge(weatherHealthOKGo(id, pcfg, src), map[string]any{"cacheHit": true, "freshness": "fresh", "reason": "Using fresh provider cache"}))
		return result
	}

	if !s.networkLikelyAvailable() {
		if src, age, ok := s.readWeatherProviderCacheGo(id, cacheKey, true); ok {
			weatherMarkProviderStaleGo(src, age, "network unavailable")
			result.Source = src
			result.Status = enrich(mapMerge(weatherHealthOKGo(id, pcfg, src), map[string]any{"freshness": "stale", "stale": true, "cacheHit": true, "status": "network_unavailable", "reason": "Using last-good provider cache while network is unavailable"}))
			return result
		}
		result.Status = enrich(mapMerge(weatherHealthErrorGo(id, pcfg, "network unavailable", false), map[string]any{"status": "network_unavailable"}))
		return result
	}

	src, err := fetchLive()
	if err == nil {
		_ = s.writeWeatherProviderCacheGo(id, cacheKey, src)
		s.clearWeatherProviderCooldownGo(id)
		s.clearProviderBackoff("weather-" + id)
		result.Source = src
		result.Status = enrich(weatherHealthOKGo(id, pcfg, src))
		return result
	}

	s.noteWeatherProviderErrorGo(id, err)
	s.noteProviderBackoff("weather-"+id, err)
	if src, age, ok := s.readWeatherProviderCacheGo(id, cacheKey, true); ok {
		weatherMarkProviderStaleGo(src, age, err.Error())
		result.Source = src
		result.Status = enrich(mapMerge(weatherHealthOKGo(id, pcfg, src), map[string]any{"freshness": "stale", "stale": true, "cacheHit": true, "status": "stale_cache", "reason": "Using stale provider cache after live fetch failed", "lastError": err.Error()}))
		return result
	}
	result.Status = enrich(weatherHealthErrorGo(id, pcfg, err.Error(), false))
	return result
}
