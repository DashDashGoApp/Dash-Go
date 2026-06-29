package routines

import (
	"path/filepath"
	"testing"
	"time"
)

func routineNow() time.Time { return time.Date(2026, 6, 24, 9, 0, 0, 0, time.Local) }
func routinePayload() map[string]any {
	return map[string]any{"people": []any{map[string]any{"id": "sam", "name": "Sam", "state": "active"}, map[string]any{"id": "alex", "name": "Alex", "state": "active"}}, "routines": []any{map[string]any{"id": "morning", "title": "Morning", "steps": []any{map[string]any{"id": "teeth", "text": "Brush"}}, "assignments": []any{map[string]any{"id": "sam-am", "personId": "sam", "schedule": map[string]any{"kind": "days", "startOn": "2026-06-01"}}}}}}
}
func TestOccurrenceCompletionAndCalendarProjection(t *testing.T) {
	payload := routinePayload()
	result, err := ApplyOccurrence(payload, map[string]any{"op": "complete", "routineId": "morning", "assignmentId": "sam-am", "date": "2026-06-24"}, routineNow())
	if err != nil {
		t.Fatal(err)
	}
	if !result.StateOnly {
		t.Fatal("ordinary completion should keep the existing state-only calendar optimization")
	}
	day := DayResponse(result.Payload, "2026-06-24", routineNow())
	if day["count"] != 1 {
		t.Fatalf("day=%#v", day)
	}
	events := CalendarEvents(result.Payload, routineNow())
	if len(events) == 0 || events[0].AppOwner != "routines" {
		t.Fatalf("events=%#v", events)
	}
}
func TestReconcileDeleteReassignsBeforeRosterNormalization(t *testing.T) {
	payload := routinePayload()
	roster := map[string]any{"people": []any{map[string]any{"id": "alex", "name": "Alex", "state": "active"}}}
	next, changed := ReconcilePeople(payload, roster, "delete", "sam", "alex", routineNow())
	if !changed {
		t.Fatal("expected correction")
	}
	_, routine := Find(next, "morning")
	assignments := routine["assignments"].([]any)
	if len(assignments) != 1 || ID(assignments[0].(map[string]any)["personId"]) != "alex" {
		t.Fatalf("assignments=%#v", assignments)
	}
}
func TestServiceWritesNormalizedDocument(t *testing.T) {
	dir := t.TempDir()
	s := New(ServiceConfig{ConfigDir: dir, Now: routineNow})
	if err := s.Write(routinePayload()); err != nil {
		t.Fatal(err)
	}
	if s.File() != filepath.Join(dir, "routines.json") {
		t.Fatalf("file=%q", s.File())
	}
	if len(s.Payload()["routines"].([]any)) != 1 {
		t.Fatalf("payload=%#v", s.Payload())
	}
}
