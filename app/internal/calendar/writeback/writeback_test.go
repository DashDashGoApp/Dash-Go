package writeback

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/calendar/icalwrite"
)

var testNow = time.Date(2026, 7, 2, 21, 0, 0, 0, time.UTC)

func newTestService(t *testing.T) (*Service, string, string) {
	t.Helper()
	home := t.TempDir()
	calDir := filepath.Join(home, "dashboard", "calendars")
	collection := filepath.Join(home, ".dashboard-vdirsyncer", "collections", "family")
	if err := os.MkdirAll(collection, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(calDir, 0700); err != nil {
		t.Fatal(err)
	}
	svc := New(Config{RegistryPath: filepath.Join(home, "dashboard", "config", "calendar-writeback.json"), StatusPath: filepath.Join(home, "dashboard", "cache", "writeback.json"), VDirHome: filepath.Join(home, ".dashboard-vdirsyncer"), CalendarDir: calDir, Now: func() time.Time { return testNow }})
	if err := os.MkdirAll(filepath.Dir(svc.RegistryPath()), 0700); err != nil {
		t.Fatal(err)
	}
	reg := Registry{Version: RegistryVersion, Enabled: true, Calendars: []Calendar{{Source: "calendars/family.blue.ics", Collection: collection, Writable: true, Enabled: true, Name: "Family"}}}
	if err := svc.writeLocked(reg); err != nil {
		t.Fatal(err)
	}
	return svc, collection, filepath.Join(calDir, "family.blue.ics")
}

func TestWritebackCreateMergeDeleteFinalEvent(t *testing.T) {
	svc, collection, mirror := newTestService(t)
	result, err := svc.Create("calendars/family.blue.ics", icalwrite.Event{Title: "Dinner", Start: testNow.Add(24 * time.Hour), End: testNow.Add(25 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if result.UID == "" {
		t.Fatal("create did not assign UID")
	}
	if err := svc.MergeCollection(result.Source, result.Collection); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(mirror)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "SUMMARY:Dinner") {
		t.Fatalf("mirror missing event: %s", body)
	}
	if _, err := svc.Delete(result.Source, result.UID); err != nil {
		t.Fatal(err)
	}
	if err := svc.MergeCollection(result.Source, collection); err != nil {
		t.Fatal(err)
	}
	body, err = os.ReadFile(mirror)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "BEGIN:VEVENT") {
		t.Fatalf("final deletion retained event: %s", body)
	}
}

func TestWritebackRejectsUnregisteredAndOutsideCollection(t *testing.T) {
	svc, _, _ := newTestService(t)
	if svc.SourceWritable("https://example.test/feed.ics") || svc.RegisteredSource("https://example.test/feed.ics") {
		t.Fatal("URL subscription must never be writable")
	}
	if _, err := svc.Create("https://example.test/feed.ics", icalwrite.Event{Title: "x", Start: testNow, End: testNow.Add(time.Hour)}); err == nil {
		t.Fatal("unregistered URL accepted")
	}
	bad := Registry{Version: RegistryVersion, Calendars: []Calendar{{Source: "calendars/x.ics", Collection: t.TempDir(), Writable: true}}}
	if err := svc.validateRegistry(bad); err == nil {
		t.Fatal("outside vdir collection accepted")
	}
}

func TestWritebackSkipPreservesTZIDMaster(t *testing.T) {
	svc, collection, _ := newTestService(t)
	item := filepath.Join(collection, "tz.ics")
	raw := "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nUID:tz\r\nDTSTART;TZID=America/Chicago:20260706T153000\r\nDTEND;TZID=America/Chicago:20260706T163000\r\nRRULE:FREQ=WEEKLY\r\nSUMMARY:Piano\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	if err := os.WriteFile(item, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SkipOccurrence("calendars/family.blue.ics", "tz", time.Date(2026, 7, 13, 20, 30, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(item)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "EXDATE;TZID=America/Chicago:20260713T153000") {
		t.Fatalf("wrong EXDATE: %s", got)
	}
}

func TestRegistryV1MigratesToV2AndKeepsTargetMetadataOptional(t *testing.T) {
	svc, collection, _ := newTestService(t)
	legacy := `{"version":1,"enabled":true,"requirePin":false,"calendars":[{"source":"calendars/family.blue.ics","collection":"` + collection + `","writable":true,"enabled":true,"name":"Family"}]}`
	if err := os.WriteFile(svc.RegistryPath(), []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	status, err := svc.Status()
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Calendars) != 1 || status.Calendars[0].Pair != "" || status.Calendars[0].Provider != "" {
		t.Fatalf("legacy registry was not preserved safely: %#v", status.Calendars)
	}
	if !svc.SourceWritable("calendars/family.blue.ics") {
		t.Fatal("migrated legacy source lost edit capability")
	}
}

func TestWritebackStatusTracksCalendarsIndependently(t *testing.T) {
	svc, collection, _ := newTestService(t)
	second := filepath.Join(filepath.Dir(collection), "personal")
	if err := os.MkdirAll(second, 0700); err != nil {
		t.Fatal(err)
	}
	reg := Registry{Version: RegistryVersion, Enabled: true, Calendars: []Calendar{
		{Source: "calendars/family.blue.ics", Collection: collection, Writable: true, Enabled: true, Name: "Family", Pair: "dash_family", Provider: "google", Connection: "google", RemoteID: "family"},
		{Source: "calendars/personal.green.ics", Collection: second, Writable: true, Enabled: true, Name: "Personal", Pair: "dash_personal", Provider: "caldav", Connection: "icloud", RemoteID: "personal"},
	}}
	if err := svc.writeLocked(reg); err != nil {
		t.Fatal(err)
	}
	svc.Record("calendars/family.blue.ics", "synced", "Family synchronized")
	svc.Record("calendars/personal.green.ics", "waiting", "Personal needs attention")
	status, err := svc.Status()
	if err != nil {
		t.Fatal(err)
	}
	if got := status.States["calendars/family.blue.ics"].State; got != "synced" {
		t.Fatalf("family state = %q", got)
	}
	if got := status.States["calendars/personal.green.ics"].State; got != "waiting" {
		t.Fatalf("personal state = %q", got)
	}
	if status.Calendars[0].Pair == "" || status.Calendars[0].Provider == "" {
		t.Fatalf("targeted pair metadata missing: %#v", status.Calendars[0])
	}
}

func TestWritebackUpdatesOneRecurringOccurrenceAndRejectsDuplicateUIDFiles(t *testing.T) {
	svc, collection, _ := newTestService(t)
	raw := "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nUID:weekly\r\nDTSTART;TZID=America/Chicago:20260706T153000\r\nDTEND;TZID=America/Chicago:20260706T163000\r\nRRULE:FREQ=WEEKLY;COUNT=4\r\nSUMMARY:Piano\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	item := filepath.Join(collection, "weekly.ics")
	if err := os.WriteFile(item, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	occurrence := time.Date(2026, 7, 13, 20, 30, 0, 0, time.UTC)
	if _, err := svc.UpdateOccurrence("calendars/family.blue.ics", "weekly", occurrence, icalwrite.Event{UID: "weekly", Title: "Piano moved", Start: time.Date(2026, 7, 14, 21, 0, 0, 0, time.UTC), End: time.Date(2026, 7, 14, 22, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(item)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "RECURRENCE-ID;TZID=America/Chicago:20260713T153000") || strings.Count(string(body), "BEGIN:VEVENT") != 2 {
		t.Fatalf("occurrence update did not create one detached override:\n%s", body)
	}
	if err := os.WriteFile(filepath.Join(collection, "duplicate.ics"), []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateOccurrence("calendars/family.blue.ics", "weekly", occurrence, icalwrite.Event{UID: "weekly", Title: "unsafe", Start: occurrence, End: occurrence.Add(time.Hour)}); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("duplicate UID source must fail closed, err=%v", err)
	}
}

func TestWritebackStatusMergesBoundedCronOutcomeWithoutOverwritingNewerState(t *testing.T) {
	svc, collection, _ := newTestService(t)
	reg := Registry{Version: RegistryVersion, Enabled: true, Calendars: []Calendar{{Source: "calendars/family.blue.ics", Collection: collection, Writable: true, Enabled: true, Name: "Family", Pair: "dash_family", Provider: "google", Connection: "google", RemoteID: "family"}}}
	if err := svc.writeLocked(reg); err != nil {
		t.Fatal(err)
	}
	resultPath := filepath.Join(filepath.Dir(collection), "..", "last-sync-results")
	resultPath = filepath.Clean(resultPath)
	if err := os.WriteFile(resultPath, []byte("dash_family|conflict|1783035660\nunknown|synced|1783035661\ninvalid|danger|0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	status, err := svc.Status()
	if err != nil {
		t.Fatal(err)
	}
	state := status.States["calendars/family.blue.ics"]
	if state.State != "conflict" || state.Sync != "conflict" {
		t.Fatalf("cron conflict not surfaced safely: %#v", state)
	}
	newer := CalendarState{UpdatedAt: time.Unix(1783035720, 0).UTC().Format(time.RFC3339), State: "synced", Detail: "newer dashboard result"}
	if err := os.MkdirAll(filepath.Dir(svc.statusPath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(svc.statusPath, []byte(`{"version":1,"calendars":{"calendars/family.blue.ics":{"updatedAt":"`+newer.UpdatedAt+`","state":"`+newer.State+`","detail":"`+newer.Detail+`"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	status, err = svc.Status()
	if err != nil {
		t.Fatal(err)
	}
	state = status.States["calendars/family.blue.ics"]
	if state.State != "synced" || state.Sync != "" {
		t.Fatalf("older cron outcome overwrote newer state: %#v", state)
	}
}

func TestWritebackSkipRefusesOccurrenceWithDetachedOverride(t *testing.T) {
	svc, collection, _ := newTestService(t)
	item := filepath.Join(collection, "weekly.ics")
	raw := "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nUID:weekly\r\nSEQUENCE:4\r\nDTSTART;TZID=America/Chicago:20260706T153000\r\nDTEND;TZID=America/Chicago:20260706T163000\r\nRRULE:FREQ=WEEKLY\r\nSUMMARY:Piano\r\nEND:VEVENT\r\nBEGIN:VEVENT\r\nUID:weekly\r\nRECURRENCE-ID;TZID=America/Chicago:20260713T153000\r\nDTSTART;TZID=America/Chicago:20260714T153000\r\nDTEND;TZID=America/Chicago:20260714T163000\r\nSUMMARY:Moved\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	if err := os.WriteFile(item, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := svc.SkipOccurrence("calendars/family.blue.ics", "weekly", time.Date(2026, 7, 13, 20, 30, 0, 0, time.UTC))
	if err == nil || !strings.Contains(err.Error(), "custom change") {
		t.Fatalf("skip with existing override should explain edit path, err=%v", err)
	}
}
