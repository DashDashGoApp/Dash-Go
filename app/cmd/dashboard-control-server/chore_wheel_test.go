package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func TestChoreWheelPayloadNormalizesWrongShapesAndLegacyAnchor(t *testing.T) {
	a := testProfileApp(t)
	if got := a.choreWheelPayload(); got["schema"] != choreWheelSchema {
		t.Fatalf("default chore payload = %#v", got)
	}
	if err := os.MkdirAll(a.configDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.choreWheelFile(), []byte(`[]`), 0644); err != nil {
		t.Fatal(err)
	}
	if got := a.choreWheelPayload(); got["schema"] != choreWheelSchema {
		t.Fatalf("non-object payload = %#v", got)
	}
	legacy := `{"people":[{"id":"sam","name":"Sam"}],"chores":[{"id":"litter","name":"Kitty litter","createdAt":"2026-03-07T22:00:00-06:00","cadence":{"type":"days","every":7},"eligible":["sam"]}],"assignments":[]}`
	if err := os.WriteFile(a.choreWheelFile(), []byte(legacy), 0644); err != nil {
		t.Fatal(err)
	}
	payload := a.choreWheelPayload()
	chores := jsonutil.List(payload["chores"])
	if len(chores) != 1 {
		t.Fatalf("chores = %#v", payload)
	}
	cadence := jsonutil.Map(jsonutil.Map(chores[0])["cadence"])
	if got := cadence["anchorDate"]; got != "2026-03-08" && got != "2026-03-07" {
		t.Fatalf("legacy local anchor = %#v", cadence)
	}
}

func TestChoreWheelCalendarIsBoundedAndStatusAware(t *testing.T) {
	a := testProfileApp(t)
	oldClock := choreWheelClock
	choreWheelClock = func() time.Time { return time.Date(2026, 6, 24, 12, 0, 0, 0, time.Local) }
	t.Cleanup(func() { choreWheelClock = oldClock })
	payload := normalizeChoreWheelPayload(map[string]any{
		"people": []any{map[string]any{"id": "alex", "name": "Alex"}},
		"chores": []any{map[string]any{"id": "dishes", "name": "Dishes"}},
		"assignments": []any{
			map[string]any{"id": "old", "date": "2026-05-20", "choreId": "dishes", "choreName": "Dishes", "personId": "alex", "personName": "Alex", "status": "assigned"},
			map[string]any{"id": "today", "date": "2026-06-24", "choreId": "dishes", "choreName": "Dishes", "personId": "alex", "personName": "Alex", "status": "completed"},
			map[string]any{"id": "far", "date": "2026-08-01", "choreId": "dishes", "choreName": "Dishes", "personId": "alex", "personName": "Alex", "status": "assigned"},
		},
		"settings": map[string]any{"horizonDays": 14},
	})
	if err := a.writeChoreWheelCalendar(payload); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(a.calDir, "chore-wheel.ics"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, "✓ Dishes — Alex") {
		t.Fatalf("completed event missing: %s", text)
	}
	if strings.Contains(text, "20260520") || strings.Contains(text, "20260801") {
		t.Fatalf("unbounded Chore calendar: %s", text)
	}
}

func TestChoreWheelHorizonAllowsThirtyDays(t *testing.T) {
	oldClock := choreWheelClock
	choreWheelClock = func() time.Time { return time.Date(2026, 6, 24, 12, 0, 0, 0, time.Local) }
	t.Cleanup(func() { choreWheelClock = oldClock })

	payload := normalizeChoreWheelPayload(map[string]any{
		"settings": map[string]any{"horizonDays": 30},
	})
	if got := jsonutil.Int(jsonutil.Map(payload["settings"])["horizonDays"], 0); got != 30 {
		t.Fatalf("30-day horizon was not preserved: %#v", payload)
	}
	_, end := choreWheelCalendarRange(payload)
	if got, want := end.Format("2006-01-02"), "2026-07-24"; got != want {
		t.Fatalf("30-day calendar range end = %s, want %s", got, want)
	}
}

func TestChoreWheelCalendarRetiresOrphanedFutureAndEarlyResolvedRows(t *testing.T) {
	a := testProfileApp(t)
	oldClock := choreWheelClock
	choreWheelClock = func() time.Time { return time.Date(2026, 6, 24, 12, 0, 0, 0, time.Local) }
	t.Cleanup(func() { choreWheelClock = oldClock })
	payload := normalizeChoreWheelPayload(map[string]any{
		"people": []any{map[string]any{"id": "alex", "name": "Alex"}},
		"chores": []any{map[string]any{"id": "dishes", "name": "Dishes"}},
		"assignments": []any{
			map[string]any{"id": "past", "date": "2026-06-23", "choreId": "removed", "choreName": "Old chore", "personId": "alex", "personName": "Alex", "status": "completed"},
			map[string]any{"id": "orphan-future", "date": "2026-06-25", "choreId": "removed", "choreName": "Old chore", "personId": "alex", "personName": "Alex", "status": "assigned"},
			map[string]any{"id": "early-complete", "date": "2026-06-26", "choreId": "dishes", "choreName": "Dishes", "personId": "alex", "personName": "Alex", "status": "completed"},
			map[string]any{"id": "future", "date": "2026-06-25", "choreId": "dishes", "choreName": "Dishes", "personId": "alex", "personName": "Alex", "status": "assigned"},
		},
	})
	if err := a.writeChoreWheelCalendar(payload); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(a.calDir, "chore-wheel.ics"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(body)
	if !strings.Contains(got, "Dishes — Alex") {
		t.Fatalf("valid future assignment missing: %s", got)
	}
	if strings.Contains(got, "UID:chore-wheel-early-complete-") || strings.Contains(got, "Old chore") {
		t.Fatalf("resolved or app-only orphaned rows must not project: %s", got)
	}
	if !strings.Contains(got, "X-DASHGO-APP-OWNER:chore-wheel") {
		t.Fatalf("chore metadata missing: %s", got)
	}
}

func TestChoreWheelRevisionDefaultsAndNormalizes(t *testing.T) {
	if got := jsonutil.Int(normalizeChoreWheelPayload(map[string]any{})["revision"], -1); got != 0 {
		t.Fatalf("default revision = %d", got)
	}
	if got := jsonutil.Int(normalizeChoreWheelPayload(map[string]any{"revision": 7})["revision"], -1); got != 7 {
		t.Fatalf("revision was not preserved: %d", got)
	}
	if got := jsonutil.Int(normalizeChoreWheelPayload(map[string]any{"revision": -4})["revision"], -1); got != 0 {
		t.Fatalf("negative revision was not clamped: %d", got)
	}
}
