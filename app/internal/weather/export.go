package weather

import (
	"context"
	"net/http"
	"time"
)

// RadarConfig is the safe, key-free Radar configuration shape used internally
// by the service. Secrets remain in the env file and are never exposed here.
type RadarConfig = radarConfig

const (
	JSONResponseLimit             = weatherJSONResponseLimit
	RefreshHardMinimumMinutes     = weatherRefreshHardMinimumMinutes
	RefreshAPIKeyMinimumMinutes   = weatherRefreshAPIKeyMinimumMinutes
	RefreshLowQuotaMinimumMinutes = weatherRefreshLowQuotaMinimumMinutes
)

func (s *Service) Payload() any                                      { return s.weatherPayload() }
func (s *Service) Fetch(ctx context.Context) (map[string]any, error) { return s.fetchGoWeather(ctx) }

func (s *Service) ReadCache(path, cacheKey string) (any, bool) {
	return s.readWeatherCache(path, cacheKey)
}

func (s *Service) CacheTTL() time.Duration            { return s.weatherCacheTTL() }
func (s *Service) ProviderCachePath(id string) string { return s.weatherProviderCachePathGo(id) }
func (s *Service) ReadProviderCache(id, cacheKey string, allowStale bool) (map[string]any, int64, bool) {
	return s.readWeatherProviderCacheGo(id, cacheKey, allowStale)
}
func (s *Service) WriteProviderCache(id, cacheKey string, source map[string]any) error {
	return s.writeWeatherProviderCacheGo(id, cacheKey, source)
}
func (s *Service) ProviderCooldown(id string) (time.Time, string, bool) {
	return s.weatherProviderCooldownGo(id)
}

func (s *Service) RefreshPolicyForSettings(values map[string]any) map[string]any {
	return s.weatherRefreshPolicyForSettings(values)
}
func (s *Service) RefreshMinutes() int { return s.weatherRefreshMinutes() }

func (s *Service) RadarStatus() map[string]any { return s.radarStatus() }

func (s *Service) RadarTileURL(provider string, z, x, y int) (string, error) {
	return s.radarTileURL(provider, z, x, y)
}
func (s *Service) HandleRadarTile(w http.ResponseWriter, r *http.Request) { s.handleRadarTile(w, r) }

func (s *Service) GenerateMoonCalendar(force bool) map[string]any {
	return s.generateMoonCalendar(force)
}
func (s *Service) MoonCalendarStatus() map[string]any         { return s.moonCalendarStatus() }
func (s *Service) GenerateStaticSkyCalendars() map[string]any { return s.generateStaticSkyCalendars() }
func (s *Service) WriteSkyStatus(meta map[string]any)         { s.writeSkyStatus(meta) }

func PayloadHasLocalDay(payload any, now time.Time) bool {
	return weatherPayloadHasLocalDayGo(payload, now)
}
func MarkCache(payload map[string]any, cfg Config, hit bool, reason string) {
	weatherMarkCacheGo(payload, cfg, hit, reason)
}
func CacheKey(cfg Config) string                       { return weatherCacheKeyGo(cfg) }
func ProviderCacheKey(id string, cfg Config) string    { return weatherProviderCacheKeyGo(id, cfg) }
func ProviderFetchConfig(id string, cfg Config) Config { return weatherProviderFetchConfigGo(id, cfg) }
func ProviderKey(id string, cfg Config) string         { return weatherProviderKeyGo(id, cfg) }

func RefreshMinimumForProviders(values []string) (int, []string) {
	return weatherRefreshMinimumForProviders(values)
}

func FetchJSON(ctx context.Context, rawURL string) (map[string]any, error) {
	return fetchJSONGo(ctx, rawURL)
}
func HealthError(id string, cfg Config, message string, disabled bool) map[string]any {
	return weatherHealthErrorGo(id, cfg, message, disabled)
}
func InsideNWSCoverage(lat, lon float64) bool                     { return insideNWSCoverageGo(lat, lon) }
func RadarFrameMode(provider string) string                       { return radarFrameMode(provider) }
func RadarTileCoordinates(r *http.Request) (int, int, int, error) { return radarTileCoordinates(r) }
func ValidateRadarSettings(values map[string]any) error           { return validateRadarSettings(values) }
