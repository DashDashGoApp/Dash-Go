package main

import (
	"testing"
	"time"
)

func themeTestEvent(title, url, tag string, when time.Time) map[string]any {
	return map[string]any{
		"title": title,
		"start": float64(when.UnixMilli()),
		"cal":   map[string]any{"url": url, "tag": tag},
	}
}

func TestThemeAvailabilityRequiresMatchingEnabledHolidaySource(t *testing.T) {
	now := time.Date(2026, time.December, 25, 9, 0, 0, 0, time.Local)

	withoutLayer := themeAvailabilityForEvents([]map[string]any{
		themeTestEvent("Hanukkah", "calendars/family.ics", "holiday", now),
		themeTestEvent("Kwanzaa", "calendars/family.ics", "", now),
	}, now)
	if withoutLayer.Available["hanukkah"] {
		t.Fatal("a generic holiday calendar must not unlock Hanukkah")
	}
	if withoutLayer.Available["kwanzaa"] {
		t.Fatal("an ordinary family calendar must not unlock Kwanzaa")
	}

	matching := themeAvailabilityForEvents([]map[string]any{
		themeTestEvent("Hanukkah (first day)", "calendars/jewish-holidays.violet.holiday.ics", "holiday", now),
		themeTestEvent("Kwanzaa", "calendars/holidays.blue.holiday.ics", "holiday", now),
	}, now)
	if !matching.Available["hanukkah"] {
		t.Fatal("an enabled Jewish holiday source with Hanukkah today must unlock Hanukkah")
	}
	if !matching.Available["kwanzaa"] {
		t.Fatal("an enabled holiday source with Kwanzaa today must unlock Kwanzaa")
	}
}

func TestThemeAvailabilityIgnoresStaleEvents(t *testing.T) {
	now := time.Date(2026, time.December, 25, 9, 0, 0, 0, time.Local)
	stale := themeAvailabilityForEvents([]map[string]any{
		themeTestEvent("Hanukkah", "calendars/jewish-holidays.violet.holiday.ics", "holiday", now.AddDate(0, 0, -1)),
		themeTestEvent("Kwanzaa", "calendars/holidays.blue.holiday.ics", "holiday", now.AddDate(0, 0, 1)),
	}, now)
	if stale.Available["hanukkah"] || stale.Available["kwanzaa"] {
		t.Fatal("only today’s local holiday events may unlock optional themes")
	}
}

func TestSeasonalThemeUsesExactHolidayEventsBeforeFixedDateFallback(t *testing.T) {
	now := time.Date(2026, time.December, 25, 9, 0, 0, 0, time.Local)
	match := seasonalThemeForEvents([]map[string]any{
		themeTestEvent("Hanukkah", "calendars/jewish-holidays.violet.holiday.ics", "holiday", now),
		themeTestEvent("Christmas Day", "calendars/holidays.blue.holiday.ics", "holiday", now),
	}, now)
	if match.Theme != "hanukkah" {
		t.Fatalf("expected exact Jewish calendar observance to win, got %#v", match)
	}

	match = seasonalThemeForEvents([]map[string]any{
		themeTestEvent("Mother's Day", "calendars/holidays.blue.holiday.ics", "holiday", now),
	}, now)
	if match.Theme != "mothersday" {
		t.Fatalf("expected holiday-tagged exact Mother’s Day event, got %#v", match)
	}
}
