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
