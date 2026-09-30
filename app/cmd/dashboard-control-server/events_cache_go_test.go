package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func TestGoEventCacheBuildsSingleAndRecurringEvents(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	dash := filepath.Join(tmp, "dash")
	a := &app{dash: dash, home: home, configDir: filepath.Join(dash, "config"), calDir: filepath.Join(dash, "calendars"), cacheDir: filepath.Join(dash, "cache"), logDir: filepath.Join(dash, "logs"), binDir: filepath.Join(dash, "bin"), settingsFile: filepath.Join(dash, "config", "settings.json"), configLocal: filepath.Join(dash, "config", "config.local.js"), celebrationsFile: filepath.Join(home, ".dashboard-celebrations")}
	a.ensureDirs()
	now := time.Now()
	oneStart := now.AddDate(0, 0, 5)
	oneEnd := oneStart.Add(time.Hour)
	recStart := now.AddDate(0, 0, -3)
	recEnd := recStart.Add(30 * time.Minute)
	ics := fmt.Sprintf(`BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:one@test
DTSTART:%s
DTEND:%s
SUMMARY:One-time Test
LOCATION:Kitchen
END:VEVENT
BEGIN:VEVENT
UID:weekly@test
DTSTART:%s
DTEND:%s
RRULE:FREQ=DAILY;COUNT=4
SUMMARY:Weekly Test
END:VEVENT
END:VCALENDAR
`, oneStart.Format("20060102T150405"), oneEnd.Format("20060102T150405"), recStart.Format("20060102T150405"), recEnd.Format("20060102T150405"))
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
