package main

import (
	"path/filepath"
	"time"

	maintenancepkg "github.com/DashDashGoApp/Dash-Go/app/internal/household/maintenance"
)

// Maintenance Tracker is now a bounded household child service. Core retains
// HTTP adaptation, canonical People lookup, and the shared ICS/manifest/cache
// commit path; the service owns its durable document, mutex, normalization,
// local-date calculations, task/history mutations, and event projection.
const maintenanceSchema = maintenancepkg.Schema

var maintenanceClock = time.Now

func maintenanceNow() time.Time { return maintenanceClock().In(time.Local) }

func (a *app) maintenanceService() *maintenancepkg.Service {
	a.maintenanceInitMu.Lock()
	defer a.maintenanceInitMu.Unlock()
	if a.maintenance == nil {
		a.maintenance = maintenancepkg.New(maintenancepkg.ServiceConfig{ConfigDir: a.configDir, Now: maintenanceNow})
	}
	return a.maintenance
}
func (a *app) maintenanceFile() string   { return a.maintenanceService().File() }
func maintenanceDefault() map[string]any { return maintenancepkg.Default() }

func maintenanceDate(v any) string { return maintenancepkg.Date(v) }

func maintenanceDateTime(day string) (time.Time, bool) { return maintenancepkg.DateTime(day) }
func maintenanceDueChanged(l, r map[string]any) bool   { return maintenancepkg.DueChanged(l, r) }
func maintenanceNextDue(completed, unit string, every int) string {
	return maintenancepkg.NextDue(completed, unit, every)
}
func normalizeMaintenancePayload(raw map[string]any) map[string]any {
	return maintenancepkg.Normalize(raw, maintenanceNow())
}
func (a *app) maintenancePayload() map[string]any { return a.maintenanceService().Payload() }
func maintenanceFind(payload map[string]any, id string) (int, map[string]any) {
	return maintenancepkg.Find(payload, id)
}
func maintenanceSummary(payload map[string]any) map[string]any {
	return maintenancepkg.Summary(payload, maintenanceNow())
}
func maintenanceTaskPersonID(task map[string]any) string { return maintenancepkg.TaskPersonID(task) }
func maintenanceTaskPersonSnapshot(task map[string]any) string {
	return maintenancepkg.TaskPersonSnapshot(task)
}
func maintenanceAddHistory(payload, task map[string]any, action, occurred, prior, next string) {
	maintenancepkg.AddHistory(payload, task, action, occurred, prior, next, maintenanceNow())
}

func maintenanceDayResponse(payload map[string]any, date string) map[string]any {
	return maintenancepkg.DayResponse(payload, date, maintenanceNow())
}
func maintenanceCalendarOutputEnabled(payload map[string]any) bool {
	return maintenancepkg.CalendarOutputEnabled(payload)
}
func maintenanceCalendarPath(a *app) string { return filepath.Join(a.calDir, "maintenance.ics") }
func maintenanceCalendarEvents(payload map[string]any) []calendarGenEvent {
	events := []calendarGenEvent{}
	for _, event := range maintenancepkg.CalendarEvents(payload) {
		parsed, ok := maintenanceDateTime(event.Date)
		if !ok {
			continue
		}
		item := allDayEvent(parsed.Year(), parsed.Month(), parsed.Day(), event.Summary, event.UID)
		item.Description, item.AppOwner = event.Description, event.AppOwner
		events = append(events, item)
	}
	return events
}
func (a *app) maintenanceTaskFromBody(body, defaults, existing map[string]any) (map[string]any, error) {
	return maintenancepkg.TaskFromBody(body, defaults, existing, maintenanceNow(), func(id string) (string, bool) {
		person, ok := a.householdPeopleActiveAssignment(id)
		if !ok {
			return "", false
		}
		return householdPersonAssignmentName(person), true
	})
}
