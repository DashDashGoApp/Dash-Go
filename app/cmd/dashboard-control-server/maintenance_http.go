package main

import (
	"net/http"
	"strings"

	maintenancepkg "github.com/DashDashGoApp/Dash-Go/app/internal/household/maintenance"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func (a *app) maintenanceResponse(payload map[string]any) map[string]any {
	return map[string]any{"state": payload, "summary": maintenanceSummary(payload), "people": jsonutil.List(a.householdPeoplePayload()["people"])}
}
func (a *app) handleMaintenanceGet(w http.ResponseWriter, r *http.Request, path string) bool {
	switch path {
	case "/api/maintenance":
		a.json(w, a.maintenanceResponse(a.maintenancePayload()))
		return true
	case "/api/maintenance/day":
		return a.handleMaintenanceDayGet(w, r)
	default:
		return false
	}
}
func (a *app) handleMaintenancePost(w http.ResponseWriter, r *http.Request, path string, body map[string]any) bool {
	if !strings.HasPrefix(path, "/api/maintenance/") {
		return false
	}
	switch path {
	case "/api/maintenance/settings", "/api/maintenance/tasks/add", "/api/maintenance/tasks/update", "/api/maintenance/tasks/complete", "/api/maintenance/tasks/reschedule", "/api/maintenance/tasks/archive", "/api/maintenance/tasks/restore", "/api/maintenance/tasks/delete":
	default:
		return false
	}
	activePeople := map[string]string{}
	for _, raw := range householdPeopleActive(a.householdPeoplePayload()) {
		person := jsonutil.Map(raw)
		if id := routinesID(person["id"]); id != "" {
			activePeople[id] = householdPersonAssignmentName(person)
		}
	}
	handled := true
	_ = a.maintenanceService().WithLock(func() error {
		result, err := maintenancepkg.Apply(a.maintenanceService().Payload(), path, body, maintenanceNow(), func(id string) (string, bool) {
			name, ok := activePeople[routinesID(id)]
			return name, ok && name != ""
		})
		if err != nil {
			a.err(w, err.Error(), http.StatusBadRequest)
			return nil
		}
		if err := a.commitMaintenancePayload(result.Payload); err != nil {
			a.err(w, err.Error(), http.StatusInternalServerError)
			return nil
		}
		response := a.maintenanceResponse(result.Payload)
		for key, value := range result.Extra {
			response[key] = value
		}
		a.json(w, response)
		return nil
	})
	return handled
}
