package icalwrite

import (
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 7, 2, 21, 0, 0, 0, time.UTC)

func TestSerializeTimedEventEscapesAndFolds(t *testing.T) {
	ev := Event{
		UID:      "dashgo-test-1",
		Title:    "Dinner; tacos, salsa\nBYO drinks",
		Desc:     strings.Repeat("Bring the long list of things we talked about — ", 6),
		Location: "Grandma's house, back yard",
		Start:    time.Date(2026, 7, 15, 22, 0, 0, 0, time.UTC),
		End:      time.Date(2026, 7, 15, 23, 30, 0, 0, time.UTC),
	}
	out, err := ev.Serialize(now)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"BEGIN:VCALENDAR\r\n", "UID:dashgo-test-1\r\n",
		"DTSTART:20260715T220000Z", "DTEND:20260715T233000Z",
		`SUMMARY:Dinner\; tacos\, salsa\nBYO drinks`, "SEQUENCE:0",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	for _, line := range strings.Split(out, "\r\n") {
		if len(line) > 75 {
			t.Fatalf("unfolded line exceeds 75 octets: %q", line)
		}
	}
	if strings.Contains(out, "ORGANIZER") || strings.Contains(out, "ATTENDEE") {
		t.Fatal("writer must never author scheduling properties")
	}
}

func TestSerializeAllDayUsesDateValuesAndValidation(t *testing.T) {
	ev := Event{UID: "d", Title: "Camp week", AllDay: true,
		Start: time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)}
	out, err := ev.Serialize(now)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "DTSTART;VALUE=DATE:20260720") || !strings.Contains(out, "DTEND;VALUE=DATE:20260725") {
		t.Fatalf("all-day encoding wrong:\n%s", out)
	}
	bad := ev
	bad.End = bad.Start
	if _, err := bad.Serialize(now); err == nil {
		t.Fatal("zero-length all-day event must be rejected")
	}
}

const providerEvent = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Apple Inc.//iCloud//EN\r\n" +
	"BEGIN:VEVENT\r\nUID:icloud-abc\r\nDTSTAMP:20260601T000000Z\r\nSEQUENCE:3\r\n" +
	"DTSTART:20260710T170000Z\r\nDTEND:20260710T180000Z\r\nSUMMARY:Dentist\r\n" +
	"X-APPLE-TRAVEL-ADVISORY-BEHAVIOR:AUTOMATIC\r\n" +
	"BEGIN:VALARM\r\nTRIGGER:-PT30M\r\nACTION:DISPLAY\r\nDESCRIPTION:Reminder\r\nEND:VALARM\r\n" +
	"END:VEVENT\r\nEND:VCALENDAR\r\n"

func TestApplyEditPreservesUnknownPropertiesAndBumpsSequence(t *testing.T) {
	edited, err := ApplyEdit(providerEvent, Event{
		UID: "icloud-abc", Title: "Dentist (moved)",
		Start: time.Date(2026, 7, 11, 17, 0, 0, 0, time.UTC),
		End:   time.Date(2026, 7, 11, 18, 0, 0, 0, time.UTC),
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"X-APPLE-TRAVEL-ADVISORY-BEHAVIOR:AUTOMATIC", // provider property preserved
		"BEGIN:VALARM", "TRIGGER:-PT30M", // alarm preserved
		"SEQUENCE:4", "SUMMARY:Dentist (moved)",
		"DTSTART:20260711T170000Z", "DTSTAMP:20260702T210000Z",
	} {
		if !strings.Contains(edited, want) {
			t.Fatalf("missing %q in:\n%s", want, edited)
		}
	}
	if strings.Contains(edited, "20260710T170000Z") {
		t.Fatal("old DTSTART leaked through")
	}
}

func TestApplyEditRefusesSchedulingAndRecurring(t *testing.T) {
	withAttendee := strings.Replace(providerEvent, "SUMMARY:Dentist", "ATTENDEE:mailto:kid@example.com\r\nSUMMARY:Dentist", 1)
	ev := Event{UID: "icloud-abc", Title: "x",
		Start: now, End: now.Add(time.Hour)}
	if _, err := ApplyEdit(withAttendee, ev, now); err == nil {
		t.Fatal("attendee events must be read-only")
	}
	recurring := strings.Replace(providerEvent, "SUMMARY:Dentist", "RRULE:FREQ=WEEKLY\r\nSUMMARY:Dentist", 1)
	if _, err := ApplyEdit(recurring, ev, now); err == nil {
		t.Fatal("recurring events must reject whole-event edits")
	}
	if _, err := ApplyEdit(providerEvent, Event{UID: "other", Title: "x", Start: now, End: now.Add(time.Hour)}, now); err == nil {
		t.Fatal("uid mismatch must be rejected")
	}
}

func TestAppendExdateRemovesOneOccurrence(t *testing.T) {
	recurring := strings.Replace(providerEvent, "SUMMARY:Dentist", "RRULE:FREQ=WEEKLY;BYDAY=FR\r\nSUMMARY:Dentist", 1)
	out, err := AppendExdate(recurring, time.Date(2026, 7, 17, 17, 0, 0, 0, time.UTC), false, now)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "EXDATE:20260717T170000Z") || !strings.Contains(out, "SEQUENCE:4") {
		t.Fatalf("exdate not applied:\n%s", out)
	}
	if _, err := AppendExdate(out, time.Date(2026, 7, 17, 17, 0, 0, 0, time.UTC), false, now); err == nil {
		t.Fatal("duplicate exdate must be rejected")
	}
	if _, err := AppendExdate(providerEvent, now, false, now); err == nil {
		t.Fatal("non-recurring event must reject exdate")
	}
}

