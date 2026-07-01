package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// handleCalendarPost owns generated-calendar and household-schedule mutations.
// Keeping these routes together makes the transactional feed/cache contract
// visible instead of hiding it in the general control POST switch.
func (a *app) handleCalendarPost(w http.ResponseWriter, r *http.Request, path string, body map[string]any) bool {
	switch path {
	case "/api/household-schedules":
		result, err := a.saveHouseholdSchedules(body)
		if err != nil {
			a.err(w, err.Error(), http.StatusBadRequest)
			return true
		}
		a.recordAction("calendars", "Save household schedules", "success", "Paydays and pickup calendars refreshed", nil)
		a.json(w, result)
	case "/api/household-schedules/override":
		result, err := a.saveHouseholdScheduleOverride(body)
		if err != nil {
			a.err(w, err.Error(), http.StatusBadRequest)
			return true
		}
		a.recordAction("calendars", "Adjust household schedule", "success", "One scheduled occurrence updated", nil)
		a.json(w, result)
	case "/api/birthdays/add", "/api/birthdays/update", "/api/birthdays/delete", "/api/celebrations/add", "/api/celebrations/update", "/api/celebrations/delete":
		a.handleSpecialDates(w, path, body)
	case "/api/location":
		lat, lon := anyFloat(body["lat"]), anyFloat(body["lon"])
		city := jsonutil.BodyString(body, "city")
		if _, err := writeConfigLocation(a.configLocal, lat, lon, city); err != nil {
			a.err(w, err.Error(), http.StatusBadRequest)
			return true
		}
		moon := a.generateMoonCalendar(true)
		a.json(w, map[string]any{"ok": true, "lat": lat, "lon": lon, "city": city, "moon": moon, "generator": "go"})
	case "/api/calendars/toggle":
		a.handleCalendarToggle(w, body)
	case "/api/calendars/manage/delete", "/api/calendars/manage/restore", "/api/calendars/manage/app-output", "/api/calendars/manage/repair":
		a.handleCalendarManagementPost(w, path, body)
	case "/api/moon/update":
		moon := a.generateMoonCalendar(true)
		sky := a.generateStaticSkyCalendars()
		a.writeSkyStatus(map[string]any{"ok": true, "updatedAt": time.Now().Unix(), "moon": moon, "sky": sky, "generator": "go"})
		a.recordAction("settings", "Update moon/sky calendars", "success", fmt.Sprintf("%v moon events", moon["eventCount"]), nil)
		a.json(w, map[string]any{"ok": true, "moon": moon, "sky": sky, "generator": "go"})
	case "/api/calendars/sync":
		res, err := a.generateDefaultCalendars(true)
		if err != nil {
			a.err(w, "calendar sync failed: "+err.Error(), http.StatusInternalServerError)
			return true
		}
		a.recordAction("calendars", "Sync calendars", "success", "Go calendar generators refreshed", nil)
		a.json(w, res)
	default:
		return false
	}
	return true
}
