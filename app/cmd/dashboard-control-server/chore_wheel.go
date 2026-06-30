package main

import (
	"net/http"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func (a *app) handleChoreWheelGet(w http.ResponseWriter, r *http.Request, path string) bool {
	switch path {
	case "/api/chore-wheel":
		a.json(w, a.choreWheelPayload())
		return true
	case "/api/chore-wheel/day":
		return a.handleChoreWheelDayGet(w, r)
	default:
		return false
	}
}

func (a *app) handleChoreWheelPost(w http.ResponseWriter, r *http.Request, path string, body map[string]any) bool {
	switch path {
	case "/api/chore-wheel/assignments/complete":
		return a.handleChoreWheelAssignmentComplete(w, body)
	case "/api/chore-wheel/assignments/status":
		return a.handleChoreWheelAssignmentStatus(w, body)
	}
	if path != "/api/chore-wheel" {
		return false
	}

	// Seed legacy Chore Wheel people before either mutation lock is held. The
	// actual write below always follows the shared People -> Chore -> calendar
	// order used by the central People reconciliation path.
	_ = a.choreWheelPayload()

	status, message := 0, ""
	var response map[string]any
	_ = a.householdService().WithLock(func() error {
		roster := a.householdPeoplePayload()
		return a.choreWheelService().WithLock(func() error {
			current := choreWheelPayloadForRoster(a.choreWheelService().Payload(), roster)
			expected := jsonutil.Int(body["revision"], -1)
			currentRevision := max(0, jsonutil.Int(current["revision"], 0))
			if expected != currentRevision {
				status, message = http.StatusConflict, "Chores changed elsewhere. Reloading the latest plan."
				return nil
			}
			payload := normalizeChoreWheelPayload(body)
			// The submitted Chore Wheel roster remains a deliberate entry point into
			// canonical People. Chores owns its model/planning state; People owns the
			// durable roster mutation while this ordered transaction is active.
			rosterPeople := jsonutil.List(roster["people"])
			submitted := map[string]map[string]any{}
			for _, raw := range jsonutil.List(payload["people"]) {
				row := jsonutil.Map(raw)
				submitted[choreWheelID(row["id"])] = row
			}
			currentPeople := map[string]bool{}
			for _, raw := range jsonutil.List(current["people"]) {
				currentPeople[choreWheelID(jsonutil.Map(raw)["id"])] = true
			}
			now := routinesNow().Format(time.RFC3339)
			for i, raw := range rosterPeople {
				person := jsonutil.Map(raw)
				id := routinesID(person["id"])
				if next := submitted[id]; next != nil {
					person["name"] = routinesText(next["name"], 64)
					person["state"] = "active"
					person["archivedAt"] = ""
					person["updatedAt"] = now
					rosterPeople[i] = person
					delete(submitted, id)
					continue
				}
				if currentPeople[id] {
					person["state"] = "archived"
					person["archivedAt"] = now
					person["updatedAt"] = now
					rosterPeople[i] = person
				}
			}
			for id, person := range submitted {
				if id != "" {
					rosterPeople = append(rosterPeople, map[string]any{"id": id, "name": routinesText(person["name"], 64), "state": "active", "createdAt": now, "updatedAt": now, "archivedAt": ""})
				}
			}
			roster["people"] = rosterPeople
			if err := a.writeHouseholdRoster(roster); err != nil {
				status, message = http.StatusInternalServerError, err.Error()
				return nil
			}
			payload = choreWheelPayloadForRoster(payload, roster)
			payload["revision"] = currentRevision + 1
			if err := a.commitChoreWheelPayload(payload); err != nil {
				status, message = http.StatusInternalServerError, err.Error()
				return nil
			}
			response = payload
			return nil
		})
	})
	if status != 0 {
		a.err(w, message, status)
		return true
	}
	a.json(w, response)
	return true
}
