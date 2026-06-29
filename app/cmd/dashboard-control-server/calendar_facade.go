package main

import (
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	calendarpkg "github.com/DashDashGoApp/Dash-Go/app/internal/calendar"
	eventspkg "github.com/DashDashGoApp/Dash-Go/app/internal/calendar/events"
	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// Calendar management is a bounded service. Core retains CLI/HTTP adaptation,
// the thin household app output callbacks, and event-service composition only.
type (
	calendarGenEvent    = calendarpkg.Event
	calendarTrashRecord = calendarpkg.TrashRecord
)

const (
	calendarTrashSchema        = calendarpkg.TrashSchema
	calendarTrashRetentionDays = calendarpkg.TrashRetentionDays
	calendarTrashLimit         = calendarpkg.TrashLimit
)

var issHTTPClient = &http.Client{Timeout: 30 * time.Second}

func (a *app) newCalendarService(refreshCacheAsync func()) *calendarpkg.Service {
	if refreshCacheAsync == nil {
		refreshCacheAsync = a.refreshCalendarCacheAsync
	}
	return calendarpkg.New(calendarpkg.ServiceConfig{
		DashDir: a.dash, HomeDir: a.home, CalendarDir: a.calDir, CacheDir: a.cacheDir,
		LogDir: a.logDir, ConfigLocal: a.configLocal, CelebrationsFile: a.celebrationsFile,
		Now:                  time.Now,
		OutputEnabled:        a.appCalendarOutputEnabled,
		AppKnown:             a.appCalendarKnown,
		SetAppOutput:         a.setAppCalendarOutputState,
		GenerateMoon:         a.generateMoonCalendar,
		GenerateSky:          a.generateStaticSkyCalendars,
		EnableISOWeekNumbers: a.migrateISOWeekSetting,
		RefreshCacheSync:     a.refreshEventCacheAfterCalendarWrite,
		RefreshCacheAsync:    refreshCacheAsync,
		IndexWarning:         a.recordCalendarIndexWarning,
		HTTPClient:           func() *http.Client { return issHTTPClient },
	})
}

func (a *app) calendarService() *calendarpkg.Service {
	a.calendarInitMu.Lock()
	defer a.calendarInitMu.Unlock()
	if a.calendar == nil {
		a.calendar = a.newCalendarService(nil)
	}
	return a.calendar
}

// refreshEventCacheAfterCalendarWrite intentionally calls only the event child
// service. Calendar has already written the manifest under its own lock, so
// re-entering the manifest generator here would create a false lock cycle.
func (a *app) refreshEventCacheAfterCalendarWrite() error {
	_, err := a.eventService().Refresh(true, 90, 365)
	return err
}

// refreshCalendarCacheAsync is the post-commit wake-up port used by Calendar.
// Runtime work remains bounded and begins only after Calendar releases its
// transaction lock. Tests inject a no-op directly through ServiceConfig before
// constructing their Calendar service.
func (a *app) refreshCalendarCacheAsync() {
	go func() { _, _ = a.refreshEventCache(true, 90, 365) }()
}
func (a *app) recordCalendarIndexWarning(owner string, err error) {
	label := map[string]string{"chore-wheel": "Chores", "maintenance": "Maintenance", "routines": "Routines"}[owner]
	if label == "" {
		label = "Calendar"
	}
	a.recordAction("calendars", "Repair "+label+" calendar index", "warning", err.Error(), nil)
}

func calendarEntryEnabled(value map[string]any) bool { return calendarpkg.CalendarEntryEnabled(value) }
func calendarSourceIdentity(url string) string       { return calendarpkg.SourceIdentity(url) }

func ownedCalendarSource(url string) (calendarSource, bool) {
	owned, ok := calendarpkg.OwnedSource(url)
	if !ok {
		return calendarSource{}, false
	}
	return eventspkg.CalendarSource{URL: owned.URL, Name: owned.Name, Color: owned.Color, Tag: owned.Tag, Owner: owned.Owner}, true
}

func allDayEvent(year int, month time.Month, day int, summary, uid string) calendarGenEvent {
	return calendarpkg.AllDayEvent(year, month, day, summary, uid)
}
func writeICSFile(path, name string, events []calendarGenEvent) error {
	return calendarpkg.WriteICSFile(path, name, events)
}

func icsEsc(value string) string { return calendarpkg.EscapeICS(value) }

func (a *app) generateCalendarManifest() error { return a.calendarService().GenerateManifest() }
func (a *app) calendars() any                  { return a.calendarService().Calendars() }

func (a *app) calendarManagementStatus() map[string]any {
	return a.calendarService().ManagementStatus()
}
func (a *app) archiveLocalCalendar(url, displayName string) (calendarTrashRecord, error) {
	return a.calendarService().Archive(url, displayName)
}
func (a *app) restoreLocalCalendar(id string) (calendarTrashRecord, error) {
	return a.calendarService().Restore(id)
}
func (a *app) repairCalendarIndex() (map[string]any, error) { return a.calendarService().Repair() }
func (a *app) setOwnedCalendarOutput(owner string, enabled bool) (map[string]any, error) {
	return a.calendarService().SetOwnedOutput(owner, enabled)
}

func (a *app) generateDefaultCalendars(refresh bool) (map[string]any, error) {
	return a.calendarService().GenerateDefaults(refresh)
}
func (a *app) updateHolidayCalendars() map[string]any { return a.calendarService().UpdateHolidays() }
func (a *app) updateISSPasses() map[string]any        { return a.calendarService().UpdateISSPasses() }
func (a *app) calendarTrashDir() string               { return a.calendarService().TrashDir() }

func (a *app) loadCalendarTrash() []calendarTrashRecord { return a.calendarService().LoadTrash() }
func (a *app) writeCalendarTrash(records []calendarTrashRecord) error {
	return a.calendarService().WriteTrash(records)
}

func (a *app) calendarPathForURL(url string) (string, string, error) {
	return a.calendarService().PathForURL(url)
}
func (a *app) purgeExpiredCalendarTrash() int { return a.calendarService().PurgeExpiredTrash() }

func (a *app) calendarOutputEnabledForURL(url string) bool {
	return a.calendarService().OutputEnabledForURL(url)
}

// appCalendarOutputEnabled and appCalendarKnown are the only household ports
// Calendar needs. They read app-local durable state without granting Calendar
// a dependency on People, Chores, Maintenance, or Routines.
func (a *app) appCalendarOutputEnabled(owner string) bool {
	switch owner {
	case "chore-wheel":
		return choreWheelCalendarOutputEnabled(a.choreWheelService().Payload())
	case "maintenance":
		return maintenanceCalendarOutputEnabled(a.maintenanceService().Payload())
	case "routines":
		return routinesCalendarOutputEnabled(a.routinesService().Payload())
	default:
		return true
	}
}
func (a *app) appCalendarKnown(owner string) bool {
	switch owner {
	case "chore-wheel":
		return fileio.Exists(a.choreWheelFile()) || fileio.Exists(filepath.Join(a.calDir, "chore-wheel.ics"))
	case "maintenance":
		return fileio.Exists(a.maintenanceFile()) || fileio.Exists(maintenanceCalendarPath(a))
	case "routines":
		return fileio.Exists(a.routinesFile()) || fileio.Exists(routinesCalendarPath(a))
	default:
		return false
	}
}

// calendarOutputSnapshot obtains the other household output flags before an
// app-held mutation enters Calendar. The committing app supplies its new value
// directly so Calendar never needs to read that app while it is locked.
func (a *app) calendarOutputSnapshot(committingOwner string, committingEnabled bool) map[string]bool {
	state := make(map[string]bool, 3)
	for _, owner := range []string{"chore-wheel", "maintenance", "routines"} {
		if owner == committingOwner {
			state[owner] = committingEnabled
			continue
		}
		state[owner] = a.appCalendarOutputEnabled(owner)
	}
	return state
}

// setAppCalendarOutputState is a thin cross-domain callback. Calendar validates
// ownership; this adapter changes only the selected child document, then each
// child commits through Calendar's owned-feed transaction.
func (a *app) setAppCalendarOutputState(owner string, enabled bool) (map[string]any, error) {
	switch owner {
	case "chore-wheel":
		var payload map[string]any
		err := a.choreWheelService().WithLock(func() error {
			payload = a.choreWheelService().Payload()
			settings := jsonutil.Map(payload["settings"])
			settings["calendarOutputEnabled"] = enabled
			payload["settings"] = settings
			payload = normalizeChoreWheelPayload(payload)
			payload["revision"] = max(0, jsonutil.Int(payload["revision"], 0)) + 1
			return a.commitChoreWheelPayload(payload)
		})
		if err != nil {
			return nil, err
		}
		return map[string]any{"owner": owner, "enabled": enabled, "state": payload}, nil
	case "maintenance":
		var payload map[string]any
		err := a.maintenanceService().WithLock(func() error {
			payload = a.maintenanceService().Payload()
			settings := jsonutil.Map(payload["settings"])
			settings["calendarOutputEnabled"] = enabled
			payload["settings"] = settings
			payload = normalizeMaintenancePayload(payload)
			return a.commitMaintenancePayload(payload)
		})
		if err != nil {
			return nil, err
		}
		return map[string]any{"owner": owner, "enabled": enabled, "state": payload, "summary": maintenanceSummary(payload)}, nil
	case "routines":
		var payload map[string]any
		err := a.routinesService().WithLock(func() error {
			payload = a.routinesService().Payload()
			settings := jsonutil.Map(payload["settings"])
			settings["calendarOutputEnabled"] = enabled
			payload["settings"] = settings
			payload = normalizeRoutinesPayload(payload)
			return a.commitRoutinesPayload(payload)
		})
		if err != nil {
			return nil, err
		}
		return map[string]any{"owner": owner, "enabled": enabled, "state": payload, "summary": routinesSummary(payload)}, nil
	default:
		return nil, fmt.Errorf("unknown app calendar")
	}
}

// Calendar invokes this Settings-owned mutation through a narrow callback while
// generating defaults; it preserves the legacy one-way ISO-week enablement.
func (a *app) migrateISOWeekSetting() {
	path := a.settingsFile
	value := jsonutil.Map(a.readJSONDefault(path, map[string]any{}))
	if value["showIsoWeekNumbers"] == true {
		return
	}
	value["showIsoWeekNumbers"] = true
	_ = fileio.WriteJSON(path, value)
}
