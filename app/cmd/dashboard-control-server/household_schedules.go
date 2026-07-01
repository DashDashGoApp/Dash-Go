package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	calendarpkg "github.com/DashDashGoApp/Dash-Go/app/internal/calendar"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// Household schedules are deliberate local dashboard data. They are not a
// generic editor for subscribed calendars, and their generated event metadata
// is the only way the dashboard exposes a per-occurrence Manage action.
func (a *app) householdSchedulesFile() string {
	return filepath.Join(a.configDir, "household-schedules.json")
}

func (a *app) householdSchedulesPayload() (map[string]any, error) {
	cfg, migrated, err := a.calendarService().HouseholdSchedules()
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"schedules":     cfg,
		"migrated":      migrated,
		"holidayLayers": a.calendarService().AvailableHolidayLayers(),
		"preview":       a.calendarService().HouseholdSchedulePreview(cfg, 3),
	}, nil
}

func householdSchedulesFromBody(body map[string]any) (calendarpkg.HouseholdSchedules, error) {
	raw := body["schedules"]
	if raw == nil {
		raw = body
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return calendarpkg.HouseholdSchedules{}, err
	}
	var out calendarpkg.HouseholdSchedules
	if err := json.Unmarshal(encoded, &out); err != nil {
		return calendarpkg.HouseholdSchedules{}, fmt.Errorf("household schedules are malformed")
	}
	return out, nil
}

func (a *app) saveHouseholdSchedules(body map[string]any) (map[string]any, error) {
	next, err := householdSchedulesFromBody(body)
	if err != nil {
		return nil, err
	}
	saved, err := a.calendarService().SaveHouseholdSchedules(next)
	if err != nil {
		return nil, err
	}
	// Calendar has committed the feed and rebuilt its manifest before this
	// synchronous cache refresh. A success response is therefore immediately
	// usable by the dashboard, not merely queued for later work.
	if _, err := a.refreshEventCache(true, 90, 365); err != nil {
		return nil, fmt.Errorf("schedule saved but event cache refresh failed: %w", err)
	}
	payload, err := a.householdSchedulesPayload()
	if err != nil {
		return nil, err
	}
	payload["schedules"] = saved
	return payload, nil
}

func (a *app) saveHouseholdScheduleOverride(body map[string]any) (map[string]any, error) {
	change := calendarpkg.ScheduleOverride{
		RuleID:      strings.ToLower(strings.TrimSpace(jsonutil.BodyString(body, "ruleId"))),
		NominalDate: strings.TrimSpace(jsonutil.BodyString(body, "nominalDate")),
		Action:      strings.ToLower(strings.TrimSpace(jsonutil.BodyString(body, "action"))),
		ActualDate:  strings.TrimSpace(jsonutil.BodyString(body, "actualDate")),
	}
	if change.RuleID == "" {
		return nil, fmt.Errorf("schedule rule is required")
	}
	result, err := a.calendarService().SetHouseholdScheduleOverrideWithResult(change)
	if err != nil {
		return nil, err
	}
	if _, err := a.refreshEventCache(true, 90, 365); err != nil {
		return nil, fmt.Errorf("schedule adjustment saved but event cache refresh failed: %w", err)
	}
	payload, err := a.householdSchedulesPayload()
	if err != nil {
		return nil, err
	}
	payload["schedules"] = result.Schedules
	payload["adjustment"] = map[string]any{
		"ruleId":      result.RuleID,
		"nominalDate": result.NominalDate,
		"actualDate":  result.ActualDate,
		"collision":   result.Collision,
	}
	return payload, nil
}
