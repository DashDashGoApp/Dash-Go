package calendar

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func testService(t *testing.T) *Service {
	t.Helper()
	root := t.TempDir()
	dash, home := filepath.Join(root, "dashboard"), filepath.Join(root, "home")
	for _, path := range []string{dash, home, filepath.Join(dash, "calendars"), filepath.Join(dash, "cache"), filepath.Join(dash, "logs"), filepath.Join(dash, "config")} {
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Fatal(err)
		}
	}
	return New(ServiceConfig{
		DashDir: dash, HomeDir: home, CalendarDir: filepath.Join(dash, "calendars"), CacheDir: filepath.Join(dash, "cache"), LogDir: filepath.Join(dash, "logs"),
		ConfigLocal: filepath.Join(dash, "config", "config.local.js"), CelebrationsFile: filepath.Join(home, ".dashboard-celebrations"), HouseholdSchedulesFile: filepath.Join(dash, "config", "household-schedules.json"),
		Now:              func() time.Time { return time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC) },
		RefreshCacheSync: func() error { return nil }, RefreshCacheAsync: func() {},
	})
}

func testCalendarBody(title string) []byte {
	return []byte("BEGIN:VCALENDAR\nVERSION:2.0\nBEGIN:VEVENT\nUID:" + title + "\nDTSTART;VALUE=DATE:20260624\nSUMMARY:" + title + "\nEND:VEVENT\nEND:VCALENDAR\n")
}

func TestManifestPreservesHiddenSourceAcrossRepair(t *testing.T) {
	service := testService(t)
	if err := os.WriteFile(filepath.Join(service.CalendarDir(), "work.ics"), testCalendarBody("Work"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(service.CalendarDir(), "calendars.json"), []byte(`[{"url":"calendars/work.ics","name":"Work","enabled":false}]`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := service.GenerateManifest(); err != nil {
		t.Fatal(err)
	}
	rows := jsonutil.List(service.readJSONDefault(filepath.Join(service.CalendarDir(), "calendars.json"), []any{}))
	if len(rows) != 1 || CalendarEntryEnabled(jsonutil.Map(rows[0])) {
		t.Fatalf("hidden preference changed: %#v", rows)
	}
}

func TestArchiveAndRestorePreservesSourceAndTrashRecord(t *testing.T) {
	service := testService(t)
	path := filepath.Join(service.CalendarDir(), "school.ics")
	if err := os.WriteFile(path, testCalendarBody("School"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := service.GenerateManifest(); err != nil {
		t.Fatal(err)
	}
	record, err := service.Archive("calendars/school.ics", "School")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("source remained after archive: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(service.TrashDir(), record.TrashName)); err != nil {
		t.Fatalf("trash source missing: %v", err)
	}
	restored, err := service.Restore(record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if restored.ID != record.ID {
		t.Fatalf("wrong restored record: %#v", restored)
	}
	body, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(body), "SUMMARY:School") {
		t.Fatalf("restored body = %q err=%v", body, err)
	}
}

func TestOwnedSourceAndManagedPathContracts(t *testing.T) {
	owned, ok := OwnedSource("CALENDARS/CHORE-WHEEL.ICS")
	if !ok || owned.Name != "Chores" || owned.Owner != "chore-wheel" {
		t.Fatalf("owned source = %#v ok=%v", owned, ok)
	}
	for _, raw := range []string{"../outside.ics", "calendars/../outside.ics", "calendars/subdir/work.ics", "https://example.invalid/x.ics"} {
		if _, _, err := LocalPathForURL(raw, "/tmp/calendars", "/tmp/dash"); err == nil {
			t.Fatalf("unsafe source accepted: %q", raw)
		}
	}
}

func TestCommitUsesCallerOutputStateWithoutReenteringHousehold(t *testing.T) {
	service := testService(t)
	service.outputEnabled = func(owner string) bool {
		t.Fatalf("Calendar re-entered household output callback for %s while committing", owner)
		return false
	}
	payloadSaved := false
	err := service.CommitOwnedFeed(OwnedFeedCommit{
		Owner: "chore-wheel", Name: "Chores", Events: []Event{AllDayEvent(2026, time.June, 24, "Kitchen", "chore-1")},
		Enabled: true, OutputState: map[string]bool{"chore-wheel": true, "maintenance": true, "routines": true},
		Save: func() error { payloadSaved = true; return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if !payloadSaved {
		t.Fatal("app payload save was not called")
	}
	if _, err := os.Stat(filepath.Join(service.CalendarDir(), "chore-wheel.ics")); err != nil {
		t.Fatalf("owned feed missing: %v", err)
	}
}

func TestReadLatLonPreservesLegacyConfigLocalShape(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.local.js")
	if err := os.WriteFile(path, []byte("const lat: 41.8781;\nconst lon: -87.6298;\n"), 0644); err != nil {
		t.Fatal(err)
	}
	lat, lon, err := ReadLatLon(path)
	if err != nil || lat != 41.8781 || lon != -87.6298 {
		t.Fatalf("lat/lon = %v/%v err=%v", lat, lon, err)
	}
}

func TestCalendarRefreshPortsRunAfterTransactionUnlock(t *testing.T) {
	service := testService(t)
	service.refreshCacheAsync = func() {
		if err := service.WithLock(nil); err != nil {
			t.Errorf("async refresh could not re-enter Calendar after commit: %v", err)
		}
	}
	service.refreshCacheSync = func() error {
		return service.WithLock(nil)
	}

	commitDone := make(chan error, 1)
	go func() {
		commitDone <- service.CommitOwnedFeed(OwnedFeedCommit{
			Owner: "chore-wheel", Name: "Chores", Enabled: true,
			Events:      []Event{AllDayEvent(2026, time.June, 24, "Kitchen", "chore-1")},
			OutputState: map[string]bool{"chore-wheel": true},
		})
	}()
	select {
	case err := <-commitDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("CommitOwnedFeed held the Calendar lock while calling refreshCacheAsync")
	}

	path := filepath.Join(service.CalendarDir(), "local.ics")
	if err := os.WriteFile(path, testCalendarBody("Local"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := service.GenerateManifest(); err != nil {
		t.Fatal(err)
	}
	archiveDone := make(chan error, 1)
	go func() {
		_, err := service.Archive("calendars/local.ics", "Local")
		archiveDone <- err
	}()
	select {
	case err := <-archiveDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("Archive held the Calendar lock while calling refreshCacheSync")
	}
}
