package main

import (
	"path/filepath"
	"time"

	routinespkg "github.com/DashDashGoApp/Dash-Go/app/internal/household/routines"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// Routines is now a bounded household child service. It owns its JSON state,
// mutex, normalization, recurrence/local-date behavior, occurrence/history
// corrections, roster projection, and calendar-event projection. Core retains
// HTTP adaptation, canonical People writes, and shared ICS/manifest/cache I/O.
const (
	routinesSchema              = routinespkg.Schema
	routinesMaxPeople           = routinespkg.MaxPeople
	routinesMaxItems            = routinespkg.MaxItems
	routinesMaxSteps            = routinespkg.MaxSteps
	routinesMaxOccurrences      = routinespkg.MaxOccurrences
	routinesMaxHistory          = routinespkg.MaxHistory
	routinesHistoryDays         = routinespkg.HistoryDays
	routinesMaxCalendarSessions = routinespkg.MaxCalendarSessions
)

var routinesClock = time.Now

func routinesNow() time.Time { return routinesClock().In(time.Local) }
func routinesToday() string  { return routinesNow().Format("2006-01-02") }
func (a *app) routinesService() *routinespkg.Service {
	a.routinesInitMu.Lock()
	defer a.routinesInitMu.Unlock()
	if a.routines == nil {
		a.routines = routinespkg.New(routinespkg.ServiceConfig{ConfigDir: a.configDir, Now: routinesNow})
	}
	return a.routines
}
func (a *app) routinesFile() string { return a.routinesService().File() }

func routinesText(v any, limit int) string { return routinespkg.Text(v, limit) }
func routinesID(v any) string              { return routinespkg.ID(v) }
func routinesDate(v any) string            { return routinespkg.Date(v) }

func routinesBoolDefault(v any, fallback bool) bool { return routinespkg.BoolDefault(v, fallback) }
func routinesSchedule(raw any, fallback string) map[string]any {
	return routinespkg.Schedule(raw, fallback, routinesNow())
}
func routineDueOn(schedule map[string]any, day time.Time) bool {
	return routinespkg.DueOn(schedule, day)
}

func routinesOccurrencesForDay(payload map[string]any, date string) []map[string]any {
	return routinespkg.OccurrencesForDay(payload, date, routinesNow())
}
func normalizeRoutinesPayload(raw map[string]any) map[string]any {
	return routinespkg.Normalize(raw, routinesNow())
}
func routinesAppendHistory(payload map[string]any, action string, occ map[string]any) {
	routinespkg.AppendHistory(payload, action, occ, routinesNow())
}
func routinesPersonName(payload map[string]any, id string) string {
	return routinespkg.PersonName(payload, id)
}
func routinesFind(payload map[string]any, id string) (int, map[string]any) {
	return routinespkg.Find(payload, id)
}

func routinesCalendarOutputEnabled(payload map[string]any) bool {
	return routinespkg.CalendarOutputEnabled(payload)
}

func routinesCalendarPath(a *app) string { return filepath.Join(a.calDir, "routines.ics") }
func routinesCalendarEvents(payload map[string]any) []calendarGenEvent {
	out := []calendarGenEvent{}
	for _, event := range routinespkg.CalendarEvents(payload, routinesNow()) {
		parsed, err := time.ParseInLocation("2006-01-02", event.Date, time.Local)
		if err != nil {
			continue
		}
		item := allDayEvent(parsed.Year(), parsed.Month(), parsed.Day(), event.Summary, event.UID)
		item.Description, item.AppOwner = event.Description, event.AppOwner
		item.Start, item.End = event.Start, event.End
		out = append(out, item)
	}
	return out
}

// routinesPayloadForRoster projects a Routines snapshot through an already-read
// canonical roster. It is deliberately read-only: callers that already hold the
// People lock (for example impact/count projections) must not call Ensure and
// attempt to re-enter the non-reentrant roster mutex.
func (a *app) routinesPayloadForRoster(roster map[string]any) map[string]any {
	payload := a.routinesService().Payload()
	return routinespkg.ProjectRoster(payload, roster, routinesNow())
}

// routinesPayload is a consumer projection of canonical People. It never holds
// the Routines lock while it asks the People service to seed/merge the roster,
// preserving People → Routines ordering for cross-domain mutations.
func (a *app) routinesPayload() map[string]any {
	payload := a.routinesService().Payload()
	chores := a.choreWheelService().Payload()
	roster := a.ensureHouseholdPeople(jsonutil.List(payload["people"]), jsonutil.List(chores["people"]))
	return routinespkg.ProjectRoster(payload, roster, routinesNow())
}
func routineOccurrenceApplyStep(payload, occ map[string]any, stepID string, checked bool, now string) (bool, error) {
	stamp, err := time.Parse(time.RFC3339, now)
	if err != nil {
		stamp = routinesNow()
	}
	return routinespkg.ApplyStep(payload, occ, routinesID(stepID), checked, stamp)
}
