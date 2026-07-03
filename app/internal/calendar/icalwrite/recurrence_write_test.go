package icalwrite

import (
	"strings"
	"testing"
	"time"
)

const recurrenceMaster = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Provider//EN\r\n" +
	"BEGIN:VEVENT\r\nUID:weekly-family\r\nDTSTAMP:20260701T000000Z\r\nSEQUENCE:3\r\n" +
	"DTSTART;TZID=America/Chicago:20260706T153000\r\nDTEND;TZID=America/Chicago:20260706T163000\r\n" +
	"RRULE:FREQ=WEEKLY;COUNT=8\r\nEXDATE;TZID=America/Chicago:20260720T153000\r\nSUMMARY:Piano\r\n" +
	"X-PROVIDER-KEEP:yes\r\nBEGIN:VALARM\r\nTRIGGER:-PT15M\r\nACTION:DISPLAY\r\nDESCRIPTION:Reminder\r\nEND:VALARM\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"

func TestApplyOccurrenceEditCreatesAndUpdatesOneDetachedInstance(t *testing.T) {
	occurrence := time.Date(2026, 7, 13, 20, 30, 0, 0, time.UTC) // 3:30 PM Chicago
	moved := Event{UID: "weekly-family", Title: "Piano moved", Location: "Studio", Start: time.Date(2026, 7, 14, 21, 0, 0, 0, time.UTC), End: time.Date(2026, 7, 14, 22, 0, 0, 0, time.UTC)}
	out, err := ApplyOccurrenceEdit(recurrenceMaster, "weekly-family", occurrence, moved, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"RRULE:FREQ=WEEKLY;COUNT=8",
		"EXDATE;TZID=America/Chicago:20260720T153000",
		"X-PROVIDER-KEEP:yes",
		"BEGIN:VALARM\r\nTRIGGER:-PT15M\r\nACTION:DISPLAY\r\nDESCRIPTION:Reminder\r\nEND:VALARM",
		"RECURRENCE-ID;TZID=America/Chicago:20260713T153000",
		"DTSTART;TZID=America/Chicago:20260714T160000",
		"SUMMARY:Piano moved",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("occurrence edit missing %q:\n%s", want, out)
		}
	}
	if got := strings.Count(out, "BEGIN:VEVENT"); got != 2 {
		t.Fatalf("VEVENT count=%d want 2:\n%s", got, out)
	}
	moved.Title = "Piano moved again"
	out, err = ApplyOccurrenceEdit(out, "weekly-family", occurrence, moved, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(out, "BEGIN:VEVENT"); got != 2 {
		t.Fatalf("existing detached exception duplicated: %d:\n%s", got, out)
	}
	if !strings.Contains(out, "SUMMARY:Piano moved again") {
		t.Fatalf("existing detached exception was not updated:\n%s", out)
	}
}

func TestApplySeriesEditPreservesRecurrenceAndProviderFields(t *testing.T) {
	changed := Event{UID: "weekly-family", Title: "Piano lesson", Desc: "Bring books", Location: "New studio", Start: time.Date(2026, 7, 7, 21, 0, 0, 0, time.UTC), End: time.Date(2026, 7, 7, 22, 0, 0, 0, time.UTC)}
	out, err := ApplySeriesEdit(recurrenceMaster, changed, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"RRULE:FREQ=WEEKLY;COUNT=8",
		// The exclusion keeps its civil day but adopts the new series time.
		// Retaining the literal 15:30 value would orphan the exclusion and
		// silently resurrect the skipped July 20 occurrence everywhere.
		"EXDATE;TZID=America/Chicago:20260720T160000",
		"X-PROVIDER-KEEP:yes",
		"BEGIN:VALARM\r\nTRIGGER:-PT15M\r\nACTION:DISPLAY\r\nDESCRIPTION:Reminder\r\nEND:VALARM",
		"DTSTART;TZID=America/Chicago:20260707T160000",
		"SUMMARY:Piano lesson",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("series edit missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "EXDATE;TZID=America/Chicago:20260720T153000") {
		t.Fatalf("series edit left an orphaned exclusion at the old time:\n%s", out)
	}
}

func TestApplySeriesEditToAllDayConvertsExdatesToDateForm(t *testing.T) {
	changed := Event{UID: "weekly-family", Title: "Piano day", AllDay: true, Start: time.Date(2026, 7, 7, 0, 0, 0, 0, time.UTC), End: time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)}
	out, err := ApplySeriesEdit(recurrenceMaster, changed, now)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "EXDATE;VALUE=DATE:20260720") {
		t.Fatalf("all-day series edit must carry exclusions as dates:\n%s", out)
	}
	if !strings.Contains(out, "DTSTART;VALUE=DATE:20260707") {
		t.Fatalf("all-day series edit missing date DTSTART:\n%s", out)
	}
}

