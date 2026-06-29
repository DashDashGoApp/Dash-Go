package main

import (
	"net/http"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// handleHouseholdPeopleInboxPINPost runs through Dashboard Control's normal
// request/session guard. A personal inbox PIN is optional and may be set,
// changed, or removed without first enabling the separate Dashboard Control PIN.
// When a Dashboard Control PIN is configured, the normal Control session boundary
// still applies to this mutation route.
func (a *app) handleHouseholdPeopleInboxPINPost(w http.ResponseWriter, r *http.Request, path string, body map[string]any) bool {
	if path != "/api/household/people/inbox-pin/set" && path != "/api/household/people/inbox-pin/remove" {
		return false
	}
	personID := routinesID(body["personId"])
	_, person := householdPeopleFind(jsonutil.List(a.householdPeoplePayload()["people"]), personID)
	if person == nil {
		a.err(w, "household person was not found", http.StatusNotFound)
		return true
	}
	if path == "/api/household/people/inbox-pin/set" {
		if person["state"] != "active" {
			a.err(w, "restore this person before setting an inbox PIN", http.StatusConflict)
			return true
		}
		if err := a.setFamilyBoardInboxPIN(personID, jsonutil.BodyString(body, "pin")); err != nil {
			a.err(w, err.Error(), http.StatusBadRequest)
			return true
		}
		a.recordAction("people", "Set personal inbox PIN", "success", householdPersonAssignmentName(person), map[string]any{"personId": personID})
	} else {
		if err := a.removeFamilyBoardInboxPIN(personID); err != nil {
			a.err(w, err.Error(), http.StatusInternalServerError)
			return true
		}
		a.recordAction("people", "Removed personal inbox PIN", "success", householdPersonAssignmentName(person), map[string]any{"personId": personID})
	}
	a.json(w, a.householdPeopleControlPayload())
	return true
}
