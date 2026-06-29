package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func withFamilyBoardClock(t *testing.T, now time.Time) {
	t.Helper()
	oldClock := familyBoardClock
	familyBoardClock = func() time.Time { return now }
	t.Cleanup(func() { familyBoardClock = oldClock })
}

func TestFamilyBoardPayloadFallsBackAndPersistsOnlySafeNotes(t *testing.T) {
	a := testProfileApp(t)
	if got := a.familyBoardPayload(); got["schema"] != familyBoardSchema {
		t.Fatalf("default family board = %#v", got)
	}
	if err := os.WriteFile(a.familyBoardFile(), []byte(`[]`), 0644); err != nil {
		t.Fatal(err)
	}
	if got := a.familyBoardPayload(); got["schema"] != familyBoardSchema {
		t.Fatalf("non-object family board = %#v", got)
	}
}

func TestFamilyBoardExactExpiryAndUrgentSummary(t *testing.T) {
	now := time.Date(2026, 6, 26, 8, 0, 0, 0, time.Local)
	withFamilyBoardClock(t, now)
	payload := normalizeFamilyBoardPayload(map[string]any{
		"settings": map[string]any{"showPinnedOnDashboard": true},
		"notes": []any{
			map[string]any{"id": "expired", "text": "Old pickup note", "pinned": true, "priority": "normal", "state": "active", "expiresAt": familyBoardExpiryAtEndOfLocalDate("2026-06-25"), "updatedAt": "2026-06-25T20:00:00-05:00"},
			map[string]any{"id": "urgent-pinned", "text": "Dog medicine at dinner", "pinned": true, "priority": "urgent", "state": "active", "expiresAt": now.Add(90 * time.Minute).Format(time.RFC3339), "updatedAt": "2026-06-26T07:00:00-05:00"},
			map[string]any{"id": "urgent-unpinned", "text": "Call school", "pinned": false, "priority": "urgent", "state": "active", "updatedAt": "2026-06-26T07:30:00-05:00"},
			map[string]any{"id": "old-archive", "text": "Do not keep", "state": "archived", "archivedAt": "2026-03-01T07:00:00-06:00", "updatedAt": "2026-03-01T07:00:00-06:00"},
		},
	})
	active := familyBoardActive(payload)
	if len(active) != 2 {
		t.Fatalf("active notes = %#v", active)
	}
	if settings := jsonutil.Map(payload["settings"]); !jsonutil.Truthy(settings["showUrgentAlertsOnDashboard"]) || settings["showPinnedOnDashboard"] != nil {
		t.Fatalf("legacy setting migration = %#v", settings)
	}
	summary := familyBoardSummary(payload)
	if got := summary["displayMode"]; got != "message" {
		t.Fatalf("urgent display mode = %#v", summary)
	}
	if got := summary["urgentCount"]; got != 2 {
		t.Fatalf("urgent count = %#v", summary)
	}
	if note := jsonutil.Map(summary["note"]); note["id"] != "urgent-pinned" {
		t.Fatalf("pinned urgent summary = %#v", summary)
	}
	for _, raw := range jsonutil.List(payload["notes"]) {
		note := jsonutil.Map(raw)
		if note["id"] == "expired" && note["state"] != "archived" {
			t.Fatalf("date-expired note was not archived: %#v", note)
		}
		if note["id"] == "expired" && note["expiresAt"] != familyBoardExpiryAtEndOfLocalDate("2026-06-25") {
			t.Fatalf("date expiry did not retain exact local timestamp: %#v", note)
		}
		if note["id"] == "old-archive" {
			t.Fatalf("archive older than retention was retained: %#v", payload)
		}
	}
}

func TestFamilyBoardRetiredExpiresOnDoesNotDriveCurrentExpiry(t *testing.T) {
	now := time.Date(2026, 6, 26, 8, 0, 0, 0, time.Local)
	withFamilyBoardClock(t, now)
	note := map[string]any{}
	if err := familyBoardMutableNote(note, map[string]any{"text": "Current note", "expiresOn": "2026-06-25"}, true); err != nil {
		t.Fatal(err)
	}
	if note["expiresAt"] != "" {
		t.Fatalf("retired expiresOn drove current expiry: %#v", note)
	}
	if _, ok := note["expiresOn"]; ok {
		t.Fatalf("retired expiresOn was persisted: %#v", note)
	}
}

