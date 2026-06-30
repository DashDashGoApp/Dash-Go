package main

import (
	"errors"
	"net/http"

	chorepkg "github.com/DashDashGoApp/Dash-Go/app/internal/household/chores"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func (a *app) handleChoreWheelDayGet(w http.ResponseWriter, r *http.Request) bool {
	date := choreWheelDateKey(r.URL.Query().Get("date"))
	if date == "" {
		a.err(w, "date must be YYYY-MM-DD", http.StatusBadRequest)
		return true
	}
	a.json(w, choreWheelDayResponse(a.choreWheelPayload(), date))
	return true
}

// handleChoreWheelAssignmentCompletion is intentionally narrow: the calendar
// can toggle an eligible historical/current assignment between assigned and
// completed, but cannot reassign, skip, remove, or overwrite the full Chore
// Wheel payload.
func (a *app) handleChoreWheelAssignmentCompletion(w http.ResponseWriter, body map[string]any, completed bool) bool {
	assignmentID := choreWheelID(body["assignmentId"])
	date := choreWheelDateKey(body["date"])
	if assignmentID == "" || date == "" {
		a.err(w, "assignment and date are required", http.StatusBadRequest)
		return true
	}
	status, message := 0, ""
	var response map[string]any
	_ = a.choreWheelService().WithLock(func() error {
		// Day completion changes an existing durable assignment only. It must not
		// call the roster-projection helper while the Chore lock is held.
		payload := a.choreWheelService().Payload()
		next, changed, err := a.choreWheelService().SetAssignmentCompleted(payload, assignmentID, date, completed)
		if err != nil {
			switch {
			case errors.Is(err, chorepkg.ErrFutureMutation), errors.Is(err, chorepkg.ErrAssignmentAndDate), errors.Is(err, chorepkg.ErrAssignmentStatus):
				status, message = http.StatusBadRequest, err.Error()
			case errors.Is(err, chorepkg.ErrAssignmentMissing):
				status, message = http.StatusNotFound, err.Error()
			default:
				status, message = http.StatusInternalServerError, err.Error()
			}
			return nil
		}
		if !changed {
			response = map[string]any{"state": next, "day": choreWheelDayResponse(next, date)}
			return nil
		}
		if err := a.commitChoreWheelPayload(next); err != nil {
			status, message = http.StatusInternalServerError, err.Error()
			return nil
		}
		response = map[string]any{"state": next, "day": choreWheelDayResponse(next, date)}
		return nil
	})
	if status != 0 {
		a.err(w, message, status)
		return true
	}
	a.json(w, response)
	return true
}

// handleChoreWheelAssignmentComplete keeps the pre-beta.4 endpoint stable for
// callers that only need completion. The new status endpoint below is used by
// the reversible checkbox UI.
func (a *app) handleChoreWheelAssignmentComplete(w http.ResponseWriter, body map[string]any) bool {
	return a.handleChoreWheelAssignmentCompletion(w, body, true)
}

func (a *app) handleChoreWheelAssignmentStatus(w http.ResponseWriter, body map[string]any) bool {
	return a.handleChoreWheelAssignmentCompletion(w, body, jsonutil.Truthy(body["completed"]))
}
