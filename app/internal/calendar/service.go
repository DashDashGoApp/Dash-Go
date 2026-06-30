package calendar

import (
	"net/http"
	"sync"
	"time"
)

// ServiceConfig supplies immutable paths and deliberately narrow core ports.
// Calendar does not receive the core application object or a sibling service. Household app settings,
// weather-generated sky feeds, Settings' ISO-week preference, and event-cache
// work remain consumer-owned callbacks.
type ServiceConfig struct {
	DashDir                string
	HomeDir                string
	CalendarDir            string
	CacheDir               string
	LogDir                 string
	ConfigLocal            string
	CelebrationsFile       string
	HouseholdSchedulesFile string
	Now                    func() time.Time

	OutputEnabled func(owner string) bool
	AppKnown      func(owner string) bool
	SetAppOutput  func(owner string, enabled bool) (map[string]any, error)

	GenerateMoon         func(force bool) map[string]any
	GenerateSky          func() map[string]any
	EnableISOWeekNumbers func()

	RefreshCacheSync  func() error
	RefreshCacheAsync func()
	IndexWarning      func(owner string, err error)
	HTTPClient        func() *http.Client
}

// Service owns the calendar-management lock, local manifest/trash persistence,
// generated default feeds, and source visibility decisions.
type Service struct {
	dashDir                string
	homeDir                string
	calendarDir            string
	cacheDir               string
	logDir                 string
	configLocal            string
	celebrationsFile       string
	householdSchedulesFile string
	nowFn                  func() time.Time

	outputEnabled func(owner string) bool
	appKnown      func(owner string) bool
	setAppOutput  func(owner string, enabled bool) (map[string]any, error)

	generateMoon  func(force bool) map[string]any
	generateSky   func() map[string]any
	enableISOWeek func()

	refreshCacheSync  func() error
	refreshCacheAsync func()
	indexWarning      func(owner string, err error)
	httpClient        func() *http.Client
	mu                sync.Mutex
}

func New(cfg ServiceConfig) *Service {
	return &Service{
		dashDir: cfg.DashDir, homeDir: cfg.HomeDir, calendarDir: cfg.CalendarDir,
		cacheDir: cfg.CacheDir, logDir: cfg.LogDir, configLocal: cfg.ConfigLocal,
		celebrationsFile: cfg.CelebrationsFile, householdSchedulesFile: cfg.HouseholdSchedulesFile, nowFn: cfg.Now,
		outputEnabled: cfg.OutputEnabled, appKnown: cfg.AppKnown, setAppOutput: cfg.SetAppOutput,
		generateMoon: cfg.GenerateMoon, generateSky: cfg.GenerateSky,
		enableISOWeek: cfg.EnableISOWeekNumbers, refreshCacheSync: cfg.RefreshCacheSync,
		refreshCacheAsync: cfg.RefreshCacheAsync, indexWarning: cfg.IndexWarning, httpClient: cfg.HTTPClient,
	}
}

func (s *Service) now() time.Time {
	if s != nil && s.nowFn != nil {
		return s.nowFn()
	}
	return time.Now()
}

func (s *Service) CalendarDir() string {
	if s == nil {
		return ""
	}
	return s.calendarDir
}

func (s *Service) WithLock(fn func() error) error {
	if s == nil {
		if fn != nil {
			return fn()
		}
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if fn != nil {
		return fn()
	}
	return nil
}