func TestRestoreConfigBackupRejectsRetiredBoardWithoutPrivateStore(t *testing.T) {
	a := testProfileApp(t)
	live := filepath.Join(a.configDir, "settings.json")
	if err := os.WriteFile(live, []byte(`{"theme":"live"}`), 0644); err != nil {
		t.Fatal(err)
	}
	name := writeConfigBackupFixture(t, a, "retired-family-board-only.zip", map[string][]byte{
		"config/settings.json":     []byte(`{"theme":"retired"}`),
		"config/family-board.json": []byte(`{"schema":1,"notes":[]}`),
	})
	if _, err := a.restoreConfigBackup(name); err == nil || !strings.Contains(err.Error(), "retired config/family-board.json") {
		t.Fatalf("retired Board-only backup error = %v", err)
	}
	got, err := os.ReadFile(live)
	if err != nil || string(got) != `{"theme":"live"}` {
		t.Fatalf("rejected backup changed live config: %q / %v", got, err)
	}
}

func TestRestoreConfigBackupUsesPrivateBoardWhenRetiredCopyIsPresent(t *testing.T) {
	a := testProfileApp(t)
	name := writeConfigBackupFixture(t, a, "family-board-private-with-retired-copy.zip", map[string][]byte{
		"config/family-board.json":  []byte(`{"schema":1,"notes":[{"id":"retired","text":"Ignore this"}]}`),
		"secrets/family-board.json": []byte(`{"schema":3,"notes":[{"id":"private","text":"Restore this","scope":"household"}]}`),
	})
	if _, err := a.restoreConfigBackup(name); err != nil {
		t.Fatal(err)
	}
	payload, err := a.familyBoardReadPayload()
	if err != nil {
		t.Fatal(err)
	}
	if got := len(jsonutil.List(payload["notes"])); got != 1 || jsonutil.Map(jsonutil.List(payload["notes"])[0])["id"] != "private" {
		t.Fatalf("private Board data was not restored: %#v", payload)
	}
	if _, err := os.Stat(filepath.Join(a.configDir, "family-board.json")); !os.IsNotExist(err) {
		t.Fatalf("retired Board copy was restored into config tree: %v", err)
	}
}

func TestFamilyBoardUrgentDisplayModes(t *testing.T) {
	now := time.Date(2026, 6, 26, 8, 0, 0, 0, time.Local)
	withFamilyBoardClock(t, now)
	base := func(enabled bool, notes []any) map[string]any {
		return normalizeFamilyBoardPayload(map[string]any{"settings": map[string]any{"showUrgentAlertsOnDashboard": enabled}, "notes": notes})
	}
	if got := familyBoardSummary(base(true, []any{map[string]any{"id": "normal", "text": "Normal pinned", "pinned": true, "priority": "normal"}}))["displayMode"]; got != "none" {
		t.Fatalf("normal pinned dashboard state = %v, want none", got)
	}
	if got := familyBoardSummary(base(true, []any{map[string]any{"id": "urgent", "text": "Urgent", "priority": "urgent", "pinned": false}}))["displayMode"]; got != "alert" {
		t.Fatalf("urgent unpinned dashboard state = %v, want alert", got)
	}
	if got := familyBoardSummary(base(false, []any{map[string]any{"id": "urgent", "text": "Urgent", "priority": "urgent", "pinned": true}}))["displayMode"]; got != "none" {
		t.Fatalf("disabled urgent dashboard state = %v, want none", got)
	}
}

