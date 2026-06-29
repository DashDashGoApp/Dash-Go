package main

import (
	"math"
	"path/filepath"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
	platformpkg "github.com/DashDashGoApp/Dash-Go/app/internal/platform"
)

// Stable main-package aliases keep routes, CLI wiring, and integration tests
// unchanged while platform owns the mutable health/status/terminal state.
type healthFact = platformpkg.HealthFact
type healthWarningSilence = platformpkg.WarningSilence
type healthWarningSilenceState = platformpkg.WarningSilenceState

const controlStatusCacheTTL = platformpkg.ControlStatusCacheTTL
const diagnosticsDirectoryName = platformpkg.DiagnosticsDirectoryName
const diagnosticsBundleName = platformpkg.DiagnosticsBundleName

var errTerminalAccessDisabled = platformpkg.ErrTerminalAccessDisabled

func (a *app) platformService() *platformpkg.Service {
	a.platformInitMu.Lock()
	defer a.platformInitMu.Unlock()
	if a.platform == nil {
		a.platform = platformpkg.New(platformpkg.ServiceConfig{
			DashDir: a.dash, HomeDir: a.home, ConfigDir: a.configDir, CalendarDir: a.calDir, CacheDir: a.cacheDir, LogDir: a.logDir, BinDir: a.binDir,
			SettingsFile: a.settingsFile, ConfigLocal: a.configLocal, ControlEnv: a.controlEnvPath(), EventCacheVersion: eventCacheVersion, Now: time.Now,
			LoadSettings: a.loadSettings, ConfigString: a.configString, ConfigRevertFile: a.configRevertFile,
			CalendarEntries: a.platformCalendarEntries, CalendarHealthEnabled: func() bool { return len(a.platformCalendarEntries()) > 0 },
			MessagesHealthEnabled: a.platformMessagesHealthEnabled, WeatherRefreshMinutes: a.weatherRefreshMinutes,
			FontStatus: a.fontStatus, ProfilePayload: func() map[string]any { return a.profilePayloadForSettings(a.loadSettings()) }, CurrentTheme: a.currentTheme,
			FontStatusPayload: func() any { return a.fontStatusPayload() }, MapCacheStatus: a.mapCacheStatus,
			ActionHistory: func() any { return a.actionHistory(25) }, SystemUpdateStatus: a.systemUpdateStatus,
			EventURLToPath: a.eventURLToPath, WeatherDoctorFindings: a.platformWeatherDoctorFindings,
			PinEnabled: func() bool { enabled, _ := a.lockConfig()["enabled"].(bool); return enabled },
		})
	}
	return a.platform
}
func (a *app) platformCalendarEntries() []map[string]any {
	rows := []map[string]any{}
	for _, raw := range jsonutil.List(a.calendars()) {
		rows = append(rows, jsonutil.Map(raw))
	}
	return rows
}
func (a *app) platformMessagesHealthEnabled() bool {
	prefs := a.messagePrefs()
	return len(a.normalizeMessageEnabled(jsonutil.List(prefs["enabled"]))) > 0
}
func (a *app) platformWeatherDoctorFindings(payload map[string]any, now time.Time) []string {
	findings := []string{}
	location := jsonutil.Map(payload["location"])
	cache := jsonutil.Map(payload["cache"])
	if !weatherPayloadHasLocalDayGo(payload, now) {
		findings = append(findings, platformpkg.DoctorDataFinding("WEATHER_CACHE_DAY_STALE", "daily forecast does not include today"))
	}
	cfg := a.weatherConfig()
	if cfg.Lat != 0 || cfg.Lon != 0 {
		cachedLat, cachedLon := anyFloat(location["lat"]), anyFloat(location["lon"])
		if math.Abs(cachedLat-cfg.Lat) > 0.0001 || math.Abs(cachedLon-cfg.Lon) > 0.0001 {
			findings = append(findings, platformpkg.DoctorDataFinding("WEATHER_CACHE_LOCATION_MISMATCH", "cached coordinates differ from current location"))
		}
		expected := weatherCacheKeyGo(cfg)
		if actual := jsonutil.StringValue(cache["cacheKey"]); actual != "" && actual != expected {
			findings = append(findings, platformpkg.DoctorDataFinding("WEATHER_CACHE_KEY_MISMATCH", "cached settings fingerprint differs"))
		}
	}
	return findings
}

