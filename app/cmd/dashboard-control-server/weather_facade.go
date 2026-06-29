package main

import (
	"context"
	"net/http"
	"time"

	weatherpkg "github.com/DashDashGoApp/Dash-Go/app/internal/weather"
)

// Weather/Radar/Sky now live in internal/weather. This file is the deliberately
// narrow core adapter that preserves route and CLI entry points while core
// startup still owns paths, settings construction, and provider-backoff files.
type weatherConfig = weatherpkg.Config
type radarConfig = weatherpkg.RadarConfig

const (
	weatherJSONResponseLimit             = weatherpkg.JSONResponseLimit
	weatherRefreshHardMinimumMinutes     = weatherpkg.RefreshHardMinimumMinutes
	weatherRefreshAPIKeyMinimumMinutes   = weatherpkg.RefreshAPIKeyMinimumMinutes
	weatherRefreshLowQuotaMinimumMinutes = weatherpkg.RefreshLowQuotaMinimumMinutes
)

func (a *app) weatherService() *weatherpkg.Service {
	a.weatherInitMu.Lock()
	defer a.weatherInitMu.Unlock()
	if a.weather == nil {
		a.weather = weatherpkg.New(weatherpkg.ServiceConfig{
			Dash:                   a.dash,
			Home:                   a.home,
			CacheDir:               a.cacheDir,
			CalendarDir:            a.calDir,
			ConfigLocal:            a.configLocal,
			LoadSettings:           a.loadSettings,
			ProfilePayload:         a.profilePayload,
			ProfileBaseForSettings: a.profileBaseForSettings,
			ProviderBackoffActive:  a.providerBackoffActive,
			NoteProviderBackoff:    a.noteProviderBackoff,
			ClearProviderBackoff:   a.clearProviderBackoff,
			NetworkLikelyAvailable: networkLikelyAvailable,
		})
	}
	return a.weather
}

func (a *app) weatherConfig() weatherConfig { return a.weatherService().Config() }
func (a *app) weatherPayload() any          { return a.weatherService().Payload() }
func (a *app) fetchGoWeather(ctx context.Context) (map[string]any, error) {
	return a.weatherService().Fetch(ctx)
}

func (a *app) readWeatherCache(path, cacheKey string) (any, bool) {
	return a.weatherService().ReadCache(path, cacheKey)
}

func (a *app) weatherCacheTTL() time.Duration { return a.weatherService().CacheTTL() }
func (a *app) weatherProviderCachePathGo(id string) string {
	return a.weatherService().ProviderCachePath(id)
}
func (a *app) readWeatherProviderCacheGo(id, cacheKey string, allowStale bool) (map[string]any, int64, bool) {
	return a.weatherService().ReadProviderCache(id, cacheKey, allowStale)
}
func (a *app) writeWeatherProviderCacheGo(id, cacheKey string, source map[string]any) error {
	return a.weatherService().WriteProviderCache(id, cacheKey, source)
}
func (a *app) weatherProviderCooldownGo(id string) (time.Time, string, bool) {
	return a.weatherService().ProviderCooldown(id)
}

func (a *app) weatherRefreshPolicyForSettings(values map[string]any) map[string]any {
	return a.weatherService().RefreshPolicyForSettings(values)
}
func (a *app) weatherRefreshMinutes() int { return a.weatherService().RefreshMinutes() }

func (a *app) radarStatus() map[string]any { return a.weatherService().RadarStatus() }

func (a *app) radarTileURL(provider string, z, x, y int) (string, error) {
	return a.weatherService().RadarTileURL(provider, z, x, y)
}
func (a *app) handleRadarTile(w http.ResponseWriter, r *http.Request) {
	a.weatherService().HandleRadarTile(w, r)
}

func (a *app) generateMoonCalendar(force bool) map[string]any {
	return a.weatherService().GenerateMoonCalendar(force)
}
func (a *app) moonCalendarStatus() map[string]any { return a.weatherService().MoonCalendarStatus() }
func (a *app) generateStaticSkyCalendars() map[string]any {
	return a.weatherService().GenerateStaticSkyCalendars()
}
func (a *app) writeSkyStatus(meta map[string]any) { a.weatherService().WriteSkyStatus(meta) }

func weatherPayloadHasLocalDayGo(payload any, now time.Time) bool {
	return weatherpkg.PayloadHasLocalDay(payload, now)
}
func weatherMarkCacheGo(payload map[string]any, cfg weatherConfig, hit bool, reason string) {
	weatherpkg.MarkCache(payload, cfg, hit, reason)
}
func weatherCacheKeyGo(cfg weatherConfig) string { return weatherpkg.CacheKey(cfg) }
func weatherProviderCacheKeyGo(id string, cfg weatherConfig) string {
	return weatherpkg.ProviderCacheKey(id, cfg)
}
func weatherProviderFetchConfigGo(id string, cfg weatherConfig) weatherConfig {
	return weatherpkg.ProviderFetchConfig(id, cfg)
}
func weatherProviderKeyGo(id string, cfg weatherConfig) string {
	return weatherpkg.ProviderKey(id, cfg)
}
func weatherRefreshMinimumForProviders(values []string) (int, []string) {
	return weatherpkg.RefreshMinimumForProviders(values)
}
func fetchJSONGo(ctx context.Context, rawURL string) (map[string]any, error) {
	return weatherpkg.FetchJSON(ctx, rawURL)
}
func weatherHealthErrorGo(id string, cfg weatherConfig, message string, disabled bool) map[string]any {
	return weatherpkg.HealthError(id, cfg, message, disabled)
}
func insideNWSCoverageGo(lat, lon float64) bool { return weatherpkg.InsideNWSCoverage(lat, lon) }
func radarFrameMode(provider string) string     { return weatherpkg.RadarFrameMode(provider) }
func radarTileCoordinates(r *http.Request) (int, int, int, error) {
	return weatherpkg.RadarTileCoordinates(r)
}
func validateRadarSettings(values map[string]any) error {
	return weatherpkg.ValidateRadarSettings(values)
}
