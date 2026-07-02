package icalwrite

import (
	"strings"
	"testing"
	"time"

	events "github.com/DashDashGoApp/Dash-Go/app/internal/calendar/events"
)

// The dashboard's own parser is the first consumer of anything this package
// writes (the merged calendar feeds the events cache). Round-trip every
// authored form through it so the writer can never produce an event the
// dashboard itself cannot display.
func TestWriterOutputRoundTripsThroughProjectParser(t *testing.T) {
	cal := events.CalendarSource{URL: "calendars/family.blue.ics", Name: "Family", Color: "#8bb4d4"}
	tnow := time.Date(2026, 7, 2, 21, 0, 0, 0, time.UTC)

	timed := Event{UID: "rt-timed", Title: "Dinner; tacos, salsa", Desc: "Line one\nLine two", Location: "Home",
		Start: time.Date(2026, 7, 15, 22, 0, 0, 0, time.UTC), End: time.Date(2026, 7, 15, 23, 30, 0, 0, time.UTC)}
	body, err := timed.Serialize(tnow)
	if err != nil {
		t.Fatal(err)
	}
	parsed := events.ParseICS(body, cal)
	if len(parsed) != 1 {
		t.Fatalf("expected 1 event, got %d", len(parsed))
	}
	got := parsed[0]
	if got.UID != "rt-timed" || got.Title != "Dinner; tacos, salsa" || got.AllDay {
		t.Fatalf("timed round-trip mismatch: %+v", got)
	}
	if !got.Start.Equal(timed.Start) || got.End == nil || !got.End.Equal(timed.End) {
		t.Fatalf("times drifted: start=%v end=%v", got.Start, got.End)
	}
	if !strings.Contains(got.Desc, "Line one\nLine two") {
		t.Fatalf("description escaping did not round-trip: %q", got.Desc)
	}

	allDay := Event{UID: "rt-allday", Title: "Camp week", AllDay: true,
		Start: time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)}
	body, err = allDay.Serialize(tnow)
	if err != nil {
		t.Fatal(err)
	}
	parsed = events.ParseICS(body, cal)
	if len(parsed) != 1 || !parsed[0].AllDay {
		t.Fatalf("all-day round-trip mismatch: %+v", parsed)
	}

	// An EXDATE appended by this package must be honored by the project's
	// recurrence expansion (the exact mechanism the dashboard uses).
	recurring := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:rt-weekly\r\n" +
		"DTSTAMP:20260601T000000Z\r\nDTSTART:20260706T210000Z\r\nDTEND:20260706T220000Z\r\n" +
		"RRULE:FREQ=WEEKLY;BYDAY=MO\r\nSUMMARY:Piano\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	edited, err := AppendExdate(recurring, time.Date(2026, 7, 13, 21, 0, 0, 0, time.UTC), false, tnow)
	if err != nil {
		t.Fatal(err)
	}
	parsed = events.ParseICS(edited, cal)
	if len(parsed) != 1 {
		t.Fatalf("expected recurring master, got %d", len(parsed))
	}
	if !parsed[0].Exdates[time.Date(2026, 7, 13, 21, 0, 0, 0, time.UTC).UnixMilli()] {
		t.Fatalf("EXDATE not recognized by project parser: %+v", parsed[0].Exdates)
	}
}
