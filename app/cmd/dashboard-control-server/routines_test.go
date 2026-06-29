package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func TestRoutinesPayloadUsesCompleteDefaultsForWrongShape(t *testing.T) {
	a := testProfileApp(t)
	if err := os.WriteFile(a.routinesFile(), []byte(`[]`), 0644); err != nil {
		t.Fatal(err)
	}
	payload := a.routinesPayload()
	if payload["schema"] != routinesSchema {
		t.Fatalf("routines defaults = %#v", payload)
	}
	if enabled := routinesBoolDefault(jsonutil.Map(payload["settings"])["calendarOutputEnabled"], false); !enabled {
		t.Fatalf("routines must default calendar output on: %#v", payload)
	}
}

func TestRoutineCadenceUsesLocalDates(t *testing.T) {
	weekly := routinesSchedule(map[string]any{"kind": "weekly", "every": 1, "weekdays": []any{"MO", "WE"}, "startOn": "2026-06-22"}, "")
	monday, _ := time.ParseInLocation("2006-01-02", "2026-06-22", time.Local)
	tuesday, _ := time.ParseInLocation("2006-01-02", "2026-06-23", time.Local)
	if !routineDueOn(weekly, monday) || routineDueOn(weekly, tuesday) {
		t.Fatalf("weekly local cadence mismatch: %#v", weekly)
	}
	monthly := routinesSchedule(map[string]any{"kind": "monthly", "every": 2, "day": 15, "startOn": "2026-01-15"}, "")
	mar, _ := time.ParseInLocation("2006-01-02", "2026-03-15", time.Local)
	feb, _ := time.ParseInLocation("2006-01-02", "2026-02-15", time.Local)
	if !routineDueOn(monthly, mar) || routineDueOn(monthly, feb) {
		t.Fatalf("monthly cadence mismatch: %#v", monthly)
	}
}

func TestRoutinesCalendarIsBoundedAndCarriesOwner(t *testing.T) {
	a := testProfileApp(t)
	oldClock := routinesClock
	routinesClock = func() time.Time { return time.Date(2026, 6, 24, 9, 0, 0, 0, time.Local) }
	t.Cleanup(func() { routinesClock = oldClock })
	payload := normalizeRoutinesPayload(map[string]any{
		"people":   []any{map[string]any{"id": "sam", "name": "Sam"}},
		"routines": []any{map[string]any{"id": "morning", "title": "Morning routine", "steps": []any{map[string]any{"id": "teeth", "text": "Brush teeth"}}, "assignments": []any{map[string]any{"id": "sam-am", "personId": "sam", "calendarEnabled": true, "schedule": map[string]any{"kind": "weekdays", "startOn": "2026-06-01", "time": "07:15", "allDay": false}}}}},
	})
	if err := a.writeRoutinesCalendar(payload); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(routinesCalendarPath(a))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, "SUMMARY:Routines — Sam · 1") || !strings.Contains(text, "DESCRIPTION:Dash-Go Routines\\nMorning routine · Sam") || !strings.Contains(text, "X-DASHGO-APP-OWNER:routines") {
		t.Fatalf("routines calendar metadata/event missing: %s", text)
	}
	if strings.Contains(text, "202609") {
		t.Fatalf("routines calendar ignored bounded horizon: %s", text)
	}
}

