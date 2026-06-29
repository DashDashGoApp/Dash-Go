package main

import (
	"net/http"
	"strings"

	routinespkg "github.com/DashDashGoApp/Dash-Go/app/internal/household/routines"
)

// handleRoutinesPost retains only route/status adaptation. Routines owns every
// domain mutation. The legacy People route deliberately acquires People before
// Routines, matching the one global household mutation order.
func (a *app) handleRoutinesPost(w http.ResponseWriter, r *http.Request, path string, body map[string]any) bool {
	if !strings.HasPrefix(path, "/api/routines") {
		return false
	}
	switch path {
	case "/api/routines/settings", "/api/routines/items", "/api/routines/occurrence":
		// Read the canonical roster before taking the Routines lock. The child
		// service receives the resulting data, never a callback into People.
		roster := a.householdPeoplePayload()
		_ = a.routinesService().WithLock(func() error { return a.handleRoutinesServiceMutation(w, path, body, roster) })
		return true
	case "/api/routines/people":
		_ = a.householdService().WithLock(func() error {
			return a.routinesService().WithLock(func() error { return a.handleRoutinesLegacyPeopleMutation(w, body) })
		})
		return true
	default:
		a.err(w, "unknown routines endpoint", http.StatusNotFound)
		return true
	}
}
func (a *app) handleRoutinesServiceMutation(w http.ResponseWriter, path string, body, roster map[string]any) error {
	payload := routinespkg.ProjectRoster(a.routinesService().Payload(), roster, routinesNow())
	var result routinespkg.MutationResult
	var err error
	switch path {
	case "/api/routines/settings":
		result = routinespkg.ApplySettings(payload, body, routinesNow())
	case "/api/routines/items":
		result, err = routinespkg.ApplyItem(payload, body, routinesNow())
	case "/api/routines/occurrence":
		result, err = routinespkg.ApplyOccurrence(payload, body, routinesNow())
	}
	if err != nil {
		a.err(w, err.Error(), routinespkg.Status(err))
		return nil
	}
	if result.StateOnly {
		err = a.saveRoutinesStateOnly(result.Payload)
	} else {
		err = a.commitRoutinesPayload(result.Payload)
	}
	if err != nil {
		a.err(w, err.Error(), http.StatusInternalServerError)
		return nil
	}
	response := routinesResponse(result.Payload)
	if result.DayDate != "" {
		response["day"] = routinesDayResponse(result.Payload, result.DayDate)
	}
	a.json(w, response)
	return nil
}
func (a *app) handleRoutinesLegacyPeopleMutation(w http.ResponseWriter, body map[string]any) error {
	payload := a.routinesService().Payload()
	roster := a.householdPeoplePayload()
	next, nextRoster, err := routinespkg.ApplyLegacyPeopleAction(payload, roster, body, routinesNow())
	if err != nil {
		a.err(w, err.Error(), routinespkg.Status(err))
		return nil
	}
	if err := a.householdService().Write(nextRoster); err != nil {
		a.err(w, err.Error(), http.StatusInternalServerError)
		return nil
	}
	if err := a.commitRoutinesPayload(next); err != nil {
		a.err(w, err.Error(), http.StatusInternalServerError)
		return nil
	}
	response := routinesResponse(next)
	if date := routinesDate(body["dayDate"]); date != "" {
		response["day"] = routinesDayResponse(next, date)
	}
	a.json(w, response)
	return nil
}