func (a *app) terminalAccessFile() string          { return a.platformService().TerminalAccessFile() }
func parseTerminalAccess(data []byte) (bool, bool) { return platformpkg.ParseTerminalAccess(data) }
func (a *app) terminalAccessEnabled() bool         { return a.platformService().TerminalAccessEnabled() }
func (a *app) setTerminalAccessEnabled(enabled bool) error {
	return a.platformService().SetTerminalAccessEnabled(enabled)
}
func (a *app) runTerminalAccessCLI(args []string) int {
	return a.platformService().RunTerminalAccessCLI(args)
}
func (a *app) terminalStatus() map[string]any        { return a.platformService().TerminalStatus() }
func (a *app) openTerminal() (map[string]any, error) { return a.platformService().OpenTerminal() }

func readHealthFile(path string) map[string]any { return platformpkg.ReadHealthFile(path) }

func (a *app) suppressDataStaleness(now time.Time, settings map[string]any, refresh int) bool {
	return a.platformService().SuppressDataStaleness(now, settings, refresh)
}

func (a *app) deviceHealth() map[string]any { return a.platformService().DeviceHealth() }

func storageKernelWarningPredatesBoot(raw map[string]any, fact healthFact, boot int64) bool {
	return platformpkg.StorageKernelWarningPredatesBoot(raw, fact, boot)
}

func (a *app) healthWarningSilencesPath() string { return a.platformService().WarningSilencesPath() }

func (a *app) healthWarningSilences(now time.Time) map[string]any {
	return a.platformService().WarningSilences(now)
}
func (a *app) setHealthWarningSilence(key string, minutes int, now time.Time) (map[string]any, error) {
	return a.platformService().SetWarningSilence(key, minutes, now)
}

func copyStatusMap(value map[string]any) map[string]any {
	return platformpkg.CopyStatusMapForFacade(value)
}
func (a *app) cacheStatus() map[string]any { return a.platformService().CacheStatus() }
func (a *app) invalidateSystemStatus()     { a.platformService().InvalidateSystemStatus() }

func (a *app) systemStatus() map[string]any { return a.platformService().SystemStatus() }

func memoryStatus() map[string]any { return platformpkg.MemoryStatus() }

func parseMemoryInfoMB(contents string) (int, int) { return platformpkg.ParseMemoryInfoMB(contents) }

func redactText(text string) string { return platformpkg.RedactText(text) }

func (a *app) diagnosticsDir() string          { return a.platformService().DiagnosticsDir() }
func (a *app) diagnosticsLocationHint() string { return a.platformService().DiagnosticsLocationHint() }
func (a *app) buildDiagnostics() (map[string]any, error) {
	return a.platformService().BuildDiagnostics()
}
func (a *app) buildDiagnosticsWithHealth(health map[string]any) (map[string]any, error) {
	return a.platformService().BuildDiagnosticsWithHealth(health)
}
func (a *app) parseDoctorReport(output string, rc int, dur time.Duration) map[string]any {
	return platformpkg.ParseDoctorReport(output, rc, dur)
}

func (a *app) runDoctorSummaryMode(repair, plan bool) map[string]any {
	return a.platformService().RunDoctorSummaryMode(repair, plan)
}

func (a *app) doctorDataFindings() []string { return a.platformService().DoctorDataFindings() }

func (a *app) runDoctorDataCLI(args []string) int { return a.platformService().RunDoctorDataCLI(args) }

// Keep the http-facing display command in core; it adapts an HTTP response and
// does not own platform state.
func (a *app) platformXAuthority() string { return filepath.Join(a.home, ".Xauthority") }