func TestRoutinesCalendarRefusesExcessiveGeneratedSessions(t *testing.T) {
	a := testProfileApp(t)
	oldClock := routinesClock
	routinesClock = func() time.Time { return time.Date(2026, 6, 24, 9, 0, 0, 0, time.Local) }
	t.Cleanup(func() { routinesClock = oldClock })
	people := []any{}
	assignments := []any{}
	for i := 0; i < 20; i++ {
		people = append(people, map[string]any{"id": fmt.Sprintf("p%d", i), "name": fmt.Sprintf("Person %d", i)})
		assignments = append(assignments, map[string]any{"id": fmt.Sprintf("a%d", i), "personId": fmt.Sprintf("p%d", i), "schedule": map[string]any{"kind": "days", "every": 1, "startOn": "2026-06-01"}})
	}
	routines := []any{}
	for i := 0; i < 10; i++ {
		routines = append(routines, map[string]any{"id": fmt.Sprintf("r%d", i), "title": fmt.Sprintf("Routine %d", i), "steps": []any{map[string]any{"id": fmt.Sprintf("s%d", i), "text": "Step"}}, "assignments": assignments})
	}
	payload := normalizeRoutinesPayload(map[string]any{"people": people, "routines": routines, "settings": map[string]any{"calendarHorizonDays": 90}})
	if err := a.writeRoutinesCalendar(payload); err == nil {
		t.Fatal("dense routines schedule must reject more than the bounded calendar session limit")
	}
}

func TestRoutineCompletionTracksFinalChecklistStep(t *testing.T) {
	payload := normalizeRoutinesPayload(map[string]any{
		"people": []any{map[string]any{"id": "sam", "name": "Sam"}},
		"routines": []any{map[string]any{
			"id": "morning", "title": "Morning", "steps": []any{
				map[string]any{"id": "teeth", "text": "Brush teeth"},
				map[string]any{"id": "bag", "text": "Pack bag"},
			},
			"assignments": []any{map[string]any{"id": "sam-am", "personId": "sam", "schedule": map[string]any{"kind": "days", "startOn": "2026-06-24"}}},
		}},
	})
	occurrence := map[string]any{"routineId": "morning", "assignmentId": "sam-am", "personId": "sam", "date": "2026-06-24", "state": "active", "completedStepIds": []any{"teeth"}}
	if _, err := routineOccurrenceApplyStep(payload, occurrence, "bag", true, "2026-06-24T12:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if got := occurrence["state"]; got != "completed" {
		t.Fatalf("final checked step state=%v, want completed", got)
	}
	if got := occurrence["completedAt"]; got == "" {
		t.Fatalf("final checked step must stamp completedAt: %#v", occurrence)
	}
	if _, err := routineOccurrenceApplyStep(payload, occurrence, "teeth", false, "2026-06-24T12:01:00Z"); err != nil {
		t.Fatal(err)
	}
	if got := occurrence["state"]; got != "active" {
		t.Fatalf("unchecked completed routine state=%v, want active", got)
	}
	if got := occurrence["completedAt"]; got != "" {
		t.Fatalf("reopened occurrence retained completedAt=%v", got)
	}
}

func TestRoutineCadenceHonorsCivilDatesWeekdayIntervalAndShortMonths(t *testing.T) {
	oldLocal := time.Local
	loc, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Fatal(err)
	}
	time.Local = loc
	t.Cleanup(func() { time.Local = oldLocal })
	parse := func(value string) time.Time {
		day, err := time.ParseInLocation("2006-01-02", value, time.Local)
		if err != nil {
			t.Fatal(err)
		}
		return day
	}
	weekdayEveryTwo := routinesSchedule(map[string]any{"kind": "weekdays", "every": 2, "startOn": "2026-03-02"}, "")
	for _, value := range []string{"2026-03-02", "2026-03-06", "2026-03-16"} {
		if !routineDueOn(weekdayEveryTwo, parse(value)) {
			t.Fatalf("weekday interval should be due on %s: %#v", value, weekdayEveryTwo)
		}
	}
	if routineDueOn(weekdayEveryTwo, parse("2026-03-09")) {
		t.Fatalf("weekday interval must not run every calendar week: %#v", weekdayEveryTwo)
	}
	everyTwoDays := routinesSchedule(map[string]any{"kind": "days", "every": 2, "startOn": "2026-03-07"}, "")
	if !routineDueOn(everyTwoDays, parse("2026-03-09")) || routineDueOn(everyTwoDays, parse("2026-03-08")) {
		t.Fatalf("daily interval must use calendar days across DST: %#v", everyTwoDays)
	}
	monthly31 := routinesSchedule(map[string]any{"kind": "monthly", "every": 1, "day": 31, "startOn": "2026-01-31"}, "")
	if !routineDueOn(monthly31, parse("2026-02-28")) || !routineDueOn(monthly31, parse("2026-03-31")) {
		t.Fatalf("monthly 31st must run on final short-month day: %#v", monthly31)
	}
	yearlyLeap := routinesSchedule(map[string]any{"kind": "yearly", "every": 1, "month": 2, "day": 29, "startOn": "2024-02-29"}, "")
	if !routineDueOn(yearlyLeap, parse("2025-02-28")) {
		t.Fatalf("yearly Feb 29 must use Feb 28 on a non-leap year: %#v", yearlyLeap)
	}
}

