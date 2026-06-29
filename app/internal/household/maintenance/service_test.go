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
