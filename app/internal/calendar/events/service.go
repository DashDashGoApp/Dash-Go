package events

import (
	"path/filepath"
	"time"
)

// ServiceConfig supplies immutable runtime paths plus the two calendar-policy
// seams that intentionally remain in core through beta.15. No callback may
// receive or retain a broad application object.
type ServiceConfig struct {
	DashDir        string
	CalendarDir    string
	CacheDir       string
	OutputEnabled  func(url string) bool
	SourceIdentity func(url string) string
	OwnedSource    func(url string) (CalendarSource, bool)
	Now            func() time.Time
}

// Service owns all event-domain behavior. It has no background worker and no
// dependency on core or a sibling domain service.
type Service struct {
	dashDir        string
	calendarDir    string
	cacheDir       string
	outputEnabled  func(string) bool
	sourceIdentity func(string) string
	ownedSource    func(string) (CalendarSource, bool)
	now            func() time.Time
}

func New(cfg ServiceConfig) *Service {
	outputEnabled := cfg.OutputEnabled
	if outputEnabled == nil {
		outputEnabled = func(string) bool { return true }
	}
	sourceIdentity := cfg.SourceIdentity
	if sourceIdentity == nil {
		sourceIdentity = defaultSourceIdentity
	}
	ownedSource := cfg.OwnedSource
	if ownedSource == nil {
		ownedSource = func(string) (CalendarSource, bool) { return CalendarSource{}, false }
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &Service{
		dashDir:        filepath.Clean(cfg.DashDir),
		calendarDir:    filepath.Clean(cfg.CalendarDir),
		cacheDir:       filepath.Clean(cfg.CacheDir),
		outputEnabled:  outputEnabled,
		sourceIdentity: sourceIdentity,
		ownedSource:    ownedSource,
		now:            now,
	}
}

func (s *Service) CalendarDir() string { return s.calendarDir }
func (s *Service) CacheDir() string    { return s.cacheDir }