func TestRoutinesCalendarAggregatesSamePersonAndTime(t *testing.T) {
	oldClock := routinesClock
	routinesClock = func() time.Time { return time.Date(2026, 6, 24, 9, 0, 0, 0, time.Local) }
	t.Cleanup(func() { routinesClock = oldClock })
	payload := normalizeRoutinesPayload(map[string]any{
		"people": []any{map[string]any{"id": "sam", "name": "Sam"}},
		"routines": []any{
			map[string]any{
				"id": "morning", "title": "Morning",
				"steps": []any{map[string]any{"id": "teeth", "text": "Brush teeth"}},
				"assignments": []any{map[string]any{
					"id": "sam-am-1", "personId": "sam", "calendarEnabled": true,
					"schedule": map[string]any{"kind": "days", "startOn": "2026-06-01", "time": "07:15", "allDay": false},
				}},
			},
			map[string]any{
				"id": "backpack", "title": "Backpack",
				"steps": []any{map[string]any{"id": "bag", "text": "Pack bag"}},
				"assignments": []any{map[string]any{
					"id": "sam-am-2", "personId": "sam", "calendarEnabled": true,
					"schedule": map[string]any{"kind": "days", "startOn": "2026-06-01", "time": "07:15", "allDay": false},
				}},
			},
		},
	})
	events := routinesCalendarEvents(payload)
	matching := 0
	for _, event := range events {
		if event.Date.In(time.Local).Format("2006-01-02") == "2026-06-24" {
			matching++
			if event.Summary != "Routines — Sam · 2" {
				t.Fatalf("aggregated summary=%q", event.Summary)
			}
		}
	}
	if matching != 1 {
		t.Fatalf("same person/time should project one calendar session, got %d", matching)
	}
}

func TestSharedRosterSeedsOnceAndPausedPersonStopsFutureSessions(t *testing.T) {
	a := testProfileApp(t)
	routinePayload := map[string]any{
		"people": []any{map[string]any{"id": "sam", "name": "Sam from Routines"}},
		"routines": []any{map[string]any{
			"id": "morning", "title": "Morning",
			"steps": []any{map[string]any{"id": "step", "text": "Step"}},
			"assignments": []any{map[string]any{
				"id": "sam-am", "personId": "sam",
				"schedule": map[string]any{"kind": "days", "startOn": routinesToday()},
			}},
		}},
	}
	if err := fileio.WriteJSON(a.routinesFile(), routinePayload); err != nil {
		t.Fatal(err)
	}
	if err := fileio.WriteJSON(a.choreWheelFile(), map[string]any{
		"people": []any{map[string]any{"id": "sam", "name": "Sam from Chores"}},
	}); err != nil {
		t.Fatal(err)
	}
	payload := a.routinesPayload()
	if got := routinesPersonName(payload, "sam"); got != "Sam from Routines" {
		t.Fatalf("canonical roster seeded wrong name=%q", got)
	}
	chorePayload := a.choreWheelPayload()
	chorePeople := jsonutil.List(chorePayload["people"])
	if len(chorePeople) != 1 {
		t.Fatalf("Chore Wheel roster=%#v", chorePeople)
	}
	if got := choreWheelText(jsonutil.Map(chorePeople[0])["name"], 64); got != "Sam from Routines" {
		t.Fatalf("Chore Wheel did not consume canonical roster name=%q", got)
	}
	if err := fileio.WriteJSON(a.householdPeopleFile(), map[string]any{
		"schema": householdPeopleSchema,
		"people": []any{map[string]any{"id": "sam", "name": "Sam from Routines", "state": "archived"}},
	}); err != nil {
		t.Fatal(err)
	}
	paused := a.routinesPayload()
	if sessions := routinesOccurrencesForDay(paused, routinesToday()); len(sessions) != 0 {
		t.Fatalf("paused person still received future routine sessions: %#v", sessions)
	}
}

