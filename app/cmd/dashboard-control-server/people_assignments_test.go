package main

import (
	"strings"
	"testing"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func writePeopleAssignmentFixture(t *testing.T, a *app, entries ...map[string]any) {
	t.Helper()
	people := make([]any, 0, len(entries))
	for _, entry := range entries {
		people = append(people, entry)
	}
	if err := fileio.WriteJSON(a.householdPeopleFile(), map[string]any{"schema": householdPeopleSchema, "revision": 1, "people": people}); err != nil {
		t.Fatal(err)
	}
}

func TestTodoPeopleAssignmentStaysLocalAndSurvivesGraphRefresh(t *testing.T) {
	a := newTodoTestApp(t)
	writePeopleAssignmentFixture(t, a,
		map[string]any{"id": "jason", "name": "Jason", "state": "active"},
		map[string]any{"id": "sam", "name": "Sam", "state": "archived"},
	)
	enableTodoMicrosoftTestList(t, a, "remote")
	if err := a.writeTodoListCache(todoListCache{Version: 1, ListID: "remote", Tasks: []todoTask{{ID: "milk", Title: "Milk", Status: "notStarted"}}}); err != nil {
		t.Fatal(err)
	}

	cache, err := a.todoAssignTaskPerson("remote", "milk", "jason")
	if err != nil {
		t.Fatal(err)
	}
	if len(cache.PendingOps) != 0 || cache.Tasks[0].DashGoAssignment == nil {
		t.Fatalf("Dash-Go-only assignment queued cloud work or was not saved: %#v", cache)
	}
	if got := cache.Tasks[0].DashGoAssignment.PersonNameSnapshot; got != "Jason" {
		t.Fatalf("assignment snapshot=%q, want Jason", got)
	}
	if _, err := a.todoAssignTaskPerson("remote", "milk", "sam"); err == nil {
		t.Fatal("archived people must not be assignable to new responsibility")
	}

	fromPhone := todoTaskPatchFromGraph(cache.Tasks[0], map[string]any{"id": "milk", "title": "Whole milk", "status": "notStarted"})
	if fromPhone.DashGoAssignment == nil || fromPhone.DashGoAssignment.PersonID != "jason" {
		t.Fatalf("phone-originated Graph patch lost local responsibility: %#v", fromPhone)
	}
	outbound := todoTaskGraphPatchBody(map[string]any{"title": "Whole milk", "dashgoAssignment": map[string]any{"personId": "jason"}})
	if _, leaks := outbound["dashgoAssignment"]; leaks || outbound["title"] != "Whole milk" {
		t.Fatalf("Graph patch must omit Dash-Go assignment metadata: %#v", outbound)
	}
	if got := a.todoTaskAssignmentName(todoTask{DashGoAssignment: &todoTaskAssignment{PersonID: "sam", PersonNameSnapshot: "Sam"}}); got != "Former: Sam" {
		t.Fatalf("archived person label=%q, want Former: Sam", got)
	}
}

func TestMaintenancePeopleAssignmentCarriesToHistoryAndCalendar(t *testing.T) {
	a := testProfileApp(t)
	writePeopleAssignmentFixture(t, a,
		map[string]any{"id": "jason", "name": "Jason", "state": "active"},
		map[string]any{"id": "sam", "name": "Sam", "state": "archived"},
	)
	defaults := maintenanceDefault()["settings"].(map[string]any)
	body := map[string]any{
		"title": "Replace HVAC filter", "cadence": map[string]any{"unit": "months", "every": 3}, "nextDueOn": "2026-09-01",
		"calendarEnabled": true, "responsiblePersonId": "jason",
	}
	task, err := a.maintenanceTaskFromBody(body, defaults, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := maintenanceTaskPersonSnapshot(task); got != "Jason" {
		t.Fatalf("maintenance responsibility snapshot=%q, want Jason", got)
	}
	if !strings.Contains(maintenanceCalendarEvents(map[string]any{"tasks": []any{task}})[0].Description, "Responsible: Jason") {
		t.Fatalf("maintenance calendar omitted responsibility: %#v", maintenanceCalendarEvents(map[string]any{"tasks": []any{task}})[0])
	}
	payload := map[string]any{"history": []any{}}
	maintenanceAddHistory(payload, task, "completed", "2026-09-01", "2026-09-01", "2026-12-01")
	history := jsonutil.Map(jsonutil.List(payload["history"])[0])
	if got := maintenanceTaskPersonSnapshot(history); got != "Jason" {
		t.Fatalf("maintenance history lost responsibility snapshot: %#v", history)
	}

	former := map[string]any{"id": "old-task", "title": "Old task", "note": "", "state": "active", "cadence": map[string]any{"unit": "months", "every": 1}, "nextDueOn": "2026-09-01", "calendarEnabled": true, "responsiblePersonId": "sam", "responsiblePersonNameSnapshot": "Sam"}
	preserved, err := a.maintenanceTaskFromBody(map[string]any{"title": "Old task", "cadence": map[string]any{"unit": "months", "every": 1}, "nextDueOn": "2026-09-01", "calendarEnabled": true, "responsiblePersonId": "sam"}, defaults, former)
	if err != nil || maintenanceTaskPersonSnapshot(preserved) != "Sam" {
		t.Fatalf("editing an existing former assignment must preserve history context: task=%#v err=%v", preserved, err)
	}
	if _, err := a.maintenanceTaskFromBody(map[string]any{"title": "New task", "cadence": map[string]any{"unit": "months", "every": 1}, "nextDueOn": "2026-09-01", "responsiblePersonId": "sam"}, defaults, nil); err == nil {
		t.Fatal("new maintenance assignments may not select archived people")
	}
}
