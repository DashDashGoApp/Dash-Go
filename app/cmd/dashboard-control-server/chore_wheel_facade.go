package main

import (
	"time"

	chorepkg "github.com/DashDashGoApp/Dash-Go/app/internal/household/chores"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// Chore Wheel now lives in internal/household/chores. Core keeps this narrow
// facade so HTTP, the People transaction, calendar manifest/cache work, and
// existing route/integration tests retain their stable contracts while the
// child service owns the local model, its lock, normalization, deterministic
// planning primitives, day projections, and calendar-event projection.
var choreWheelClock = time.Now

func choreWheelNow() time.Time { return choreWheelClock().In(time.Local) }

func (a *app) choreWheelService() *chorepkg.Service {
	a.choresInitMu.Lock()
	defer a.choresInitMu.Unlock()
	if a.chores == nil {
		a.chores = chorepkg.New(chorepkg.ServiceConfig{ConfigDir: a.configDir, Now: choreWheelNow})
	}
	return a.chores
}

func (a *app) choreWheelFile() string { return a.choreWheelService().File() }

func choreWheelID(v any) string              { return chorepkg.ID(v) }
func choreWheelText(v any, limit int) string { return chorepkg.Text(v, limit) }
func choreWheelDateKey(v any) string         { return chorepkg.DateKey(v) }

func normalizeChoreWheelPayload(raw map[string]any) map[string]any {
	return chorepkg.NormalizeAt(raw, choreWheelNow())
}
func choreWheelCalendarRange(payload map[string]any) (time.Time, time.Time) {
	return chorepkg.CalendarRange(payload, choreWheelNow())
}
func choreWheelCalendarOutputEnabled(payload map[string]any) bool {
	return chorepkg.CalendarOutputEnabled(payload)
}

func choreWheelDayResponse(payload map[string]any, date string) map[string]any {
	return chorepkg.DayResponse(payload, date, choreWheelNow())
}

// choreWheelPayloadForRoster applies the canonical active People projection
// without acquiring a People lock. Cross-domain mutations use it only after
// core has taken the People lock, so no Chore -> People lock inversion exists.
func choreWheelPayloadForRoster(payload, roster map[string]any) map[string]any {
	payload = normalizeChoreWheelPayload(payload)
	if len(jsonutil.List(roster["people"])) > 0 {
		payload["people"] = householdPeopleActive(roster)
	}
	return normalizeChoreWheelPayload(payload)
}

func (a *app) choreWheelPayload() map[string]any {
	payload := a.choreWheelService().Payload()
	// Chore Wheel remains a roster consumer. The one-time seed and active
	// People projection stay in core because the People bounded service owns the
	// canonical roster and its mutation lock. This read path holds no Chore lock
	// while it calls People, leaving mutations to use People -> Chore ordering.
	roster := a.ensureHouseholdPeople(jsonutil.List(payload["people"]))
	return choreWheelPayloadForRoster(payload, roster)
}

const choreWheelSchema = chorepkg.Schema