func TestFamilyBoardExpirationDurationValidationAndExactArchive(t *testing.T) {
	now := time.Date(2026, 6, 26, 8, 0, 0, 0, time.Local)
	withFamilyBoardClock(t, now)
	note := map[string]any{}
	body := map[string]any{"text": "Leave in fifteen minutes", "priority": "urgent", "pinned": false, "expiration": map[string]any{"kind": "duration", "amount": 15.0, "unit": "minutes"}}
	if err := familyBoardMutableNote(note, body, true); err != nil {
		t.Fatalf("duration expiration rejected: %v", err)
	}
	want := now.Add(15 * time.Minute).Format(time.RFC3339)
	if got := note["expiresAt"]; got != want {
		t.Fatalf("duration expiry = %v, want %v", got, want)
	}
	if got := familyBoardNote(note, now.Add(15*time.Minute)); got["state"] != "archived" {
		t.Fatalf("exact expiry did not archive at deadline: %#v", got)
	}
	for _, expiration := range []map[string]any{
		{"kind": "duration", "amount": 0.0, "unit": "minutes"},
		{"kind": "duration", "amount": 1.5, "unit": "minutes"},
		{"kind": "duration", "amount": 1441.0, "unit": "minutes"},
		{"kind": "duration", "amount": 169.0, "unit": "hours"},
		{"kind": "duration", "amount": 2.0, "unit": "days"},
		{"kind": "date", "date": "2026-99-99"},
	} {
		if _, err := familyBoardExpiration(map[string]any{}, map[string]any{"expiration": expiration}, true, now); err == nil {
			t.Fatalf("invalid expiration accepted: %#v", expiration)
		}
	}
	dateExpiry, err := familyBoardExpiration(map[string]any{}, map[string]any{"expiration": map[string]any{"kind": "date", "date": "2026-06-30"}}, true, now)
	if err != nil || dateExpiry != familyBoardExpiryAtEndOfLocalDate("2026-06-30") {
		t.Fatalf("date-only expiry = %q, %v", dateExpiry, err)
	}
}

func TestFamilyBoardReadPersistsExpiryArchiveOnceAndThenPrunesByRetention(t *testing.T) {
	a := testProfileApp(t)
	now := time.Date(2026, 6, 26, 8, 0, 0, 0, time.Local)
	withFamilyBoardClock(t, now)
	if err := fileio.WriteJSON(a.familyBoardFile(), map[string]any{
		"schema":   familyBoardSchema,
		"settings": map[string]any{"showUrgentAlertsOnDashboard": false},
		"notes":    []any{map[string]any{"id": "expired", "text": "Old pickup", "state": "active", "expiresAt": now.Add(-time.Minute).Format(time.RFC3339), "createdAt": now.AddDate(0, 0, -6).Format(time.RFC3339), "updatedAt": now.AddDate(0, 0, -6).Format(time.RFC3339)}},
	}); err != nil {
		t.Fatal(err)
	}
	payload, err := a.familyBoardReadPayload()
	if err != nil {
		t.Fatal(err)
	}
	note := jsonutil.Map(jsonutil.List(payload["notes"])[0])
	expectedArchive := familyBoardArchiveStamp(now)
	if note["state"] != "archived" || note["archivedAt"] != expectedArchive {
		t.Fatalf("expired note was not durably archived: %#v", note)
	}
	stored := jsonutil.Map(a.readJSONDefault(a.familyBoardFile(), familyBoardDefault()))
	storedNote := jsonutil.Map(jsonutil.List(stored["notes"])[0])
	if storedNote["archivedAt"] != note["archivedAt"] {
		t.Fatalf("archive stamp was not persisted: %#v", storedNote)
	}
	familyBoardClock = func() time.Time { return now.Add(24 * time.Hour) }
	payload, err = a.familyBoardReadPayload()
	if err != nil {
		t.Fatal(err)
	}
	if got := jsonutil.Map(jsonutil.List(payload["notes"])[0])["archivedAt"]; got != expectedArchive {
		t.Fatalf("repeated reads reset archivedAt: %v", got)
	}
	familyBoardClock = func() time.Time { return now.AddDate(0, 0, familyBoardArchiveDays+1) }
	payload, err = a.familyBoardReadPayload()
	if err != nil {
		t.Fatal(err)
	}
	if got := len(jsonutil.List(payload["notes"])); got != 0 {
		t.Fatalf("expired archive remained past retention: %#v", payload)
	}
}

func TestFamilyBoardRejectsMissingOrNonStringTextWithoutInventingNilText(t *testing.T) {
	for _, body := range []map[string]any{
		{},
		{"text": nil},
		{"text": 42},
		{"text": "   "},
	} {
		if err := familyBoardMutableNote(map[string]any{}, body, true); err == nil {
			t.Fatalf("missing/non-string note text was accepted: %#v", body)
		}
	}
	if err := familyBoardMutableNote(map[string]any{}, map[string]any{"text": "A real family note"}, true); err != nil {
		t.Fatalf("optional missing expiration must remain valid: %v", err)
	}
}