func TestApplyOccurrenceEditMatchesUTCFormOverride(t *testing.T) {
	occurrence := time.Date(2026, 7, 13, 20, 30, 0, 0, time.UTC) // 3:30 PM Chicago
	first := Event{UID: "weekly-family", Title: "Piano moved", Start: time.Date(2026, 7, 14, 21, 0, 0, 0, time.UTC), End: time.Date(2026, 7, 14, 22, 0, 0, 0, time.UTC)}
	out, err := ApplyOccurrenceEdit(recurrenceMaster, "weekly-family", occurrence, first, now)
	if err != nil {
		t.Fatal(err)
	}
	// A provider may re-encode the same instant in UTC form even though the
	// master uses TZID form. Editing that occurrence again must revise the
	// existing override, never append a duplicate for the same instance.
	utcForm := strings.Replace(out, "RECURRENCE-ID;TZID=America/Chicago:20260713T153000", "RECURRENCE-ID:20260713T203000Z", 1)
	first.Title = "Piano moved again"
	out, err = ApplyOccurrenceEdit(utcForm, "weekly-family", occurrence, first, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(out, "BEGIN:VEVENT"); got != 2 {
		t.Fatalf("UTC-form override duplicated (VEVENT count=%d):\n%s", got, out)
	}
	if !strings.Contains(out, "SUMMARY:Piano moved again") {
		t.Fatalf("UTC-form override was not revised:\n%s", out)
	}
}

func TestApplyOccurrenceEditRefusesThisAndFutureOverride(t *testing.T) {
	occurrence := time.Date(2026, 7, 13, 20, 30, 0, 0, time.UTC)
	first := Event{UID: "weekly-family", Title: "Piano moved", Start: time.Date(2026, 7, 14, 21, 0, 0, 0, time.UTC), End: time.Date(2026, 7, 14, 22, 0, 0, 0, time.UTC)}
	out, err := ApplyOccurrenceEdit(recurrenceMaster, "weekly-family", occurrence, first, now)
	if err != nil {
		t.Fatal(err)
	}
	ranged := strings.Replace(out, "RECURRENCE-ID;TZID=America/Chicago:20260713T153000", "RECURRENCE-ID;RANGE=THISANDFUTURE;TZID=America/Chicago:20260713T153000", 1)
	if _, err := ApplyOccurrenceEdit(ranged, "weekly-family", occurrence, first, now.Add(time.Minute)); err == nil {
		t.Fatal("this-and-future override must be refused, not duplicated")
	}
}

func TestSeriesEditableRefusesAdvancedRulesAndExistingExceptions(t *testing.T) {
	if !SeriesEditable(recurrenceMaster, "weekly-family") {
		t.Fatal("simple weekly series should be editable")
	}
	advanced := strings.Replace(recurrenceMaster, "FREQ=WEEKLY;COUNT=8", "FREQ=MONTHLY;BYDAY=MO;BYSETPOS=2", 1)
	if SeriesEditable(advanced, "weekly-family") {
		t.Fatal("selector-heavy recurrence must remain provider-managed")
	}
	withException, err := ApplyOccurrenceEdit(recurrenceMaster, "weekly-family", time.Date(2026, 7, 13, 20, 30, 0, 0, time.UTC), Event{UID: "weekly-family", Title: "Piano", Start: time.Date(2026, 7, 13, 20, 30, 0, 0, time.UTC), End: time.Date(2026, 7, 13, 21, 30, 0, 0, time.UTC)}, now)
	if err != nil {
		t.Fatal(err)
	}
	if SeriesEditable(withException, "weekly-family") {
		t.Fatal("series with detached exception must stay provider-managed")
	}
	if _, err := ApplySeriesEdit(withException, Event{UID: "weekly-family", Title: "unsafe", Start: time.Date(2026, 7, 7, 21, 0, 0, 0, time.UTC), End: time.Date(2026, 7, 7, 22, 0, 0, 0, time.UTC)}, now); err == nil {
		t.Fatal("series edit with detached exception must reject")
	}
}

func TestApplyOccurrenceEditRefusesSchedulingAndMixedUIDs(t *testing.T) {
	attendee := strings.Replace(recurrenceMaster, "SUMMARY:Piano", "ATTENDEE:mailto:family@example.test\r\nSUMMARY:Piano", 1)
	if _, err := ApplyOccurrenceEdit(attendee, "weekly-family", time.Date(2026, 7, 13, 20, 30, 0, 0, time.UTC), Event{UID: "weekly-family", Title: "x", Start: now, End: now.Add(time.Hour)}, now); err == nil {
		t.Fatal("attendee event accepted for recurring edit")
	}
	mixed := strings.Replace(recurrenceMaster, "UID:weekly-family\r\nDTSTAMP", "UID:other\r\nDTSTAMP", 1)
	if HasUID(mixed, "weekly-family") {
		t.Fatal("mixed UID item reported as safe")
	}
}

func TestOccurrenceOverrideUsesMasterSequenceAndSkipDetectsIt(t *testing.T) {
	occurrence := time.Date(2026, 7, 13, 20, 30, 0, 0, time.UTC)
	moved := Event{UID: "weekly-family", Title: "Moved", Start: time.Date(2026, 7, 14, 21, 0, 0, 0, time.UTC), End: time.Date(2026, 7, 14, 22, 0, 0, 0, time.UTC)}
	out, err := ApplyOccurrenceEdit(recurrenceMaster, "weekly-family", occurrence, moved, now)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "RECURRENCE-ID;TZID=America/Chicago:20260713T153000\r\nDTSTAMP") || !strings.Contains(out, "SEQUENCE:3") {
		t.Fatalf("new detached occurrence must inherit the master sequence:\n%s", out)
	}
	overridden, err := OccurrenceHasOverride(out, "weekly-family", occurrence)
	if err != nil {
		t.Fatal(err)
	}
	if !overridden {
		t.Fatal("matching detached override was not detected")
	}
}

