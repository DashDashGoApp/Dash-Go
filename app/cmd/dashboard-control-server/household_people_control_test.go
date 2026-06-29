package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func TestHouseholdPeopleControlDeleteReassignsOpenLocalWork(t *testing.T) {
	a := testApp(t)
	disableCalendarCacheRefreshForTest(t, a)
	writePeopleAssignmentFixture(t, a,
		map[string]any{"id": "sam", "name": "Sam", "state": "active"},
		map[string]any{"id": "alex", "name": "Alex", "state": "active"},
	)
	if err := fileio.WriteJSON(a.routinesFile(), map[string]any{
		"people":   []any{map[string]any{"id": "sam", "name": "Sam"}, map[string]any{"id": "alex", "name": "Alex"}},
		"routines": []any{map[string]any{"id": "morning", "title": "Morning", "steps": []any{map[string]any{"id": "step", "text": "Step"}}, "assignments": []any{map[string]any{"id": "sam-am", "personId": "sam", "schedule": map[string]any{"kind": "days", "startOn": routinesToday()}}}}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.writeTodoListCache(todoListCache{Version: 1, ListID: todoLocalGroceryListID, Tasks: []todoTask{{ID: "milk", Title: "Milk", Status: "notStarted", DashGoAssignment: &todoTaskAssignment{PersonID: "sam", PersonNameSnapshot: "Sam"}}}}); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/household/people", nil)
	completed := make(chan bool, 1)
	go func() {
		completed <- a.handleHouseholdPeoplePost(w, r, "/api/household/people", map[string]any{"op": "delete", "id": "sam", "resolution": "reassign", "reassignTo": "alex"})
	}()
	select {
	case handled := <-completed:
		if !handled {
			t.Fatal("People control route was not handled")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("People deletion must finish without re-entering the household roster lock while building its response")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("delete status=%d body=%s", w.Code, w.Body.String())
	}
	if _, person := householdPeopleFind(jsonutil.List(a.householdPeoplePayload()["people"]), "sam"); person != nil {
		t.Fatalf("removed person remained in canonical roster: %#v", a.householdPeoplePayload())
	}
	_, routine := routinesFind(a.routinesPayload(), "morning")
	assignments := jsonutil.List(routine["assignments"])
	if len(assignments) != 1 || routinesID(jsonutil.Map(assignments[0])["personId"]) != "alex" {
		t.Fatalf("future routine reassignment=%#v, want one Alex assignment", assignments)
	}
	cache := a.readTodoListCache(todoLocalGroceryListID)
	if len(cache.Tasks) != 1 || cache.Tasks[0].DashGoAssignment == nil || cache.Tasks[0].DashGoAssignment.PersonID != "alex" {
		t.Fatalf("open Grocery assignment was not reassigned locally: %#v", cache.Tasks)
	}
	if len(cache.PendingOps) != 0 {
		t.Fatalf("People reassignment must not enqueue Microsoft work: %#v", cache.PendingOps)
	}
}

func TestHouseholdPeopleImpactForRosterDoesNotReenterPeopleLock(t *testing.T) {
	a := testApp(t)
	writePeopleAssignmentFixture(t, a, map[string]any{"id": "sam", "name": "Sam", "state": "active"})
	completed := make(chan error, 1)
	go func() {
		completed <- a.householdService().WithLock(func() error {
			impact := a.householdPeopleImpactForRoster("sam", a.householdPeoplePayload())
			if impact == nil {
				return errors.New("People impact must return a complete zero/default count map")
			}
			return nil
		})
	}()
	select {
	case err := <-completed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("People impact must use its supplied roster and must not re-enter the household roster lock")
	}
}

func TestHouseholdPeopleControlRejectsDeleteWithoutValidTarget(t *testing.T) {
	a := testApp(t)
	writePeopleAssignmentFixture(t, a, map[string]any{"id": "sam", "name": "Sam", "state": "active"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/household/people", nil)
	if !a.handleHouseholdPeoplePost(w, r, "/api/household/people", map[string]any{"op": "delete", "id": "sam", "resolution": "reassign", "reassignTo": "sam"}) {
		t.Fatal("People control route was not handled")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("bad reassignment status=%d body=%s", w.Code, w.Body.String())
	}
	if _, person := householdPeopleFind(jsonutil.List(a.householdPeoplePayload()["people"]), "sam"); person == nil {
		t.Fatal("failed deletion changed the canonical roster")
	}
}

func TestHouseholdPeopleInboxPINCanBeManagedWithoutMasterControlPIN(t *testing.T) {
	a := testApp(t)
	writePeopleAssignmentFixture(t, a, map[string]any{"id": "sam", "name": "Sam", "state": "active"})
	if a.lockConfig()["enabled"] == true {
		t.Fatal("test fixture must start without a Dashboard Control PIN")
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/household/people/inbox-pin/set", nil)
	if !a.handleHouseholdPeopleInboxPINPost(w, r, "/api/household/people/inbox-pin/set", map[string]any{"personId": "sam", "pin": "1234"}) {
		t.Fatal("personal inbox PIN route was not handled")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("set personal inbox PIN without master Control PIN: status=%d body=%s", w.Code, w.Body.String())
	}
	if !a.verifyFamilyBoardInboxPIN("sam", "1234") {
		t.Fatal("personal inbox PIN was not saved without a master Control PIN")
	}

	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/api/household/people/inbox-pin/remove", nil)
	if !a.handleHouseholdPeopleInboxPINPost(w, r, "/api/household/people/inbox-pin/remove", map[string]any{"personId": "sam"}) {
		t.Fatal("personal inbox PIN removal route was not handled")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("remove personal inbox PIN without master Control PIN: status=%d body=%s", w.Code, w.Body.String())
	}
	if !a.verifyFamilyBoardInboxPIN("sam", "any-value") {
		t.Fatal("removed personal inbox PIN should leave inbox available")
	}
}
