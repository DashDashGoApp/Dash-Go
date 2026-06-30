package maintenance

import (
	"path/filepath"
	"testing"
	"time"
)

func fixedNow() time.Time { return time.Date(2026, 6, 24, 9, 0, 0, 0, time.Local) }
func TestApplyCompletionRetainsCalendarAndHistoryContract(t *testing.T) {
	payload := map[string]any{"settings": map[string]any{"defaultCalendarEnabled": true, "calendarOutputEnabled": true}, "tasks": []any{map[string]any{"id": "filter", "title": "Filter", "cadence": map[string]any{"unit": "months", "every": 3}, "nextDueOn": "2026-06-24", "calendarEnabled": true, "state": "active"}}}
	result, err := Apply(payload, "/api/maintenance/tasks/complete", map[string]any{"id": "filter", "completedOn": "2026-06-24"}, fixedNow(), func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	_, task := Find(result.Payload, "filter")
	if task == nil || Date(task["nextDueOn"]) != "2026-09-24" {
		t.Fatalf("next due=%#v", task)
	}
	if len(result.Extra) == 0 || len(result.Payload["history"].([]any)) != 1 {
		t.Fatalf("result=%#v", result)
	}
	events := CalendarEvents(result.Payload)
	if len(events) != 1 || events[0].AppOwner != "maintenance" {
		t.Fatalf("events=%#v", events)
	}
}
func TestServiceWritesNormalizedDocument(t *testing.T) {
	dir := t.TempDir()
	s := New(ServiceConfig{ConfigDir: dir, Now: fixedNow})
	if err := s.Write(map[string]any{"tasks": []any{map[string]any{"id": "air", "title": "Air", "cadence": map[string]any{"unit": "months", "every": 1}, "nextDueOn": "2026-07-01"}}}); err != nil {
		t.Fatal(err)
	}
	if got := s.File(); got != filepath.Join(dir, "maintenance-tracker.json") {
		t.Fatalf("file=%q", got)
	}
	if len(s.Payload()["tasks"].([]any)) != 1 {
		t.Fatalf("payload=%#v", s.Payload())
	}
}

func TestCompletionCanBeUndoneOnlyWhileItIsTaskLatestState(t *testing.T) {
	payload := map[string]any{"settings": map[string]any{"defaultCalendarEnabled": true, "calendarOutputEnabled": true}, "tasks": []any{map[string]any{"id": "filter", "title": "Filter", "cadence": map[string]any{"unit": "months", "every": 3}, "nextDueOn": "2026-06-24", "calendarEnabled": true, "state": "active"}}}
	completed, err := Apply(payload, "/api/maintenance/tasks/complete", map[string]any{"id": "filter", "completedOn": "2026-06-24"}, fixedNow(), func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	completedRows := DayResponse(completed.Payload, "2026-06-24", fixedNow())
	rows := completedRows["completedItems"].([]any)
	if len(rows) != 1 || !rows[0].(map[string]any)["undoAvailable"].(bool) {
		t.Fatalf("completed day rows=%#v", completedRows)
	}
	completionID := rows[0].(map[string]any)["completionId"].(string)
	undone, err := Apply(completed.Payload, "/api/maintenance/tasks/undo-complete", map[string]any{"id": "filter", "completionId": completionID}, fixedNow(), func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	_, task := Find(undone.Payload, "filter")
	if task == nil || Date(task["nextDueOn"]) != "2026-06-24" || Date(task["lastCompletedOn"]) != "" {
		t.Fatalf("undo task=%#v", task)
	}
	day := DayResponse(undone.Payload, "2026-06-24", fixedNow())
	if len(day["items"].([]any)) != 1 || len(day["completedItems"].([]any)) != 0 {
		t.Fatalf("undo day=%#v", day)
	}
}

func TestCompletionUndoRejectsLaterTaskChange(t *testing.T) {
	payload := map[string]any{"settings": map[string]any{"defaultCalendarEnabled": true, "calendarOutputEnabled": true}, "tasks": []any{map[string]any{"id": "filter", "title": "Filter", "cadence": map[string]any{"unit": "months", "every": 3}, "nextDueOn": "2026-06-24", "calendarEnabled": true, "state": "active"}}}
	completed, err := Apply(payload, "/api/maintenance/tasks/complete", map[string]any{"id": "filter", "completedOn": "2026-06-24"}, fixedNow(), func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	completionID := completed.Extra["completedTask"].(map[string]any)["completionId"].(string)
	changed, err := Apply(completed.Payload, "/api/maintenance/tasks/reschedule", map[string]any{"id": "filter", "nextDueOn": "2026-10-01"}, fixedNow(), func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(changed.Payload, "/api/maintenance/tasks/undo-complete", map[string]any{"id": "filter", "completionId": completionID}, fixedNow(), func(string) (string, bool) { return "", false }); err == nil {
		t.Fatal("undo after reschedule was accepted")
	}
	day := DayResponse(changed.Payload, "2026-06-24", fixedNow())
	rows := day["completedItems"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["undoAvailable"].(bool) {
		t.Fatalf("changed completion should be visible but read-only: %#v", day)
	}
}
