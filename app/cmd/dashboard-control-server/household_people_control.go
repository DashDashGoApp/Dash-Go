package main

import (
	"maps"
	"net/http"
	"slices"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// household_people_control.go owns the Dashboard Control People surface.
// The canonical roster remains local-first and is shared by household apps;
// app-specific task/calendar data stays in its existing domain file.
func householdPeopleControlRow(person map[string]any, impact map[string]int, inboxPinConfigured bool, notifications map[string]any) map[string]any {
	row := make(map[string]any, len(person))
	maps.Copy(row, person)
	row["impact"] = impact
	row["inboxPinConfigured"] = inboxPinConfigured
	row["notifications"] = notifications
	return row
}

func (a *app) householdPeopleControlPayload() map[string]any {
	roster := a.householdPeoplePayload()
	people := make([]any, 0, len(jsonutil.List(roster["people"])))
	pins := jsonutil.Map(a.familyBoardInboxPinsPayload()["pins"])
	for _, raw := range jsonutil.List(roster["people"]) {
		person := jsonutil.Map(raw)
		_, pinSet := pins[routinesID(person["id"])]
		people = append(people, householdPeopleControlRow(person, a.householdPeopleImpactForRoster(routinesID(person["id"]), roster), pinSet, a.apprisePersonControlStatus(routinesID(person["id"]))))
	}
	slices.SortStableFunc(people, func(left, right any) int {
		return compareFoldedText(householdPersonAssignmentName(jsonutil.Map(left)), householdPersonAssignmentName(jsonutil.Map(right)))
	})
	return map[string]any{
		"schema": householdPeopleSchema, "revision": max(0, jsonutil.Int(roster["revision"], 0)), "people": people,
		"note": "People are shared by Chore Wheel, Routines, To Do, Grocery, Maintenance, and Family Message Board.",
	}
}

func (a *app) handleHouseholdPeopleGet(w http.ResponseWriter, r *http.Request, path string) bool {
	if path != "/api/household/people" {
		return false
	}
	a.json(w, a.householdPeopleControlPayload())
	return true
}

func (a *app) handleHouseholdPeoplePost(w http.ResponseWriter, r *http.Request, path string, body map[string]any) bool {
	if path != "/api/household/people" {
		return false
	}

	status := 0
	message := ""
	action := "Updated household person"
	personID := ""
	reassignTo := ""
	if err := a.householdService().WithLock(func() error {
		current := a.householdPeoplePayload()
		next, op, id, err := a.householdService().NextRoster(current, body)
		if err != nil {
			status, message = http.StatusBadRequest, err.Error()
			return nil
		}
		target := ""
		if op == "delete" {
			if messages := a.familyBoardPersonMessageCount(id); messages > 0 {
				status = http.StatusConflict
				message = "archive this person instead: their private Family Message Board history is preserved and cannot be reassigned"
				return nil
			}
			target, err = householdPeopleDeleteTarget(next, body, id)
			if err != nil {
				status, message = http.StatusBadRequest, err.Error()
				return nil
			}
		}
		// Reconcile derived future work before committing the canonical roster.
		// Each domain retains its own completed/history snapshots; only future/open
		// work is reassigned or made unassigned when a person is permanently removed.
		if err := a.reconcileHouseholdPeople(next, op, id, target); err != nil {
			status, message = http.StatusInternalServerError, err.Error()
			return nil
		}
		if err := a.householdService().Write(next); err != nil {
			status, message = http.StatusInternalServerError, err.Error()
			return nil
		}
		if op == "archive" || op == "delete" {
			a.revokeFamilyBoardInboxSessions(id)
		}
		if op == "delete" {
			if err := a.removeFamilyBoardInboxPIN(id); err != nil {
				status, message = http.StatusInternalServerError, "person was removed but inbox PIN cleanup failed: "+err.Error()
				return nil
			}
		}
		switch op {
		case "add":
			action = "Added household person"
		case "rename":
			action = "Renamed household person"
		case "archive":
			action = "Archived household person"
		case "restore":
			action = "Restored household person"
		case "delete":
			action = "Removed household person"
		}
		personID, reassignTo = id, target
		return nil
	}); err != nil && status == 0 {
		status, message = http.StatusInternalServerError, err.Error()
	}
	if status != 0 {
		a.err(w, message, status)
		return true
	}

	// The People lock protects roster mutation and dependent future-work
	// reconciliation only. Build the post-mutation control payload after the
	// lock is released: impact projections can read Routines/To Do/Families but
	// must never re-enter the non-reentrant People mutex.
	a.recordAction("people", action, "success", time.Now().In(time.Local).Format("Jan 2, 3:04 PM"), map[string]any{"personId": personID, "reassignTo": reassignTo})
	a.json(w, a.householdPeopleControlPayload())
	return true
}
