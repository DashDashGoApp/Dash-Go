package main

import (
	"net/http"
	"path/filepath"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
)

func (a *app) handleGet(w http.ResponseWriter, r *http.Request, path string) {
	if a.handleFontGet(w, r, path) {
		return
	}
	if path == "/api/ready" {
		a.json(w, a.runtimeReady())
		return
	}
	if path == "/api/lock/status" {
		a.json(w, a.pinStatus(r.Header.Get("X-Dashboard-Token")))
		return
	}
	if path == "/api/health" {
		a.json(w, a.deviceHealth())
		return
	}
	if path == "/api/weather" {
		a.json(w, a.weatherPayload())
		return
	}
	if path == "/api/event-map" {
		a.json(w, a.eventMapLookup(r.URL.Query().Get("q")))
		return
	}
	if path == "/api/event-map-img" {
		a.handleMapImage(w, r)
		return
	}
	if path == "/api/maps/status" {
		a.json(w, a.mapCacheStatus())
		return
	}
	if path == "/api/radar/status" {
		a.json(w, a.radarStatus())
		return
	}
	if path == "/api/radar/tile" {
		a.handleRadarTile(w, r)
		return
	}
	if a.handleTodoGet(w, r, path) {
		return
	}
	if a.handleChoreWheelGet(w, r, path) {
		return
	}
	if a.handleFamilyBoardGet(w, r, path) {
		return
	}
	if a.handleMaintenanceGet(w, r, path) {
		return
	}
	if a.handleRoutinesGet(w, r, path) {
		return
	}
	if !a.tokenOK(r.Header.Get("X-Dashboard-Token")) {
		a.err(w, "locked", 401)
		return
	}
	if a.handleHouseholdPeopleGet(w, r, path) {
		return
	}
	switch path {
	case "/api/status":
		a.json(w, a.systemStatus())
	case "/api/terminal/status":
		a.json(w, a.terminalStatus())
	case "/api/action-history":
		a.json(w, a.actionHistory(25))
	case "/api/system-update/status":
		a.json(w, a.systemUpdateStatus())
	case "/api/compliments":
		a.json(w, a.complimentsPayload())
	case "/api/message-sources":
		a.json(w, a.messageSourcesStatus())
	case "/api/temporary-messages":
		a.json(w, map[string]any{"items": a.temporaryMessages()})
	case "/api/scheduled-messages":
		a.json(w, map[string]any{"items": a.scheduledMessages()})
	case "/api/birthdays":
		a.json(w, map[string]any{"items": a.loadBirthdays()})
	case "/api/celebrations":
		a.json(w, map[string]any{"items": a.loadCelebrations()})
	case "/api/calendars":
		a.json(w, a.calendars())
	case "/api/calendars/manage":
		a.json(w, a.calendarManagementStatus())
	case "/api/household-schedules":
		payload, err := a.householdSchedulesPayload()
		if err != nil {
			a.err(w, err.Error(), 500)
			return
		}
		a.json(w, payload)
	case "/api/calendars/health", "/api/cache/status":
		a.json(w, a.cacheStatus())
	case "/api/moon/status":
		a.json(w, a.moonCalendarStatus())
	case "/api/settings":
		a.json(w, a.loadSettings())
	case "/api/profile":
		settings := a.loadSettings()
		a.json(w, a.profilePayloadForSettings(settings))
	case "/api/update/status":
		if r.URL.Query().Get("fresh") == "1" {
			a.json(w, a.updateStatusFresh())
		} else {
			a.json(w, a.updateStatus())
		}
	case "/api/update/progress":
		a.json(w, a.updateProgress())
	case "/api/update/availability":
		a.json(w, a.checkUpdateAvailability())
	case "/api/update/log":
		a.json(w, map[string]any{"log": tailFile(filepath.Join(a.logDir, "update.log"), 8000)})
	case "/api/doctor/status":
		a.json(w, a.loadHealthStatus())
	case "/api/memory/status":
		a.json(w, memoryStatus())
	case "/api/chalkboard":
		a.json(w, a.readJSONDefault(filepath.Join(a.configDir, "chalkboard.json"), map[string]any{"version": 1, "strokes": []any{}}))
	case "/api/themes":
		themes, availability := a.availableThemes()
		a.json(w, map[string]any{"themes": themes, "current": a.currentTheme(), "base": fileio.ReadString(filepath.Join(a.home, ".dashboard-base-theme"), "basic"), "seasonal": a.seasonalThemesEnabled(), "optionalThemeInfo": availability.Reasons, "optionalThemes": availability.Available, "themeAvailabilityDate": availability.Today})
	case "/api/logs":
		name := r.URL.Query().Get("name")
		a.json(w, map[string]any{"name": name, "log": tailFile(a.logPath(name), 12000)})
	case "/api/geocode":
		a.json(w, a.geocode(r.URL.Query().Get("q")))
	default:
		a.err(w, "unknown endpoint", 404)
	}
}
