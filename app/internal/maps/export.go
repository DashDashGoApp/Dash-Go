package maps

import "net/http"

// ImageDir and related path accessors are exposed to the core facade and
// focused integration tests; all path composition remains owned by Maps.
func (s *Service) ImageDir() string     { return s.mapImageDir() }
func (s *Service) TileDir() string      { return s.mapTileDir() }
func (s *Service) CacheFile() string    { return s.mapCacheFile() }
func (s *Service) ProviderFile() string { return s.mapProviderFile() }

func (s *Service) EventLookup(query string) map[string]any { return s.eventMapLookup(query) }
func (s *Service) HandleImage(w http.ResponseWriter, r *http.Request) {
	s.handleMapImage(w, r)
}
func (s *Service) CacheStatus() map[string]any { return s.mapCacheStatus() }
func (s *Service) InvalidateStatusCache()      { s.invalidateMapStatusCache() }
func (s *Service) ClearCache(clearGeocodes, clearProvider, clearTiles bool) map[string]any {
	return s.clearMapCache(clearGeocodes, clearProvider, clearTiles)
}
func (s *Service) StartPrewarm(body map[string]any) map[string]any { return s.startMapPrewarm(body) }
func (s *Service) RunPrewarmCLI(args []string) int                 { return s.runMapPrewarmCLI(args) }
func (s *Service) PrewarmEventMaps(limit int) map[string]any       { return s.prewarmEventMaps(limit) }
func (s *Service) FetchImage(lat, lon float64, zoom int, style string, force bool) (string, string) {
	return s.fetchMapImage(lat, lon, zoom, style, force)
}

// Float remains exported for core call sites that historically shared the
// Maps decoded-number helper. It preserves the prior supported scalar forms.
func Float(value any) float64 { return anyFloat(value) }

// TileBounds and TileCacheBase expose pure renderer helpers for focused core
// integration tests without leaking provider/service state.
func TileBounds(lat, lon float64, zoom, width, height int) (int, float64, float64, int, int, int, int, int) {
	return tileBounds(lat, lon, zoom, width, height)
}
func TileCacheBase(providerName string, z, x, y int, layer string) string {
	return tileCacheBase(providerName, z, x, y, layer)
}

func (s *Service) Geocode(query string) map[string]any { return s.geocode(query) }
func (s *Service) CleanImageCache() map[string]any     { return s.cleanMapImageCache() }
func (s *Service) CleanTileCache() map[string]any      { return s.cleanMapTileCache() }
