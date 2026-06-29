// Package weather owns Dash-Go's bounded weather, Radar, and sky-calendar
// domain. It receives only immutable paths plus narrow callbacks for settings
// and shared provider backoff; it never imports the executable package.
package weather

import (
	"encoding/json"
	"net/http"
	"os"
	"sync"
	"time"
)

// ServiceConfig supplies the minimal core-owned handles used by the weather
// domain. Callbacks are deliberately consumer-shaped so Weather does not need
// to import Settings, core HTTP wiring, or provider-backoff implementation.
type ServiceConfig struct {
	Dash        string
	Home        string
	CacheDir    string
	CalendarDir string
	ConfigLocal string

	LoadSettings           func() map[string]any
	ProfilePayload         func() map[string]any
	ProfileBaseForSettings func(map[string]any) string

	ProviderBackoffActive  func(string) (time.Time, string, int, bool)
	NoteProviderBackoff    func(string, error)
	ClearProviderBackoff   func(string)
	NetworkLikelyAvailable func() bool
}

// Service owns all mutable Weather/Radar state. File caches remain user-owned
// dashboard data at paths provided by core; the in-memory radar request window
// is intentionally local to the extracted service.
type Service struct {
	dash        string
	home        string
	cacheDir    string
	calDir      string
	configLocal string

	loadSettingsFn           func() map[string]any
	profilePayloadFn         func() map[string]any
	profileBaseForSettingsFn func(map[string]any) string
	providerBackoffActiveFn  func(string) (time.Time, string, int, bool)
	noteProviderBackoffFn    func(string, error)
	clearProviderBackoffFn   func(string)
	networkLikelyAvailableFn func() bool

	radarMu           sync.Mutex
	radarRequestTimes map[string][]time.Time
}

func New(cfg ServiceConfig) *Service {
	return &Service{
		dash:                     cfg.Dash,
		home:                     cfg.Home,
		cacheDir:                 cfg.CacheDir,
		calDir:                   cfg.CalendarDir,
		configLocal:              cfg.ConfigLocal,
		loadSettingsFn:           cfg.LoadSettings,
		profilePayloadFn:         cfg.ProfilePayload,
		profileBaseForSettingsFn: cfg.ProfileBaseForSettings,
		providerBackoffActiveFn:  cfg.ProviderBackoffActive,
		noteProviderBackoffFn:    cfg.NoteProviderBackoff,
		clearProviderBackoffFn:   cfg.ClearProviderBackoff,
		networkLikelyAvailableFn: cfg.NetworkLikelyAvailable,
		radarRequestTimes:        map[string][]time.Time{},
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

func (s *Service) profilePayload() map[string]any {
	if s == nil || s.profilePayloadFn == nil {
		return map[string]any{}
	}
	if payload := s.profilePayloadFn(); payload != nil {
		return payload
	}
	return map[string]any{}
}

func (s *Service) profileBaseForSettings(values map[string]any) string {
	if s == nil || s.profileBaseForSettingsFn == nil {
		return ""
	}
	return s.profileBaseForSettingsFn(values)
}

func (s *Service) providerBackoffActive(name string) (time.Time, string, int, bool) {
	if s == nil || s.providerBackoffActiveFn == nil {
		return time.Time{}, "", 0, false
	}
	return s.providerBackoffActiveFn(name)
}

func (s *Service) noteProviderBackoff(name string, err error) {
	if s != nil && s.noteProviderBackoffFn != nil {
		s.noteProviderBackoffFn(name, err)
	}
}

func (s *Service) clearProviderBackoff(name string) {
	if s != nil && s.clearProviderBackoffFn != nil {
		s.clearProviderBackoffFn(name)
	}
}

func (s *Service) networkLikelyAvailable() bool {
	if s == nil || s.networkLikelyAvailableFn == nil {
		return true
	}
	return s.networkLikelyAvailableFn()
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
