package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func TestMaintenancePayloadUsesCompleteDefaultsForWrongShape(t *testing.T) {
	a := testProfileApp(t)
	if err := os.WriteFile(a.maintenanceFile(), []byte(`[]`), 0644); err != nil {
		t.Fatal(err)
	}
	payload := a.maintenancePayload()
	if payload["schema"] != maintenanceSchema {
		t.Fatalf("wrong-shape payload = %#v", payload)
	}
	if enabled, _ := jsonutil.Map(payload["settings"])["defaultCalendarEnabled"].(bool); !enabled {
		t.Fatalf("maintenance defaults must keep calendar enabled: %#v", payload)
	}
}

func TestMaintenanceCompletionUsesLocalCalendarCadence(t *testing.T) {
	oldClock := maintenanceClock
	maintenanceClock = func() time.Time { return time.Date(2026, 1, 31, 8, 0, 0, 0, time.Local) }
	t.Cleanup(func() { maintenanceClock = oldClock })

	if got, want := maintenanceNextDue("2026-01-31", "months", 1), "2026-03-03"; got != want {
		t.Fatalf("month-end local due = %s, want %s", got, want)
	}
	if got, want := maintenanceNextDue("2026-03-07", "weeks", 2), "2026-03-21"; got != want {
		t.Fatalf("week cadence = %s, want %s", got, want)
	}
}

func TestMaintenanceCalendarHasOneNextDueEventPerActiveEnabledTask(t *testing.T) {
	a := testProfileApp(t)
	payload := normalizeMaintenancePayload(map[string]any{
		"tasks": []any{
			map[string]any{"id": "filter", "title": "Replace HVAC filter", "state": "active", "cadence": map[string]any{"unit": "months", "every": 3}, "nextDueOn": "2026-09-01", "calendarEnabled": true},
			map[string]any{"id": "hidden", "title": "Hidden task", "state": "active", "cadence": map[string]any{"unit": "weeks", "every": 1}, "nextDueOn": "2026-06-28", "calendarEnabled": false},
			map[string]any{"id": "archived", "title": "Archived task", "state": "archived", "cadence": map[string]any{"unit": "weeks", "every": 1}, "nextDueOn": "2026-06-28", "calendarEnabled": true},
		},
	})
	if err := a.writeMaintenanceCalendar(payload); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(a.calDir, "maintenance.ics"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, "Replace HVAC filter") || !strings.Contains(text, "maintenance-filter-2026-09-01") {
		t.Fatalf("enabled task event missing: %s", text)
	}
	if strings.Contains(text, "Hidden task") || strings.Contains(text, "Archived task") {
		t.Fatalf("calendar included inactive task: %s", text)
	}
}

func TestMaintenanceRejectsMissingOrNonStringTitleWithoutInventingNilText(t *testing.T) {
	a := testProfileApp(t)
	defaults := maintenanceDefault()["settings"].(map[string]any)
	for _, body := range []map[string]any{
		{},
		{"title": nil},
		{"title": 42},
		{"title": "   "},
	} {
		if _, err := a.maintenanceTaskFromBody(body, defaults, nil); err == nil {
			t.Fatalf("missing/non-string maintenance title was accepted: %#v", body)
		}
	}
	row, err := a.maintenanceTaskFromBody(map[string]any{
		"title":     "Replace HVAC filter",
		"cadence":   map[string]any{"unit": "months", "every": 3},
		"nextDueOn": "2026-09-01",
	}, defaults, nil)
	if err != nil || row["title"] != "Replace HVAC filter" {
		t.Fatalf("valid maintenance task rejected: row=%#v err=%v", row, err)
	}
}

func TestMaintenanceCalendarCarriesAppOwnerMetadata(t *testing.T) {
	a := testProfileApp(t)
	payload := normalizeMaintenancePayload(map[string]any{
		"tasks": []any{
			map[string]any{
				"id": "filter", "title": "Replace filter", "state": "active", "nextDueOn": "2026-06-30", "calendarEnabled": true,
				"cadence": map[string]any{"unit": "months", "every": 3},
			},
		},
	})
	if err := a.writeMaintenanceCalendar(payload); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(maintenanceCalendarPath(a))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "X-DASHGO-APP-OWNER:maintenance") {
		t.Fatalf("maintenance metadata missing: %s", body)
	}
}

func TestMaintenanceOnlyLabelsDueDateChangesAsReschedules(t *testing.T) {
	old := map[string]any{"nextDueOn": "2026-07-01", "title": "Replace filter", "note": "16x25"}
	renameOnly := map[string]any{"nextDueOn": "2026-07-01", "title": "Replace HVAC filter", "note": "16x25"}
	rescheduled := map[string]any{"nextDueOn": "2026-07-15", "title": "Replace filter", "note": "16x25"}
	if maintenanceDueChanged(old, renameOnly) {
		t.Fatal("rename-only edit must not create a rescheduled history entry")
	}
	if !maintenanceDueChanged(old, rescheduled) {
		t.Fatal("due-date change must create a rescheduled history entry")
	}
}
