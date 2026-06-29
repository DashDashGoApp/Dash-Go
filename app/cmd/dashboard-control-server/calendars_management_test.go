package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func calendarBody(title string) []byte {
	return []byte("BEGIN:VCALENDAR\nVERSION:2.0\nBEGIN:VEVENT\nUID:" + title + "\nDTSTART;VALUE=DATE:20260624\nSUMMARY:" + title + "\nEND:VEVENT\nEND:VCALENDAR\n")
}

func manifestRows(a *app) []map[string]any {
	out := []map[string]any{}
	for _, raw := range jsonutil.List(a.readJSONDefault(filepath.Join(a.calDir, "calendars.json"), []any{})) {
		out = append(out, jsonutil.Map(raw))
	}
	return out
}

func hasCalendarURL(rows []map[string]any, url string) bool {
	for _, row := range rows {
		if calendarSourceIdentity(strOr(row["url"], "")) == calendarSourceIdentity(url) {
			return true
		}
	}
	return false
}

func TestCalendarManagerArchivesAndRestoresLocalCalendar(t *testing.T) {
	a := testProfileApp(t)
	source := filepath.Join(a.calDir, "school.ics")
	if err := os.WriteFile(source, calendarBody("School"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := a.generateCalendarManifest(); err != nil {
		t.Fatal(err)
	}
	if !hasCalendarURL(manifestRows(a), "calendars/school.ics") {
		t.Fatal("source missing from initial calendar manifest")
	}

	record, err := a.archiveLocalCalendar("calendars/school.ics", "School")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(source); !os.IsNotExist(err) {
		t.Fatalf("source should be absent after archive: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(a.calendarTrashDir(), record.TrashName)); err != nil {
		t.Fatalf("archived source missing: %v", err)
	}
	if hasCalendarURL(manifestRows(a), "calendars/school.ics") {
		t.Fatal("archived source remained active in manifest")
	}
	if got := a.loadCalendarTrash(); len(got) != 1 || got[0].ID != record.ID || !got[0].WasEnabled {
		t.Fatalf("trash record = %#v", got)
	}

	restored, err := a.restoreLocalCalendar(record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if restored.ID != record.ID {
		t.Fatalf("restored wrong record: %#v", restored)
	}
	body, err := os.ReadFile(source)
	if err != nil || !strings.Contains(string(body), "SUMMARY:School") {
		t.Fatalf("restored source incorrect: %v %q", err, body)
	}
	if !hasCalendarURL(manifestRows(a), "calendars/school.ics") {
		t.Fatal("restored source missing from manifest")
	}
	if got := a.loadCalendarTrash(); len(got) != 0 {
		t.Fatalf("restored calendar left stale trash record: %#v", got)
	}
}

func TestCalendarManagerArchivesSymlinkWithoutTouchingTarget(t *testing.T) {
	a := testProfileApp(t)
	outside := filepath.Join(t.TempDir(), "external.ics")
	original := calendarBody("External")
	if err := os.WriteFile(outside, original, 0644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(a.calDir, "linked.ics")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := a.generateCalendarManifest(); err != nil {
		t.Fatal(err)
	}
	record, err := a.archiveLocalCalendar("calendars/linked.ics", "Linked")
	if err != nil {
		t.Fatal(err)
	}
	if !record.IsSymlink {
		t.Fatalf("symlink archive must record link ownership: %#v", record)
	}
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Fatalf("Dash-Go symlink should be moved, got %v", err)
	}
	if body, err := os.ReadFile(outside); err != nil || string(body) != string(original) {
		t.Fatalf("archive must never alter external link target: %v %q", err, body)
	}
	trashPath := filepath.Join(a.calendarTrashDir(), record.TrashName)
	if info, err := os.Lstat(trashPath); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("trash must retain symlink itself: info=%#v err=%v", info, err)
	}
	if _, err := a.restoreLocalCalendar(record.ID); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("restore must recreate the local link: info=%#v err=%v", info, err)
	}
	if body, err := os.ReadFile(outside); err != nil || string(body) != string(original) {
		t.Fatalf("restore must never alter external target: %v %q", err, body)
	}
}

func TestCalendarManagerRejectsPathTraversal(t *testing.T) {
	a := testProfileApp(t)
	for _, source := range []string{
		"../outside.ics", "calendars/../outside.ics", "calendars/subdir/file.ics",
		"/tmp/outside.ics", "https://example.invalid/calendar.ics", "calendar/../x.ics",
	} {
		if _, _, err := a.calendarPathForURL(source); err == nil {
			t.Fatalf("unsafe source accepted: %q", source)
		}
	}
}

func TestAppCalendarOutputCanBeStoppedAndRebuiltWithoutLosingAppState(t *testing.T) {
	a := testProfileApp(t)
	disableCalendarCacheRefreshForTest(t, a)
	chorePayload := normalizeChoreWheelPayload(map[string]any{
		"people":      []any{map[string]any{"id": "alex", "name": "Alex"}},
		"chores":      []any{map[string]any{"id": "litter", "name": "Kitty litter", "eligible": []any{"alex"}}},
		"assignments": []any{map[string]any{"id": "litter-today", "date": "2026-06-24", "choreId": "litter", "choreName": "Kitty litter", "personId": "alex", "personName": "Alex"}},
	})
	if err := a.commitChoreWheelPayload(chorePayload); err != nil {
		t.Fatal(err)
	}
	choreFeed := filepath.Join(a.calDir, "chore-wheel.ics")
	if !fileio.Exists(choreFeed) {
		t.Fatal("enabled Chore Wheel output did not create feed")
	}
	if _, err := a.setOwnedCalendarOutput("chore-wheel", false); err != nil {
		t.Fatal(err)
	}
	if fileio.Exists(choreFeed) {
		t.Fatal("disabled Chore Wheel output retained feed")
	}
	stored := a.choreWheelPayload()
	if choreWheelCalendarOutputEnabled(stored) || len(jsonutil.List(stored["assignments"])) != 1 {
		t.Fatalf("disabling output must retain Chore Wheel data: %#v", stored)
	}
	if _, err := a.setOwnedCalendarOutput("chore-wheel", true); err != nil {
		t.Fatal(err)
	}
	if !fileio.Exists(choreFeed) {
		t.Fatal("re-enabled Chore Wheel output did not rebuild feed")
	}

	maintenancePayload := normalizeMaintenancePayload(map[string]any{
		"tasks": []any{map[string]any{"id": "filter", "title": "Replace HVAC filter", "state": "active", "cadence": map[string]any{"unit": "months", "every": 3}, "nextDueOn": "2026-09-01", "calendarEnabled": true}},
	})
	if err := a.commitMaintenancePayload(maintenancePayload); err != nil {
		t.Fatal(err)
	}
	maintenanceFeed := maintenanceCalendarPath(a)
	if !fileio.Exists(maintenanceFeed) {
		t.Fatal("enabled Maintenance output did not create feed")
	}
	if _, err := a.setOwnedCalendarOutput("maintenance", false); err != nil {
		t.Fatal(err)
	}
	if fileio.Exists(maintenanceFeed) {
		t.Fatal("disabled Maintenance output retained feed")
	}
	storedMaintenance := a.maintenancePayload()
	if maintenanceCalendarOutputEnabled(storedMaintenance) || len(jsonutil.List(storedMaintenance["tasks"])) != 1 {
		t.Fatalf("disabling output must retain Maintenance data: %#v", storedMaintenance)
	}
	if _, err := a.setOwnedCalendarOutput("maintenance", true); err != nil {
		t.Fatal(err)
	}
	if !fileio.Exists(maintenanceFeed) {
		t.Fatal("re-enabled Maintenance output did not rebuild feed")
	}
}

func TestCalendarManagerRepairRegeneratesAndDeduplicatesManifest(t *testing.T) {
	a := testProfileApp(t)
	if err := os.WriteFile(filepath.Join(a.calDir, "work.ics"), calendarBody("Work"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := fileio.WriteJSON(filepath.Join(a.calDir, "calendars.json"), []any{
		map[string]any{"url": "calendars/work.ics", "name": "Work", "enabled": false},
		map[string]any{"url": "calendars/work.ics", "name": "Duplicate Work", "enabled": true},
		map[string]any{"url": "calendars/missing.ics", "name": "Missing", "enabled": true},
	}); err != nil {
		t.Fatal(err)
	}
	result, err := a.repairCalendarIndex()
	if err != nil {
		t.Fatal(err)
	}
	if jsonutil.Int(result["after"], 0) != 1 {
		t.Fatalf("repair result = %#v", result)
	}
	rows := manifestRows(a)
	if len(rows) != 1 || !hasCalendarURL(rows, "calendars/work.ics") {
		t.Fatalf("repair manifest = %#v", rows)
	}
	if calendarEntryEnabled(rows[0]) {
		t.Fatalf("repair must preserve prior visibility where possible: %#v", rows[0])
	}
}

func TestCalendarTrashPurgesExpiredRecordsOnly(t *testing.T) {
	a := testProfileApp(t)
	if err := os.MkdirAll(a.calendarTrashDir(), 0755); err != nil {
		t.Fatal(err)
	}
	expired := calendarTrashRecord{
		ID: "old", Name: "Old calendar", URL: "calendars/old.ics", TrashName: "old.ics",
		DeletedAt: "2026-01-01T00:00:00Z", PurgeAfter: "2026-01-02T00:00:00Z", WasEnabled: true,
	}
	active := calendarTrashRecord{
		ID: "new", Name: "New calendar", URL: "calendars/new.ics", TrashName: "new.ics",
		DeletedAt: "2099-01-01T00:00:00Z", PurgeAfter: "2099-02-01T00:00:00Z", WasEnabled: true,
	}
	if err := os.WriteFile(filepath.Join(a.calendarTrashDir(), expired.TrashName), calendarBody("Old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.calendarTrashDir(), active.TrashName), calendarBody("New"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := a.writeCalendarTrash([]calendarTrashRecord{expired, active}); err != nil {
		t.Fatal(err)
	}
	if got := a.purgeExpiredCalendarTrash(); got != 1 {
		t.Fatalf("purged %d records, want 1", got)
	}
	if _, err := os.Stat(filepath.Join(a.calendarTrashDir(), expired.TrashName)); !os.IsNotExist(err) {
		t.Fatalf("expired file remained: %v", err)
	}
	if _, err := os.Stat(filepath.Join(a.calendarTrashDir(), active.TrashName)); err != nil {
		t.Fatalf("unexpired file was removed: %v", err)
	}
	if got := a.loadCalendarTrash(); len(got) != 1 || got[0].ID != active.ID {
		t.Fatalf("trash after purge = %#v", got)
	}
}

func TestCalendarManagerReadDoesNotRepairManifest(t *testing.T) {
	a := testProfileApp(t)
	if err := os.WriteFile(filepath.Join(a.calDir, "work.ics"), calendarBody("Work"), 0644); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(a.calDir, "calendars.json")
	original := []byte(`[{"url":"calendars/missing.ics","name":"Old missing","enabled":false}]`)
	if err := os.WriteFile(manifestPath, original, 0644); err != nil {
		t.Fatal(err)
	}
	status := a.calendarManagementStatus()
	rows, ok := status["calendars"].([]map[string]any)
	if !ok || len(rows) < 2 {
		t.Fatalf("manager should show stale manifest and directly discovered source before repair: %#v", status["calendars"])
	}
	for _, row := range rows {
		if row["url"] == "calendars/missing.ics" && row["kind"] != "missing" {
			t.Fatalf("missing manifest source should be inspectable, got %#v", row)
		}
	}
	after, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(original) {
		t.Fatalf("reading Calendar Manager must not rewrite manifest before explicit Repair:\n got %s\nwant %s", after, original)
	}
}
