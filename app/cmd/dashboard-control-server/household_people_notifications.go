package main

import (
	"net/http"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// Notification preferences are non-secret local People metadata. Destination
// URLs remain in the owner-only Apprise route store and never cross this API.
func (a *app) handleHouseholdPeopleNotificationPost(w http.ResponseWriter, r *http.Request, path string, body map[string]any) bool {
	if path != "/api/household/people/notifications" {
		return false
	}
	personID := routinesID(body["personId"])
	person, ok := a.familyBoardActivePerson(personID)
	if !ok || person == nil {
		a.err(w, "active household person was not found", http.StatusNotFound)
		return true
	}
	if !a.appriseConfiguredForPerson(personID) {
		a.err(w, "set a private delivery route through Installer > Notifications (Apprise-Go) first", http.StatusConflict)
		return true
	}
	pref := apprisePersonPreferences{
		UrgentHousehold: jsonutil.Truthy(body["urgentHousehold"]),
		PrivateMessages: jsonutil.Truthy(body["privateMessages"]),
		PrivatePreviews: jsonutil.Truthy(body["privatePreviews"]),
	}
	if err := a.setApprisePersonPreferences(personID, pref); err != nil {
		a.err(w, "could not save external notification preferences", http.StatusInternalServerError)
		return true
	}
	a.recordAction("people", "Update external notification preferences", "success", householdPersonAssignmentName(person), map[string]any{"personId": personID})
	a.json(w, a.householdPeopleControlPayload())
	return true
}