func TestUnfoldTangleTolerance(t *testing.T) {
	folded := "BEGIN:VCALENDAR\nBEGIN:VEVENT\nUID:x\nSUMMARY:Long\n  tail\nRRULE:FREQ=DAILY\nEND:VEVENT\nEND:VCALENDAR\n"
	lines := unfold(folded)
	joined := strings.Join(lines, "|")
	if !strings.Contains(joined, "SUMMARY:Long tail") {
		t.Fatalf("continuation join failed: %s", joined)
	}
	if !IsRecurring(folded) {
		t.Fatal("recurrence detection through LF endings failed")
	}
	if UID(folded) != "x" {
		t.Fatal("uid extraction failed")
	}
}

func TestApplyEditNeverRewritesNestedAlarmDescription(t *testing.T) {
	edited, err := ApplyEdit(providerEvent, Event{UID: "icloud-abc", Title: "Dentist moved", Desc: "New event description", Start: time.Date(2026, 7, 11, 17, 0, 0, 0, time.UTC), End: time.Date(2026, 7, 11, 18, 0, 0, 0, time.UTC)}, now)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(edited, "DESCRIPTION:New event description") {
		t.Fatalf("direct description missing:\n%s", edited)
	}
	if !strings.Contains(edited, "BEGIN:VALARM\r\nTRIGGER:-PT30M\r\nACTION:DISPLAY\r\nDESCRIPTION:Reminder\r\nEND:VALARM") {
		t.Fatalf("nested VALARM description changed:\n%s", edited)
	}
}

func TestAppendExdatePreservesTZIDAndRefusesDetachedInstance(t *testing.T) {
	tzMaster := "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nUID:tz-weekly\r\nDTSTART;TZID=America/Chicago:20260706T153000\r\nDTEND;TZID=America/Chicago:20260706T163000\r\nRRULE:FREQ=WEEKLY\r\nSUMMARY:Piano\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	out, err := AppendExdate(tzMaster, time.Date(2026, 7, 13, 20, 30, 0, 0, time.UTC), false, now)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "EXDATE;TZID=America/Chicago:20260713T153000") {
		t.Fatalf("TZID EXDATE wrong:\n%s", out)
	}
	detached := strings.Replace(tzMaster, "RRULE:FREQ=WEEKLY", "RECURRENCE-ID;TZID=America/Chicago:20260713T153000\r\nRRULE:FREQ=WEEKLY", 1)
	if _, err := AppendExdate(detached, time.Now(), false, now); err == nil {
		t.Fatal("detached instance must be refused")
	}
}

func TestRejectsMultiEventAggregate(t *testing.T) {
	aggregate := providerEvent + providerEvent
	if _, err := ApplyEdit(aggregate, Event{UID: "icloud-abc", Title: "x", Start: now, End: now.Add(time.Hour)}, now); err == nil {
		t.Fatal("aggregate ICS must be rejected")
	}
}
