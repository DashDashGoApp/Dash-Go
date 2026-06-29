package events

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func fixtureEvents(t *testing.T, name string) []ICSEvent {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return ParseICS(string(body), CalendarSource{})
}

func expandedByUID(t *testing.T, events []ICSEvent, start, end time.Time) map[string][]ICSEvent {
	t.Helper()
	out := map[string][]ICSEvent{}
	for _, event := range events {
		out[event.UID] = Expand(event, start, end)
	}
	return out
}

func dates(values []ICSEvent) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, value.Start.Format("20060102"))
	}
	return out
}

func equalStrings(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestRDateAndExdateComposeTheRecurrenceSet(t *testing.T) {
	events := fixtureEvents(t, "rdate-exdate.ics")
	windowStart := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2026, 6, 30, 23, 59, 59, 0, time.UTC)
	got := expandedByUID(t, events, windowStart, windowEnd)
	equalStrings(t, dates(got["rdate-series@example.test"]), []string{"20260601", "20260615", "20260618", "20260622"})
	equalStrings(t, dates(got["rdate-only@example.test"]), []string{"20260602", "20260604"})
}

func TestTZIDAndVTimezoneRetainSourceWallClockAcrossDST(t *testing.T) {
	events := fixtureEvents(t, "timezones.ics")
	windowStart := time.Date(2026, 3, 6, 0, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2026, 3, 13, 23, 59, 59, 0, time.UTC)
	got := expandedByUID(t, events, windowStart, windowEnd)
	for _, uid := range []string{"iana-dst@example.test", "custom-dst@example.test"} {
		values := got[uid]
		if len(values) != 4 {
			t.Fatalf("%s values=%#v", uid, values)
		}
		for _, value := range values {
			if value.Start.Hour() != 9 || value.Start.Minute() != 0 {
				t.Fatalf("%s wall clock drifted: %s", uid, value.Start)
			}
		}
		if _, before := values[0].Start.Zone(); before != -5*3600 {
			t.Fatalf("%s pre-DST offset=%d", uid, before)
		}
		if _, after := values[1].Start.Zone(); after != -4*3600 {
			t.Fatalf("%s post-DST offset=%d", uid, after)
		}
	}
}

func TestWeeklyWKSTUsesRFCDefaultAndChangesIntervalCycles(t *testing.T) {
	events := fixtureEvents(t, "weekly-wkst.ics")
	windowStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2026, 2, 10, 23, 59, 59, 0, time.UTC)
	got := expandedByUID(t, events, windowStart, windowEnd)
	monday := []string{"20260104", "20260112", "20260118", "20260126", "20260201"}
	equalStrings(t, dates(got["wkst-default@example.test"]), monday)
	equalStrings(t, dates(got["wkst-monday@example.test"]), monday)
	equalStrings(t, dates(got["wkst-sunday@example.test"]), []string{"20260104", "20260105", "20260118", "20260119", "20260201"})
}

func TestYearlyByMonthByDayAndLeapDaySelectors(t *testing.T) {
	events := fixtureEvents(t, "yearly-selectors.ics")
	windowStart := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2033, 12, 31, 23, 59, 59, 0, time.UTC)
	got := expandedByUID(t, events, windowStart, windowEnd)
	equalStrings(t, dates(got["thanksgiving@example.test"]), []string{"20261126", "20271125", "20281123"})
	equalStrings(t, dates(got["leap-day@example.test"]), []string{"20240229", "20280229"})
}

func TestDateTimeUntilIsInclusiveInSourceZone(t *testing.T) {
	ics := "BEGIN:VCALENDAR\nBEGIN:VEVENT\nUID:until@example.test\nDTSTART;TZID=America/New_York:20260307T090000\nRRULE:FREQ=DAILY;COUNT=6;UNTIL=20260310T090000\nSUMMARY:Inclusive until\nEND:VEVENT\nEND:VCALENDAR\n"
	events := ParseICS(ics, CalendarSource{})
	values := Expand(events[0], time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC))
	equalStrings(t, dates(values), []string{"20260307", "20260308", "20260309", "20260310"})
}

func TestRefreshRebuildsPriorRecurrenceSemanticCache(t *testing.T) {
	service := testService(t)
	ics := "BEGIN:VCALENDAR\nBEGIN:VEVENT\nUID:cache-version@example.test\nDTSTART:20260601T090000\nRRULE:FREQ=YEARLY;BYMONTH=11;BYDAY=4TH\nSUMMARY:Cache version\nEND:VEVENT\nEND:VCALENDAR\n"
	if err := os.WriteFile(filepath.Join(service.CalendarDir(), "interop.ics"), []byte(ics), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Refresh(true, 30, 365); err != nil {
		t.Fatal(err)
	}
	cachePath := filepath.Join(service.CacheDir(), "events.cache.json")
	cache := jsonutil.Map(readJSONDefault(cachePath, map[string]any{}))
	cache["version"] = CacheVersion - 1
	if err := fileio.WriteJSON(cachePath, cache); err != nil {
		t.Fatal(err)
	}
	result, err := service.Refresh(false, 30, 365)
	if err != nil {
		t.Fatal(err)
	}
	if result["unchanged"] == true {
		t.Fatalf("old cache version was reused: %#v", result)
	}
	updated := jsonutil.Map(readJSONDefault(cachePath, map[string]any{}))
	if jsonutil.Int(updated["version"], 0) != CacheVersion {
		t.Fatalf("cache version=%#v", updated)
	}
}