func TestArchivePersonEndpointPausesFutureRoutineSession(t *testing.T) {
	a := testProfileApp(t)
	disableCalendarCacheRefreshForTest(t, a)
	today := routinesToday()
	roster := map[string]any{
		"schema": householdPeopleSchema,
		"people": []any{map[string]any{"id": "sam", "name": "Sam", "state": "active"}},
	}
	if err := fileio.WriteJSON(a.householdPeopleFile(), roster); err != nil {
		t.Fatal(err)
	}
	if err := fileio.WriteJSON(a.routinesFile(), map[string]any{
		"people": jsonutil.List(roster["people"]),
		"routines": []any{map[string]any{
			"id": "morning", "title": "Morning",
			"steps": []any{map[string]any{"id": "step", "text": "Step"}},
			"assignments": []any{map[string]any{
				"id": "sam-am", "personId": "sam",
				"schedule": map[string]any{"kind": "days", "startOn": today},
			}},
		}},
	}); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/routines/people", nil)
	if !a.handleRoutinesPost(w, r, "/api/routines/people", map[string]any{"op": "archive", "id": "sam"}) {
		t.Fatal("archive endpoint was not handled")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("archive status=%d body=%s", w.Code, w.Body.String())
	}
	if sessions := routinesOccurrencesForDay(a.routinesPayload(), today); len(sessions) != 0 {
		t.Fatalf("archive endpoint left future sessions active: %#v", sessions)
	}
}

func TestArchivePersonReassignmentDoesNotDuplicateExistingTarget(t *testing.T) {
	a := testProfileApp(t)
	disableCalendarCacheRefreshForTest(t, a)
	today := routinesToday()
	roster := map[string]any{
		"schema": householdPeopleSchema,
		"people": []any{
			map[string]any{"id": "sam", "name": "Sam", "state": "active"},
			map[string]any{"id": "alex", "name": "Alex", "state": "active"},
		},
	}
	if err := fileio.WriteJSON(a.householdPeopleFile(), roster); err != nil {
		t.Fatal(err)
	}
	if err := fileio.WriteJSON(a.routinesFile(), map[string]any{
		"people": jsonutil.List(roster["people"]),
		"routines": []any{map[string]any{
			"id": "morning", "title": "Morning",
			"steps": []any{map[string]any{"id": "step", "text": "Step"}},
			"assignments": []any{
				map[string]any{"id": "sam-am", "personId": "sam", "schedule": map[string]any{"kind": "days", "startOn": today}},
				map[string]any{"id": "alex-am", "personId": "alex", "schedule": map[string]any{"kind": "days", "startOn": today}},
			},
		}},
	}); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/routines/people", nil)
	if !a.handleRoutinesPost(w, r, "/api/routines/people", map[string]any{"op": "archive", "id": "sam", "reassignTo": "alex"}) {
		t.Fatal("reassignment endpoint was not handled")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("reassignment status=%d body=%s", w.Code, w.Body.String())
	}
	payload := a.routinesPayload()
	_, routine := routinesFind(payload, "morning")
	if routine == nil {
		t.Fatal("routine disappeared")
	}
	assignments := jsonutil.List(routine["assignments"])
	if len(assignments) != 1 || routinesID(jsonutil.Map(assignments[0])["personId"]) != "alex" {
		t.Fatalf("reassignment assignments=%#v, want one Alex assignment", assignments)
	}
}
