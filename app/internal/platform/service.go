// Package platform owns the local appliance-facing platform surface: terminal
// access, device/health facts, cached system snapshots, diagnostics, and Doctor
// data. It intentionally receives explicit read-only ports from core instead
// of importing the dashboard executable or any sibling domain service.
package platform

import (
	"path/filepath"
	"sync"
	"time"
)

const ControlStatusCacheTTL = 5 * time.Second
const DiagnosticsDirectoryName = ".dashboard-diagnostics"
const DiagnosticsBundleName = "dashboard-diagnostics.zip"

type ServiceConfig struct {
	DashDir           string
	HomeDir           string
	ConfigDir         string
	CalendarDir       string
	CacheDir          string
	LogDir            string
	BinDir            string
	SettingsFile      string
	ConfigLocal       string
	ControlEnv        string
	EventCacheVersion int
	Now               func() time.Time

	LoadSettings          func() map[string]any
	ConfigString          func(string, string) string
	ConfigRevertFile      func() string
	CalendarEntries       func() []map[string]any
	CalendarHealthEnabled func() bool
	MessagesHealthEnabled func() bool
	WeatherRefreshMinutes func() int
	FontStatus            func() map[string]any
	ProfilePayload        func() map[string]any
	CurrentTheme          func() string
	FontStatusPayload     func() any
	MapCacheStatus        func() map[string]any
	ActionHistory         func() any
	SystemUpdateStatus    func() map[string]any
	EventURLToPath        func(string) string
	WeatherDoctorFindings func(map[string]any, time.Time) []string
	PinEnabled            func() bool
}

type Service struct {
	dashDir, homeDir, configDir, calendarDir, cacheDir, logDir, binDir string
	settingsFile, configLocal, controlEnv                              string
	eventCacheVersion                                                  int
	now                                                                func() time.Time
	loadSettings                                                       func() map[string]any
	configString                                                       func(string, string) string
	configRevertFile                                                   func() string
	calendarEntries                                                    func() []map[string]any
	calendarHealthEnabled                                              func() bool
	messagesHealthEnabled                                              func() bool
	weatherRefreshMinutes                                              func() int
	fontStatus                                                         func() map[string]any
	profilePayload                                                     func() map[string]any
	currentTheme                                                       func() string
	fontStatusPayload                                                  func() any
	mapCacheStatus                                                     func() map[string]any
	actionHistory                                                      func() any
	systemUpdateStatus                                                 func() map[string]any
	eventURLToPath                                                     func(string) string
	weatherDoctorFindings                                              func(map[string]any, time.Time) []string
	pinEnabled                                                         func() bool

	warningMu   sync.Mutex
	statusMu    sync.Mutex
	statusCache map[string]any
	statusAt    time.Time
}

func New(cfg ServiceConfig) *Service {
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &Service{
		dashDir: cfg.DashDir, homeDir: cfg.HomeDir, configDir: cfg.ConfigDir, calendarDir: cfg.CalendarDir,
		cacheDir: cfg.CacheDir, logDir: cfg.LogDir, binDir: cfg.BinDir, settingsFile: cfg.SettingsFile,
		configLocal: cfg.ConfigLocal, controlEnv: cfg.ControlEnv, eventCacheVersion: cfg.EventCacheVersion, now: now,
		loadSettings: cfg.LoadSettings, configString: cfg.ConfigString, configRevertFile: cfg.ConfigRevertFile,
		calendarEntries: cfg.CalendarEntries, calendarHealthEnabled: cfg.CalendarHealthEnabled,
		messagesHealthEnabled: cfg.MessagesHealthEnabled, weatherRefreshMinutes: cfg.WeatherRefreshMinutes,
		fontStatus: cfg.FontStatus, profilePayload: cfg.ProfilePayload, currentTheme: cfg.CurrentTheme,
		fontStatusPayload: cfg.FontStatusPayload, mapCacheStatus: cfg.MapCacheStatus, actionHistory: cfg.ActionHistory,
		systemUpdateStatus: cfg.SystemUpdateStatus, eventURLToPath: cfg.EventURLToPath,
		weatherDoctorFindings: cfg.WeatherDoctorFindings, pinEnabled: cfg.PinEnabled,
	}
}

func (s *Service) nowTime() time.Time { return s.now() }
func (s *Service) TerminalAccessFile() string {
	return filepath.Join(s.homeDir, ".dashboard-terminal-access.env")
}
func (s *Service) WarningSilencesPath() string {
	return filepath.Join(s.configDir, "health-warning-silences.json")
}
func (s *Service) DiagnosticsDir() string { return filepath.Join(s.logDir, DiagnosticsDirectoryName) }
func (s *Service) DiagnosticsLocationHint() string {
	return filepath.Join("logs", DiagnosticsDirectoryName)
}
func (s *Service) getSettings() map[string]any {
	if s.loadSettings != nil {
		return s.loadSettings()
	}
	return map[string]any{}
}
func (s *Service) getString(key, def string) string {
	if s.configString != nil {
		return s.configString(key, def)
	}
	return def
}
func (s *Service) getCalendarEntries() []map[string]any {
	if s.calendarEntries != nil {
		return s.calendarEntries()
	}
	return nil
}
func (s *Service) isCalendarHealthEnabled() bool {
	if s.calendarHealthEnabled != nil {
		return s.calendarHealthEnabled()
	}
	return len(s.getCalendarEntries()) > 0
}
func (s *Service) isMessagesHealthEnabled() bool {
	if s.messagesHealthEnabled != nil {
		return s.messagesHealthEnabled()
	}
	return false
}
func (s *Service) weatherMinutes() int {
	if s.weatherRefreshMinutes != nil {
		return s.weatherRefreshMinutes()
	}
	return 30
}
func (s *Service) isPinEnabled() bool {
	if s.pinEnabled != nil {
		return s.pinEnabled()
	}
	return false
}
