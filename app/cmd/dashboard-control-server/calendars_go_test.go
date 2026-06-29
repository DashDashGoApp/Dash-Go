package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTempCalendarApp(t *testing.T) *app {
	t.Helper()
	root := t.TempDir()
	dash := filepath.Join(root, "dashboard")
	home := filepath.Join(root, "home")
	for _, d := range []string{dash, home, filepath.Join(dash, "calendars"), filepath.Join(dash, "config"), filepath.Join(dash, "cache"), filepath.Join(dash, "logs"), filepath.Join(dash, "bin")} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}
	return &app{dash: dash, home: home, configDir: filepath.Join(dash, "config"), calDir: filepath.Join(dash, "calendars"), cacheDir: filepath.Join(dash, "cache"), logDir: filepath.Join(dash, "logs"), binDir: filepath.Join(dash, "bin"), settingsFile: filepath.Join(dash, "config", "settings.json"), configLocal: filepath.Join(dash, "config", "config.local.js"), celebrationsFile: filepath.Join(home, ".dashboard-celebrations")}
}

func TestGenerateCalendarManifestGo(t *testing.T) {
	a := newTempCalendarApp(t)
	os.WriteFile(filepath.Join(a.calDir, "work.red.ics"), []byte("BEGIN:VCALENDAR\nEND:VCALENDAR\n"), 0644)
	os.WriteFile(filepath.Join(a.calDir, "holidays.blue.holiday.ics"), []byte("BEGIN:VCALENDAR\nEND:VCALENDAR\n"), 0644)
	os.WriteFile(filepath.Join(a.home, ".dashboard-disabled-calendars"), []byte("work\n"), 0644)
	if err := a.generateCalendarManifest(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(a.calDir, "calendars.json"))
	if err != nil {
		t.Fatal(err)
	}
	var items []map[string]any
	if err := json.Unmarshal(b, &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one enabled calendar, got %d: %s", len(items), string(b))
	}
	if items[0]["name"] != "Holidays" || items[0]["tag"] != "holiday" || items[0]["color"] != "#8bb4d4" {
		t.Fatalf("unexpected manifest item: %#v", items[0])
	}
}

func TestGenerateDefaultCalendarsGo(t *testing.T) {
	a := newTempCalendarApp(t)
	os.WriteFile(filepath.Join(a.home, ".dashboard-default-calendars"), []byte("DEFAULT_SEASONS=1\nTRASH_WEEKDAY=Monday\nRECYCLING_WEEKDAY=Tuesday\nPAYDAY_MODE=monthly\nPAYDAY_DAY=15\n"), 0600)
	os.WriteFile(a.celebrationsFile, []byte("07-04 | Family BBQ\n2027-01-02 | One-off\n"), 0600)
	res, err := a.generateDefaultCalendars(true)
	if err != nil {
		t.Fatal(err)
	}
	if res["generator"] != "go" {
		t.Fatalf("unexpected generator: %#v", res)
	}
	for _, file := range []string{"seasons.gold.holiday.ics", "trash.amber.ics", "recycling.teal.ics", "payday.violet.pay.ics", "celebrations.gold.ics", "calendars.json"} {
		if _, err := os.Stat(filepath.Join(a.calDir, file)); err != nil {
			t.Fatalf("missing generated %s: %v", file, err)
		}
	}
	b, _ := os.ReadFile(filepath.Join(a.calDir, "celebrations.gold.ics"))
	if !strings.Contains(string(b), "Family BBQ") {
		t.Fatalf("celebration missing from ics: %s", string(b))
	}
}

func TestISSPassesDisabledRemovesFile(t *testing.T) {
	a := newTempCalendarApp(t)
	dest := filepath.Join(a.calDir, "iss.slate.ics")
	os.WriteFile(dest, []byte("old"), 0644)
	res := a.updateISSPasses()
	if res["ok"] != true || res["enabled"] != false {
		t.Fatalf("unexpected result: %#v", res)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("ISS file should be removed when disabled")
	}
}

func useLocalZone(t *testing.T, name string) {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Skipf("time zone %s unavailable: %v", name, err)
	}
	old := time.Local
	time.Local = loc
	t.Cleanup(func() { time.Local = old })
}

func expandedStarts(t *testing.T, events []icsEvent, start, end time.Time) []time.Time {
	t.Helper()
	out := []time.Time{}
	for _, ev := range events {
		for _, inst := range expandEventGo(ev, start, end) {
			out = append(out, inst.Start)
		}
	}
	return out
}

func TestDateOnlyUntilIncludesFinalOccurrenceDay(t *testing.T) {
	useLocalZone(t, "America/Chicago")
	ics := "BEGIN:VCALENDAR\nBEGIN:VEVENT\nUID:until-day\nDTSTART:20260606T090000\nRRULE:FREQ=WEEKLY;BYDAY=SA;UNTIL=20260620\nSUMMARY:Inclusive UNTIL\nEND:VEVENT\nEND:VCALENDAR\n"
	events := parseICSGo(ics, calendarSource{})
	got := expandedStarts(t, events, time.Date(2026, 6, 1, 0, 0, 0, 0, time.Local), time.Date(2026, 6, 30, 23, 59, 59, 0, time.Local))
	if len(got) != 3 || got[2].Format("20060102 1504") != "20260620 0900" {
		t.Fatalf("date-only UNTIL did not retain final day: %#v", got)
	}
}

