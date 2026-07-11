package events

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseICSReconcilesRevisionsCancellationAndDuration(t *testing.T) {
	cal := CalendarSource{URL: "calendars/test.ics"}
	ics := "BEGIN:VCALENDAR\n" +
		"BEGIN:VEVENT\nUID:revised\nSEQUENCE:1\nDTSTAMP:20260701T120000Z\nDTSTART:20260712T090000Z\nSUMMARY:Old title\nEND:VEVENT\n" +
		"BEGIN:VEVENT\nUID:revised\nSEQUENCE:2\nDTSTAMP:20260702T120000Z\nDTSTART:20260712T090000Z\nDURATION:PT90M\nSUMMARY:New title\nEND:VEVENT\n" +
		"BEGIN:VEVENT\nUID:cancelled\nSTATUS:CANCELLED\nDTSTART:20260712T100000Z\nSUMMARY:Cancelled\nEND:VEVENT\n" +
		"END:VCALENDAR\n"
	events := parseICS(ics, cal)
	if len(events) != 1 {
		t.Fatalf("events=%#v, want one reconciled non-cancelled event", events)
	}
	if events[0].Title != "New title" || events[0].End == nil || events[0].End.Sub(events[0].Start) != 90*time.Minute {
		t.Fatalf("reconciled event=%#v", events[0])
	}
}

func TestICSDurationKeepsLocalWallTimeAcrossDST(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("timezone data unavailable: %v", err)
	}
	duration, ok := parseICSDuration("P1D")
	if !ok {
		t.Fatal("P1D duration was rejected")
	}
	start := time.Date(2026, 3, 7, 12, 0, 0, 0, location)
	end := duration.addTo(start)
	if end.Day() != 8 || end.Hour() != 12 {
		t.Fatalf("end=%v, want next local day at 12:00", end)
	}
	if end.Sub(start) != 23*time.Hour {
		t.Fatalf("elapsed=%s, want 23h across spring DST", end.Sub(start))
	}
}

func TestCancelledRecurrenceOverrideSuppressesMasterOccurrence(t *testing.T) {
	cal := CalendarSource{URL: "calendars/test.ics"}
	ics := "BEGIN:VCALENDAR\n" +
		"BEGIN:VEVENT\nUID:series\nDTSTART:20260706T090000Z\nRRULE:FREQ=WEEKLY;COUNT=3\nSUMMARY:Series\nEND:VEVENT\n" +
		"BEGIN:VEVENT\nUID:series\nRECURRENCE-ID:20260713T090000Z\nSTATUS:CANCELLED\nDTSTART:20260713T090000Z\nSUMMARY:Cancelled occurrence\nEND:VEVENT\n" +
		"END:VCALENDAR\n"
	events := parseICS(ics, cal)
	if len(events) != 1 || events[0].RRule == "" {
		t.Fatalf("parsed events=%#v", events)
	}
	instances := expand(events[0], time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 7, 31, 23, 59, 59, 0, time.UTC))
	if len(instances) != 2 || instances[0].Start.Day() != 6 || instances[1].Start.Day() != 20 {
		t.Fatalf("instances=%#v", instances)
	}
}

func TestCancelledMasterSuppressesSeriesOverrides(t *testing.T) {
	cal := CalendarSource{URL: "calendars/test.ics"}
	ics := "BEGIN:VCALENDAR\n" +
		"BEGIN:VEVENT\nUID:cancelled-series\nSTATUS:CANCELLED\nDTSTART:20260706T090000Z\nRRULE:FREQ=WEEKLY;COUNT=3\nSUMMARY:Cancelled series\nEND:VEVENT\n" +
		"BEGIN:VEVENT\nUID:cancelled-series\nRECURRENCE-ID:20260713T090000Z\nDTSTART:20260713T100000Z\nSUMMARY:Old exception\nEND:VEVENT\n" +
		"END:VCALENDAR\n"
	if events := parseICS(ics, cal); len(events) != 0 {
		t.Fatalf("cancelled master retained events=%#v", events)
	}
}

func TestSourceMetaReusesDigestWhenStatIdentityMatches(t *testing.T) {
	s := testService(t)
	path := filepath.Join(s.CalendarDir(), "digest.ics")
	if err := os.WriteFile(path, []byte("first"), 0644); err != nil {
		t.Fatal(err)
	}
	first := s.sourceMeta("calendars/digest.ics", "Digest", "", "", "", path, nil)
	if first.SHA256 == nil || first.MtimeNs == nil || first.Size == nil {
		t.Fatalf("first metadata=%#v", first)
	}
	second := s.sourceMeta("calendars/digest.ics", "Digest", "", "", "", path, &first)
	if second.SHA256 == nil || *second.SHA256 != *first.SHA256 {
		t.Fatalf("digest was not reused: first=%#v second=%#v", first, second)
	}
}

func TestCacheWindowHonorsConfiguredWeekStart(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC) // Wednesday
	mondayStart, _ := cacheWindow(now, 0, 7, 1)
	sundayStart, _ := cacheWindow(now, 0, 7, 0)
	if mondayStart.Weekday() != time.Monday || sundayStart.Weekday() != time.Sunday {
		t.Fatalf("monday=%v sunday=%v", mondayStart, sundayStart)
	}
}

func TestSerializedEventIdentityDoesNotDependOnEmissionOrder(t *testing.T) {
	s := testService(t)
	cal := CalendarSource{URL: "calendars/personal.ics", Name: "Personal"}
	event := ICSEvent{UID: "stable-uid", Title: "Stable", Start: time.Date(2026, 7, 12, 9, 0, 0, 0, time.UTC)}
	first := s.serializeEvent(event, cal)
	second := s.serializeEvent(event, cal)
	if first["id"] == "" || first["id"] != second["id"] {
		t.Fatalf("ids=%#v / %#v", first["id"], second["id"])
	}
}

func TestSourceMetaPeriodicallyRehashesSameStatIdentity(t *testing.T) {
	root := t.TempDir()
	calDir := filepath.Join(root, "calendars")
	if err := os.MkdirAll(calDir, 0755); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	s := New(ServiceConfig{
		DashDir:     root,
		CalendarDir: calDir,
		CacheDir:    filepath.Join(root, "cache"),
		Now:         func() time.Time { return now },
	})
	path := filepath.Join(calDir, "digest.ics")
	if err := os.WriteFile(path, []byte("first"), 0644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	first := s.sourceMeta("calendars/digest.ics", "Digest", "", "", "", path, nil)
	if first.SHA256 == nil || first.HashedAt == nil {
		t.Fatalf("first metadata=%#v", first)
	}
	if err := os.WriteFile(path, []byte("other"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	now = now.Add(25 * time.Hour)
	second := s.sourceMeta("calendars/digest.ics", "Digest", "", "", "", path, &first)
	if second.SHA256 == nil || *second.SHA256 == *first.SHA256 {
		t.Fatalf("periodic rehash did not detect same-size, same-mtime replacement: first=%#v second=%#v", first, second)
	}
	if second.HashedAt == nil || *second.HashedAt <= *first.HashedAt {
		t.Fatalf("periodic rehash timestamp did not advance: first=%#v second=%#v", first.HashedAt, second.HashedAt)
	}
}
