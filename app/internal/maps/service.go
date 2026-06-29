// Package maps owns Dash-Go's static event-map preview, geocoding, provider,
// cache, and prewarm behavior. It is deliberately local-first: successful
// provider results are bounded on disk and all network failures fall back to
// the existing cached/placeholder flow.
package maps

import (
	"encoding/json"
	stdmaps "maps"
	"net/http"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// ServiceConfig contains the narrow core handles Maps needs. The package never
// imports package main; core startup supplies the paths and callbacks.
type ServiceConfig struct {
	CacheDir               string
	ConfigDir              string
	LogDir                 string
	LoadSettings           func() map[string]any
	InvalidateSystemStatus func()
}

// Service owns map cache status state and the paths/configuration used by map
// rendering, geocoding, provider selection, and cache prewarming.
type Service struct {
	cacheDir  string
	configDir string
	logDir    string

	loadSettingsFn           func() map[string]any
	invalidateSystemStatusFn func()

	mapStatusMu    sync.Mutex
	mapStatusCache map[string]any
	mapStatusAt    time.Time
}

func New(cfg ServiceConfig) *Service {
	return &Service{
		cacheDir:                 cfg.CacheDir,
		configDir:                cfg.ConfigDir,
		logDir:                   cfg.LogDir,
		loadSettingsFn:           cfg.LoadSettings,
		invalidateSystemStatusFn: cfg.InvalidateSystemStatus,
	}
}

func (s *Service) loadSettings() map[string]any {
	if s == nil || s.loadSettingsFn == nil {
		return map[string]any{}
	}
	if values := s.loadSettingsFn(); values != nil {
		return values
	}
	return map[string]any{}
}

func (s *Service) invalidateSystemStatus() {
	if s != nil && s.invalidateSystemStatusFn != nil {
		s.invalidateSystemStatusFn()
	}
}

func (s *Service) readJSONDefault(path string, def any) any {
	b, err := os.ReadFile(path)
	if err != nil {
		return def
	}
	var value any
	if err := json.Unmarshal(b, &value); err != nil {
		return def
	}
	return value
}

func (s *Service) err(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": message})
}

func copyStatusMap(value map[string]any) map[string]any {
	copy := make(map[string]any, len(value))
	stdmaps.Copy(copy, value)
	return copy
}

func compareInts(left, right int) int {
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}

func compareInt64s(left, right int64) int {
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}

func clamp(value, lower, upper int) int {
	return min(max(value, lower), upper)
}

func strOr(value any, def string) string {
	if text := jsonutil.TextValue(value); text != "" {
		return text
	}
	return def
}

func defaultString(value, def string) string {
	if value == "" {
		return def
	}
	return value
}

var (
	reMapCacheExtension  = regexp.MustCompile(`(?i)\.(svg|png|jpg|jpeg|webp)$`)
	reMapCacheCoordinate = regexp.MustCompile(`^(?:standard|hybrid)_z\d+_([pm]\d+\.\d+)_([pm]\d+\.\d+)$`)
	reWhitespace         = regexp.MustCompile(`\s+`)
	reUSPostal           = regexp.MustCompile(`(?i)\b[A-Z]{2}\s+\d{5}(?:-\d{4})?\b`)
	reUSCountry          = regexp.MustCompile(`(?i)\b(United States|USA|US)\b`)
	reStreetNumber       = regexp.MustCompile(`^\d{1,7}\s+`)
	reStreetSuffix       = regexp.MustCompile(`(?i)\b(road|rd\.?|street|st\.?|avenue|ave\.?|drive|dr\.?|lane|ln\.?|court|ct\.?|boulevard|blvd\.?|highway|hwy\.?|parkway|pkwy\.?)\b`)
	reUSCountrySuffix    = regexp.MustCompile(`(?i),?\s*(United States of America|United States|USA|US)\s*$`)
	mapProviderNameSafe  = regexp.MustCompile(`[^a-z0-9._-]+`)
)
