package calendar

import (
	"io"
	"net/http"
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

type issRoundTripper func(*http.Request) (*http.Response, error)

func (fn issRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestReadLatLonRejectsUnsetCoordinatesAndComments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.local.js")
	body := "// lat: 9; lon: 9; old sample\nconst lat: 0;\nconst lon: 0;\n"
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReadLatLon(path); err == nil {
		t.Fatal("0,0 placeholder location was accepted")
	}
}

func TestISSPassesKeepLastGoodFeedOnHTTPOrEnvelopeFailure(t *testing.T) {
	service := testService(t)
	if err := os.WriteFile(filepath.Join(service.homeDir, ".dashboard-default-calendars"), []byte("DEFAULT_ISS_PASSES=1\nISS_N2YO_API_KEY=test-key\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(service.configLocal, []byte("const lat: 41.8781; const lon: -87.6298;"), 0644); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(service.CalendarDir(), "iss.slate.ics")
	previous := []byte("previous-good-feed")
	if err := os.WriteFile(destination, previous, 0644); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		status int
		body   string
	}{
		{"http-status", http.StatusTooManyRequests, `{"error":"quota"}`},
		{"error-envelope", http.StatusOK, `{"error":{"code":1},"passes":[]}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			service.httpClient = func() *http.Client {
				return &http.Client{Transport: issRoundTripper(func(*http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: test.status, Body: io.NopCloser(strings.NewReader(test.body)), Header: make(http.Header)}, nil
				})}
			}
			result := service.UpdateISSPasses()
			if result["ok"] == true {
				t.Fatalf("failure payload was accepted: %#v", result)
			}
			body, err := os.ReadFile(destination)
			if err != nil || string(body) != string(previous) {
				t.Fatalf("previous ISS feed changed: %q err=%v", body, err)
			}
		})
	}
	service.httpClient = func() *http.Client {
		return &http.Client{Transport: issRoundTripper(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"info":{"passescount":0},"passes":[]}`)), Header: make(http.Header)}, nil
		})}
	}
	result := service.UpdateISSPasses()
	if result["ok"] != true || result["eventCount"] != 0 {
		t.Fatalf("valid zero-pass response was rejected: %#v", result)
	}
	body, err := os.ReadFile(destination)
	if err != nil || !strings.Contains(string(body), "BEGIN:VCALENDAR") {
		t.Fatalf("valid zero-pass response did not write an empty calendar: %q err=%v", body, err)
	}
}

func TestGenerateDefaultsReportsOnlySuccessfulMoonAndLogsLeapObservation(t *testing.T) {
	service := testService(t)
	if err := os.WriteFile(filepath.Join(service.homeDir, ".dashboard-default-calendars"), []byte("DEFAULT_MOON_PHASES=1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(service.celebrationsFile, []byte("02-29|Leap day birthday\n"), 0600); err != nil {
		t.Fatal(err)
	}
	service.generateMoon = func(bool) map[string]any { return map[string]any{"ok": false, "error": "simulated moon failure"} }
	result, err := service.GenerateDefaults(false)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range jsonutil.List(result["written"]) {
		if file == "moon.violet.ics" || file == "moon.slate.ics" {
			t.Fatalf("failed moon file was reported as written: %#v", result)
		}
	}
	log, err := os.ReadFile(filepath.Join(service.logDir, "calendar-defaults.log"))
	if err != nil || !strings.Contains(string(log), "observed February 28") {
		t.Fatalf("leap-day observation was not logged: %q err=%v", log, err)
	}
}

func TestCelebrationsHaveStableUIDsAndObserveLeapDays(t *testing.T) {
	path := filepath.Join(t.TempDir(), "celebrations")
	if err := os.WriteFile(path, []byte("02-29|Leap birthday\n05-10|Anniversary\n"), 0600); err != nil {
		t.Fatal(err)
	}
	start, end := DateOnly(2026, time.January, 1), DateOnly(2028, time.January, 1)
	events := CelebrationICSEvents(path, []int{2026, 2027}, start, end)
	bySummary := map[string]string{}
	foundLeap := false
	for _, event := range events {
		bySummary[event.Summary] = event.UID
		if event.Summary == "Leap birthday" && scheduleDateKey(event.Date) == "2026-02-28" && strings.Contains(event.Description, "Observed February 28") {
			foundLeap = true
		}
	}
	if !foundLeap {
		t.Fatalf("non-leap-year birthday was not observed: %#v", events)
	}
	if err := os.WriteFile(path, []byte("05-10|Anniversary\n02-29|Leap birthday\n"), 0600); err != nil {
		t.Fatal(err)
	}
	reordered := CelebrationICSEvents(path, []int{2026}, start, end)
	for _, event := range reordered {
		if want := bySummary[event.Summary]; want != "" && event.UID != want {
			t.Fatalf("UID changed after line reorder for %s: %s want %s", event.Summary, event.UID, want)
		}
	}
}
