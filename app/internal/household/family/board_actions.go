package family

import (
	"fmt"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// ActionError carries the established HTTP-facing status category without
// coupling the Family Board domain to an HTTP router. Core remains responsible
// for route matching and response serialization.
type ActionError struct {
	Status     int
	Message    string
	Lockout    bool
	RetryAfter int
}

func (e *ActionError) Error() string { return e.Message }

func actionError(status int, format string, args ...any) *ActionError {
	return &ActionError{Status: status, Message: fmt.Sprintf(format, args...)}
}

// MutateHousehold applies one public Family Board action as one serialized
// private-store transaction. The optional dispatch note is returned only after
// the durable write succeeds so core can retain existing urgent notification
// composition without exposing notification state to this package.
func (s *Service) MutateHousehold(action string, body map[string]any) (map[string]any, map[string]any, *ActionError) {
	s.Lock()
	defer s.Unlock()

	payload := s.PayloadLocked()
	settings := jsonutil.Map(payload["settings"])
	notes := jsonutil.List(payload["notes"])
	var dispatch map[string]any
	fail := func(status int, format string, args ...any) (map[string]any, map[string]any, *ActionError) {
		return nil, nil, actionError(status, format, args...)
	}
	save := func() (map[string]any, map[string]any, *ActionError) {
		payload["notes"] = notes
		payload["settings"] = settings
		payload = Normalize(payload, s.Now())
		if err := s.WriteLocked(payload); err != nil {
			return nil, nil, actionError(500, "%s", err)
		}
		return payload, dispatch, nil
	}

	switch action {
	case "acknowledge":
		idx, note := Find(payload, ID(body["id"]))
		if idx < 0 || note == nil || note["scope"] != "household" || note["state"] != "active" || note["priority"] != "urgent" {
			return fail(400, "unknown urgent household message")
		}
		note["householdAcknowledgedAt"] = ArchiveStamp(s.Now())
		note["updatedAt"] = note["householdAcknowledgedAt"]
		notes[idx] = note
		return save()
	case "settings":
		if _, ok := body["showUrgentAlertsOnDashboard"]; ok {
			settings["showUrgentAlertsOnDashboard"] = jsonutil.Truthy(body["showUrgentAlertsOnDashboard"])
		} else if _, ok := body["showPinnedOnDashboard"]; ok {
			settings["showUrgentAlertsOnDashboard"] = jsonutil.Truthy(body["showPinnedOnDashboard"])
		} else {
			return fail(400, "urgent dashboard alert setting is required")
		}
		delete(settings, "showPinnedOnDashboard")
		return save()
	case "add":
		if len(Active(payload)) >= 100 {
			return fail(400, "family note limit reached")
		}
		note := map[string]any{}
		if err := MutableNote(note, body, true, s.Now()); err != nil {
			return fail(400, "%s", err)
		}
		notes = append(notes, note)
		if note["priority"] == "urgent" {
			dispatch = note
		}
		return save()
	case "update":
		idx, note := Find(payload, ID(body["id"]))
		if idx < 0 || note == nil || note["scope"] != "household" {
			return fail(400, "unknown family note")
		}
		if note["state"] != "active" {
			return fail(400, "archived note must be restored before editing")
		}
		wasUrgent := note["priority"] == "urgent"
		if err := MutableNote(note, body, false, s.Now()); err != nil {
			return fail(400, "%s", err)
		}
		notes[idx] = note
		if !wasUrgent && note["priority"] == "urgent" {
			dispatch = note
		}
		return save()
	case "archive":
		idx, note := Find(payload, ID(body["id"]))
		if idx < 0 || note == nil || note["scope"] != "household" {
			return fail(400, "unknown family note")
		}
		note["state"] = "archived"
		note["archivedAt"] = ArchiveStamp(s.Now())
		note["updatedAt"] = note["archivedAt"]
		notes[idx] = note
		return save()
	case "restore":
		idx, note := Find(payload, ID(body["id"]))
		if idx < 0 || note == nil || note["scope"] != "household" {
			return fail(400, "unknown family note")
		}
		if note["state"] != "archived" {
			return fail(400, "family note is already active")
		}
		if err := MutableNote(note, body, false, s.Now()); err != nil {
			return fail(400, "%s", err)
		}
		note["state"] = "active"
		note["archivedAt"] = ""
		notes[idx] = note
		return save()
	case "delete":
		idx, note := Find(payload, ID(body["id"]))
		if idx < 0 || note == nil || note["scope"] != "household" {
			return fail(400, "unknown family note")
		}
		notes = append(notes[:idx], notes[idx+1:]...)
		return save()
	default:
		return fail(400, "unknown family board action")
	}
}
