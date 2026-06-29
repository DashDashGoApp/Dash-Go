package main

import (
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// Notification composition remains in core because Family Board and household
// ownership stay in package main until their own bounded-context extractions.
// internal/notify owns all private route storage, queueing, rate limits, and
// external delivery; core supplies only already-authorized event content.
func (a *app) notifyUrgentHouseholdMessage(note map[string]any) {
	if note["scope"] != "household" || note["priority"] != "urgent" || note["state"] != "active" {
		return
	}
	sender := familyBoardPersonSnapshot(note["senderNameSnapshot"])
	body := familyBoardText(note["text"], 320)
	if sender != "" {
		body = sender + ": " + body
	}
	for _, raw := range householdPeopleActive(a.householdPeoplePayload()) {
		person := jsonutil.Map(raw)
		personID := routinesID(person["id"])
		pref := a.apprisePersonPreferences(personID)
		if pref.UrgentHousehold && a.appriseConfiguredForPerson(personID) {
			a.enqueueAppriseEvent(appriseNotificationEvent{PersonID: personID, MessageID: familyBoardID(note["id"]), Title: "Dash-Go urgent household message", Body: body, Warning: true})
		}
	}
}

func (a *app) notifyPrivateFamilyMessage(note map[string]any) {
	if note["scope"] != "direct" || familyBoardStamp(note["withdrawnAt"]) != "" {
		return
	}
	personID := routinesID(note["recipientPersonId"])
	pref := a.apprisePersonPreferences(personID)
	if !pref.PrivateMessages || !a.appriseConfiguredForPerson(personID) {
		return
	}
	urgent := familyBoardPriority(note["priority"]) == "urgent"
	title, body := "Dash-Go private message", "You have a new private message in Dash-Go."
	if urgent {
		title, body = "Dash-Go urgent private message", "You have an urgent private message in Dash-Go."
	}
	if pref.PrivatePreviews {
		body = apprisePreviewText(familyBoardPersonSnapshot(note["senderNameSnapshot"]), familyBoardText(note["text"], 320))
	}
	a.enqueueAppriseEvent(appriseNotificationEvent{PersonID: personID, MessageID: familyBoardID(note["id"]), Private: true, Title: title, Body: body, Warning: urgent})
}