func TestApplySeriesEditUTCExdateFollowsDTSTARTAcrossUTCMidnight(t *testing.T) {
	src := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\n" +
		"UID:utc-weekly\r\nDTSTAMP:20250101T000000Z\r\n" +
		"DTSTART:20250107T010000Z\r\nDTEND:20250107T020000Z\r\n" +
		"RRULE:FREQ=WEEKLY;COUNT=4\r\nEXDATE:20250114T010000Z\r\n" +
		"SUMMARY:Evening\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	// The original master is 7:00 PM CST. Moving it to 5:00 PM CST changes
	// DTSTART from Jan 7 01:00Z to Jan 6 23:00Z. The excluded next occurrence
	// must become Jan 13 23:00Z, not Jan 14 23:00Z.
	changed := Event{
		UID:   "utc-weekly",
		Title: "Earlier evening",
		Start: time.Date(2025, 1, 6, 23, 0, 0, 0, time.UTC),
		End:   time.Date(2025, 1, 7, 0, 0, 0, 0, time.UTC),
	}
	out, err := ApplySeriesEdit(src, changed, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"DTSTART:20250106T230000Z",
		"EXDATE:20250113T230000Z",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("UTC series edit missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "EXDATE:20250114T230000Z") {
		t.Fatalf("UTC series edit retained the old UTC civil day:\n%s", out)
	}
}

func TestApplySeriesEditTZIDExdateKeepsCivilDayAcrossDST(t *testing.T) {
	src := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\n" +
		"UID:dst-weekly\r\nDTSTAMP:20260201T000000Z\r\n" +
		"DTSTART;TZID=America/Chicago:20260302T190000\r\nDTEND;TZID=America/Chicago:20260302T200000\r\n" +
		"RRULE:FREQ=WEEKLY;COUNT=4\r\nEXDATE;TZID=America/Chicago:20260309T190000\r\n" +
		"SUMMARY:Evening\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	changed := Event{
		UID:   "dst-weekly",
		Title: "Earlier evening",
		// March 2 is CST (UTC-6), so 5:00 PM is 23:00Z.
		Start: time.Date(2026, 3, 2, 23, 0, 0, 0, time.UTC),
		End:   time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC),
	}
	out, err := ApplySeriesEdit(src, changed, now)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "DTSTART;TZID=America/Chicago:20260302T170000") {
		t.Fatalf("TZID series edit did not use the new wall time:\n%s", out)
	}
	// March 9 is after the DST shift; the exclusion must retain March 9 in
	// Chicago and take the new 5:00 PM wall time.
	if !strings.Contains(out, "EXDATE;TZID=America/Chicago:20260309T170000") {
		t.Fatalf("TZID exclusion did not retain its civil day across DST:\n%s", out)
	}
}

func TestApplySeriesEditFloatingExdateKeepsCivilDay(t *testing.T) {
	previousLocal := time.Local
	time.Local = time.FixedZone("dashboard-test", -6*60*60)
	t.Cleanup(func() { time.Local = previousLocal })

	src := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\n" +
		"UID:floating-weekly\r\nDTSTAMP:20260101T000000Z\r\n" +
		"DTSTART:20260105T190000\r\nDTEND:20260105T200000\r\n" +
		"RRULE:FREQ=WEEKLY;COUNT=4\r\nEXDATE:20260112T190000\r\n" +
		"SUMMARY:Evening\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	changed := Event{
		UID:   "floating-weekly",
		Title: "Earlier evening",
		Start: time.Date(2026, 1, 5, 17, 0, 0, 0, time.Local),
		End:   time.Date(2026, 1, 5, 18, 0, 0, 0, time.Local),
	}
	out, err := ApplySeriesEdit(src, changed, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"DTSTART:20260105T170000",
		"EXDATE:20260112T170000",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("floating series edit missing %q:\n%s", want, out)
		}
	}
}
