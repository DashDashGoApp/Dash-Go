package main

import (
	"net/http"

	mapspkg "github.com/DashDashGoApp/Dash-Go/app/internal/maps"
)

// Maps now live in internal/maps. Core retains a deliberately narrow facade so
// route, CLI, diagnostics, and integration call sites do not need to know the
// package wiring while this structural refactor is in progress.
func (a *app) mapsService() *mapspkg.Service {
	a.mapsInitMu.Lock()
	defer a.mapsInitMu.Unlock()
	if a.maps == nil {
		a.maps = mapspkg.New(mapspkg.ServiceConfig{
			CacheDir:               a.cacheDir,
			ConfigDir:              a.configDir,
			LogDir:                 a.logDir,
			LoadSettings:           a.loadSettings,
			InvalidateSystemStatus: a.invalidateSystemStatus,
		})
	}
	return a.maps
}

func (a *app) mapImageDir() string  { return a.mapsService().ImageDir() }
func (a *app) mapTileDir() string   { return a.mapsService().TileDir() }
func (a *app) mapCacheFile() string { return a.mapsService().CacheFile() }

func (a *app) eventMapLookup(query string) map[string]any { return a.mapsService().EventLookup(query) }
func (a *app) geocode(query string) map[string]any        { return a.mapsService().Geocode(query) }
func (a *app) handleMapImage(w http.ResponseWriter, r *http.Request) {
	a.mapsService().HandleImage(w, r)
}
func (a *app) mapCacheStatus() map[string]any { return a.mapsService().CacheStatus() }
func (a *app) invalidateMapStatusCache()      { a.mapsService().InvalidateStatusCache() }
func (a *app) clearMapCache(clearGeocodes, clearProvider, clearTiles bool) map[string]any {
	return a.mapsService().ClearCache(clearGeocodes, clearProvider, clearTiles)
}
func (a *app) cleanMapImageCache() map[string]any { return a.mapsService().CleanImageCache() }
func (a *app) cleanMapTileCache() map[string]any  { return a.mapsService().CleanTileCache() }
func (a *app) startMapPrewarm(body map[string]any) map[string]any {
	return a.mapsService().StartPrewarm(body)
}
func (a *app) runMapPrewarmCLI(args []string) int { return a.mapsService().RunPrewarmCLI(args) }
func (a *app) prewarmEventMaps(limit int) map[string]any {
	return a.mapsService().PrewarmEventMaps(limit)
}
func (a *app) fetchMapImage(lat, lon float64, zoom int, style string, force bool) (string, string) {
	return a.mapsService().FetchImage(lat, lon, zoom, style, force)
}

// anyFloat is retained in core because calendar, doctor, and request handlers
// already share this decoded-number behavior. New map code uses maps.Float.
func anyFloat(value any) float64 { return mapspkg.Float(value) }

// Root test and integration seams preserve the former unexported helper names
// while their implementation now lives in the Maps bounded context.
func tileBounds(lat, lon float64, zoom, width, height int) (int, float64, float64, int, int, int, int, int) {
	return mapspkg.TileBounds(lat, lon, zoom, width, height)
}
func tileCacheBase(providerName string, z, x, y int, layer string) string {
	return mapspkg.TileCacheBase(providerName, z, x, y, layer)
}
