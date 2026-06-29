package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func TestChoreCalendarDayProjectionAndDirectCompletion(t *testing.T) {
	a := testProfileApp(t)
	disableCalendarCacheRefreshForTest(t, a)
	oldClock := choreWheelClock
	choreWheelClock = func() time.Time { return time.Date(2026, 6, 24, 9, 0, 0, 0, time.Local) }
	t.Cleanup(func() { choreWheelClock = oldClock })
	payload := normalizeChoreWheelPayload(map[string]any{
		"people": []any{map[string]any{"id": "jason", "name": "Jason"}},
		"chores": []any{map[string]any{"id": "litter", "name": "Kitty Litter"}},
		"assignments": []any{
			map[string]any{"id": "today", "date": "2026-06-24", "choreId": "litter", "choreName": "Kitty Litter", "personId": "jason", "personName": "Jason", "status": "assigned"},
			map[string]any{"id": "future", "date": "2026-06-25", "choreId": "litter", "choreName": "Kitty Litter", "personId": "jason", "personName": "Jason", "status": "assigned"},
		},
	})
	if err := fileio.WriteJSON(a.choreWheelFile(), payload); err != nil {
		t.Fatal(err)
	}
	day := choreWheelDayResponse(a.choreWheelPayload(), "2026-06-24")
	items := jsonutil.List(day["items"])
	if len(items) != 1 || !jsonutil.Truthy(jsonutil.Map(items[0])["actionable"]) {
		t.Fatalf("today chore day response = %#v", day)
	}
	w := httptest.NewRecorder()
	if !a.handleChoreWheelAssignmentComplete(w, map[string]any{"assignmentId": "today", "date": "2026-06-24"}) {
		t.Fatal("completion handler not handled")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("completion status=%d body=%s", w.Code, w.Body.String())
	}
	updated := a.choreWheelPayload()
	if status := choreWheelText(jsonutil.Map(jsonutil.List(updated["assignments"])[0])["status"], 16); status != "completed" {
		t.Fatalf("direct completion did not persist: %#v", updated)
	}
	future := httptest.NewRecorder()
	if !a.handleChoreWheelAssignmentComplete(future, map[string]any{"assignmentId": "future", "date": "2026-06-25"}) {
		t.Fatal("future completion handler not handled")
	}
	if future.Code != http.StatusBadRequest {
		t.Fatalf("future completion code=%d body=%s", future.Code, future.Body.String())
	}
}

func TestMaintenanceCalendarDayProjectionAndDirectCompletion(t *testing.T) {
	a := testProfileApp(t)
	disableCalendarCacheRefreshForTest(t, a)
	oldClock := maintenanceClock
	maintenanceClock = func() time.Time { return time.Date(2026, 6, 24, 9, 0, 0, 0, time.Local) }
	t.Cleanup(func() { maintenanceClock = oldClock })
	payload := normalizeMaintenancePayload(map[string]any{"tasks": []any{
		map[string]any{"id": "filter", "title": "Replace HVAC filter", "state": "active", "cadence": map[string]any{"unit": "months", "every": 3}, "nextDueOn": "2026-06-24", "calendarEnabled": true},
		map[string]any{"id": "future", "title": "Future task", "state": "active", "cadence": map[string]any{"unit": "months", "every": 1}, "nextDueOn": "2026-06-25", "calendarEnabled": true},
	}})
	if err := fileio.WriteJSON(a.maintenanceFile(), payload); err != nil {
		t.Fatal(err)
	}
	day := maintenanceDayResponse(a.maintenancePayload(), "2026-06-24")
	if items := jsonutil.List(day["items"]); len(items) != 1 || !jsonutil.Truthy(jsonutil.Map(items[0])["actionable"]) {
		t.Fatalf("maintenance day response=%#v", day)
	}
	w := httptest.NewRecorder()
	if !a.handleMaintenancePost(w, httptest.NewRequest(http.MethodPost, "/api/maintenance/tasks/complete", nil), "/api/maintenance/tasks/complete", map[string]any{"id": "filter", "completedOn": "2026-06-24"}) {
		t.Fatal("maintenance completion not handled")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("maintenance completion status=%d body=%s", w.Code, w.Body.String())
	}
	updated := a.maintenancePayload()
	_, filter := maintenanceFind(updated, "filter")
	if filter == nil || maintenanceDate(filter["nextDueOn"]) != "2026-09-24" {
		t.Fatalf("maintenance next due not recalculated: %#v", updated)
	}
	future := httptest.NewRecorder()
	if !a.handleMaintenancePost(future, httptest.NewRequest(http.MethodPost, "/api/maintenance/tasks/complete", nil), "/api/maintenance/tasks/complete", map[string]any{"id": "future", "completedOn": "2026-06-24"}) {
		t.Fatal("future completion not handled")
	}
	if future.Code != http.StatusBadRequest {
		t.Fatalf("future maintenance completion code=%d body=%s", future.Code, future.Body.String())
	}
}

func TestRoutineOccurrenceRejectsFutureCalendarMutation(t *testing.T) {
	a := testProfileApp(t)
	oldClock := routinesClock
	routinesClock = func() time.Time { return time.Date(2026, 6, 24, 9, 0, 0, 0, time.Local) }
	t.Cleanup(func() { routinesClock = oldClock })
	payload := normalizeRoutinesPayload(map[string]any{
		"people":   []any{map[string]any{"id": "sam", "name": "Sam"}},
		"routines": []any{map[string]any{"id": "morning", "title": "Morning", "steps": []any{map[string]any{"id": "teeth", "text": "Brush teeth"}}, "assignments": []any{map[string]any{"id": "sam-am", "personId": "sam", "schedule": map[string]any{"kind": "days", "startOn": "2026-06-01"}}}}},
	})
	if err := fileio.WriteJSON(a.routinesFile(), payload); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	if !a.handleRoutinesPost(w, httptest.NewRequest(http.MethodPost, "/api/routines/occurrence", nil), "/api/routines/occurrence", map[string]any{"op": "complete", "routineId": "morning", "assignmentId": "sam-am", "date": "2026-06-25"}) {
		t.Fatal("future routine mutation not handled")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("future routine completion code=%d body=%s", w.Code, w.Body.String())
	}
}