func TestDateOnlyExdateAndRecurrenceIDSuppressBaseInstance(t *testing.T) {
	useLocalZone(t, "America/Chicago")
	ics := "BEGIN:VCALENDAR\n" +
		"BEGIN:VEVENT\nUID:skip-day\nDTSTART:20260606T090000\nRRULE:FREQ=WEEKLY;BYDAY=SA;COUNT=4\nEXDATE;VALUE=DATE:20260620\nSUMMARY:Skipped date\nEND:VEVENT\n" +
		"BEGIN:VEVENT\nUID:moved-day\nDTSTART:20260606T090000\nRRULE:FREQ=WEEKLY;BYDAY=SA;COUNT=4\nSUMMARY:Moved date\nEND:VEVENT\n" +
		"BEGIN:VEVENT\nUID:moved-day\nRECURRENCE-ID;VALUE=DATE:20260620\nDTSTART:20260621T100000\nSUMMARY:Moved replacement\nEND:VEVENT\nEND:VCALENDAR\n"
	events := parseICSGo(ics, calendarSource{})
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.Local)
	end := time.Date(2026, 6, 30, 23, 59, 59, 0, time.Local)
	baseCounts := map[string]int{}
	for _, ev := range events {
		if ev.RRule == "" {
			continue
		}
		for _, inst := range expandEventGo(ev, start, end) {
			baseCounts[inst.UID+"/"+inst.Start.Format("20060102")]++
		}
	}
	if baseCounts["skip-day/20260620"] != 0 || baseCounts["moved-day/20260620"] != 0 {
		t.Fatalf("date-only exclusion did not suppress base recurrence: %#v", baseCounts)
	}
	if baseCounts["skip-day/20260613"] != 1 || baseCounts["moved-day/20260613"] != 1 {
		t.Fatalf("normal recurrence instances changed unexpectedly: %#v", baseCounts)
	}
}

func TestRecurrenceFastForwardPreservesIntervalAndCount(t *testing.T) {
	useLocalZone(t, "America/Chicago")
	start := time.Date(2026, 1, 1, 9, 0, 0, 0, time.Local)
	windowStart := time.Date(2026, 1, 8, 0, 0, 0, 0, time.Local)
	windowEnd := time.Date(2026, 1, 20, 23, 59, 59, 0, time.Local)
	daily := icsEvent{Start: start, RRule: "FREQ=DAILY;INTERVAL=2;COUNT=6"}
	got := expandEventGo(daily, windowStart, windowEnd)
	if len(got) != 2 || got[0].Start.Format("20060102") != "20260109" || got[1].Start.Format("20060102") != "20260111" {
		t.Fatalf("daily interval/count fast-forward wrong: %#v", got)
	}
	weekly := icsEvent{Start: time.Date(2026, 1, 5, 9, 0, 0, 0, time.Local), RRule: "FREQ=WEEKLY;INTERVAL=2;BYDAY=MO;COUNT=6"}
	got = expandEventGo(weekly, time.Date(2026, 2, 1, 0, 0, 0, 0, time.Local), time.Date(2026, 2, 28, 23, 59, 59, 0, time.Local))
	if len(got) != 2 || got[0].Start.Format("20060102") != "20260202" || got[1].Start.Format("20060102") != "20260216" {
		t.Fatalf("weekly interval/count fast-forward wrong: %#v", got)
	}
}

func TestDailyRecurrenceKeepsWallClockAcrossDST(t *testing.T) {
	useLocalZone(t, "America/Chicago")
	ev := icsEvent{Start: time.Date(2026, 3, 7, 9, 0, 0, 0, time.Local), RRule: "FREQ=DAILY;COUNT=3"}
	got := expandEventGo(ev, time.Date(2026, 3, 6, 0, 0, 0, 0, time.Local), time.Date(2026, 3, 12, 0, 0, 0, 0, time.Local))
	if len(got) != 3 {
		t.Fatalf("daily DST recurrence count = %d", len(got))
	}
	for _, inst := range got {
		if inst.Start.Hour() != 9 {
			t.Fatalf("daily recurrence drifted across DST: %s", inst.Start)
		}
	}
}

func TestParseICSGoIgnoresMalformedPropertyLine(t *testing.T) {
	ics := "BEGIN:VCALENDAR\nBEGIN:VEVENT\nUID:good\nBROKEN-PROPERTY\nDTSTART:20260620T090000\nSUMMARY:Still parsed\nEND:VEVENT\nEND:VCALENDAR\n"
	got := parseICSGo(ics, calendarSource{})
	if len(got) != 1 || got[0].Title != "Still parsed" {
		t.Fatalf("malformed property disrupted parser: %#v", got)
	}
}
