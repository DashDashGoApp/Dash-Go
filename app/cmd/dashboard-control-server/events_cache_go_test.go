package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func TestGoEventCacheBuildsSingleAndRecurringEvents(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	dash := filepath.Join(tmp, "dash")
	a := &app{dash: dash, home: home, configDir: filepath.Join(dash, "config"), calDir: filepath.Join(dash, "calendars"), cacheDir: filepath.Join(dash, "cache"), logDir: filepath.Join(dash, "logs"), binDir: filepath.Join(dash, "bin"), settingsFile: filepath.Join(dash, "config", "settings.json"), configLocal: filepath.Join(dash, "config", "config.local.js"), celebrationsFile: filepath.Join(home, ".dashboard-celebrations")}
	a.ensureDirs()
	ics := `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:one@test
DTSTART:20260620T090000
DTEND:20260620T100000
SUMMARY:One-time Test
LOCATION:Kitchen
END:VEVENT
BEGIN:VEVENT
UID:weekly@test
DTSTART:20260601T120000
DTEND:20260601T123000
RRULE:FREQ=WEEKLY;COUNT=4;BYDAY=MO
SUMMARY:Weekly Test
END:VEVENT
END:VCALENDAR
`
	if err := os.WriteFile(filepath.Join(a.calDir, "personal.green.ics"), []byte(ics), 0644); err != nil {
		t.Fatal(err)
	}
	res, err := a.refreshEventCache(true, 30, 60)
	if err != nil {
		t.Fatalf("refreshEventCache failed: %v", err)
	}
	if res["generator"] != "go" {
		t.Fatalf("expected go generator: %#v", res)
	}
	if jsonutil.Int(res["eventCount"], 0) < 2 {
		t.Fatalf("expected at least 2 events, got %#v", res)
	}
	if !fileio.Exists(filepath.Join(a.cacheDir, "events.cache.json")) {
		t.Fatal("events.cache.json not written")
	}
}

func TestParseICSDateGoUTC(t *testing.T) {
	dt, allDay, ok := parseICSDateGo("20260620T120000Z", nil)
	if !ok || allDay {
		t.Fatalf("bad utc parse ok=%v allDay=%v", ok, allDay)
	}
	if dt.IsZero() {
		t.Fatal("zero time")
	}
}
