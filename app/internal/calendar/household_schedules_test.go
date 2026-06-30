package calendar

import (
	"testing"
	"time"
)

func TestHouseholdPaydayMonthlyDatesUseSelectedHolidayLayerAndPreviousBusinessDay(t *testing.T) {
	cfg := HouseholdSchedules{Schema: HouseholdSchedulesSchema, Paydays: []PaydayRule{{
		ID: "spouse", Label: "Spouse payday", Enabled: true, Kind: "monthly-dates", Days: []int{14, 29},
		Adjustment: ScheduleAdjustment{Mode: "previous-business-day", Weekends: true, HolidayLayers: []string{"civil"}},
	}}}
	start, end := DateOnly(2025, time.June, 1), DateOnly(2025, time.July, 1)
	feeds := householdScheduleFeeds(cfg, start, end, map[string]map[string]bool{
		"civil":  {"20250627": true}, // Friday before Sunday June 29.
		"jewish": {"20250614": true}, // Not selected by this rule.
	})
	events := feeds["payday"]
	if len(events) != 2 {
		t.Fatalf("expected two June paydays, got %d", len(events))
	}
	var fourteenth, twentyNinth Event
	for _, event := range events {
		if event.Meta["X-DASHGO-NOMINAL-DATE"] == "2025-06-14" {
			fourteenth = event
		}
		if event.Meta["X-DASHGO-NOMINAL-DATE"] == "2025-06-29" {
			twentyNinth = event
		}
	}
	if got := scheduleDateKey(fourteenth.Date); got != "2025-06-13" { // Saturday backs up; Jewish layer must not make it June 12.
		t.Fatalf("June 14 actual date = %s, want 2025-06-13", got)
	}
	if got := scheduleDateKey(twentyNinth.Date); got != "2025-06-26" { // Sunday -> Saturday -> Friday holiday -> Thursday.
		t.Fatalf("June 29 actual date = %s, want 2025-06-26", got)
	}
	if got := twentyNinth.Meta["X-DASHGO-MANAGED-SCHEDULE"]; got != "payday" {
		t.Fatalf("managed schedule metadata = %q, want payday", got)
	}
}

func TestHouseholdOverrideWinsAndSkipSuppressesOccurrence(t *testing.T) {
	cfg := HouseholdSchedules{Schema: HouseholdSchedulesSchema,
		Paydays:   []PaydayRule{{ID: "payday", Label: "Payday", Enabled: true, Kind: "monthly-dates", Days: []int{14, 29}, Adjustment: ScheduleAdjustment{Mode: "none"}}},
		Overrides: []ScheduleOverride{{RuleID: "payday", NominalDate: "2025-06-14", Action: "move", ActualDate: "2025-06-13"}, {RuleID: "payday", NominalDate: "2025-06-29", Action: "skip"}},
	}
	feeds := householdScheduleFeeds(cfg, DateOnly(2025, time.June, 1), DateOnly(2025, time.July, 1), nil)
	events := feeds["payday"]
	if len(events) != 1 {
		t.Fatalf("expected one non-skipped occurrence, got %d", len(events))
	}
	if got := scheduleDateKey(events[0].Date); got != "2025-06-13" {
		t.Fatalf("override actual = %s, want 2025-06-13", got)
	}
	if got := events[0].Meta["X-DASHGO-NOMINAL-DATE"]; got != "2025-06-14" {
		t.Fatalf("override nominal = %s, want 2025-06-14", got)
	}
	if got := events[0].Meta["X-DASHGO-SCHEDULE-REASON"]; got != "manually moved" {
		t.Fatalf("override reason = %q, want manually moved", got)
	}
}

func TestNormalizeHouseholdSchedulesDeduplicatesMonthlyDatesAndRejectsUnknownPickup(t *testing.T) {
	normalized, err := normalizeHouseholdSchedules(HouseholdSchedules{Schema: HouseholdSchedulesSchema, Paydays: []PaydayRule{{
		ID: "payday", Label: "Payday", Enabled: true, Kind: "monthly-dates", Days: []int{29, 14, 29}, Adjustment: ScheduleAdjustment{Mode: "none"},
	}}})
	if err != nil {
		t.Fatalf("normalize valid schedules: %v", err)
	}
	if got, want := normalized.Paydays[0].Days, []int{14, 29}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("monthly days = %#v, want %#v", got, want)
	}
	_, err = normalizeHouseholdSchedules(HouseholdSchedules{Schema: HouseholdSchedulesSchema, Pickups: []PickupRule{{
		ID: "yard", Label: "Yard", Enabled: true, Weekday: "monday", EveryWeeks: 1, Adjustment: ScheduleAdjustment{Mode: "none"},
	}}})
	if err == nil {
		t.Fatal("unknown pickup id was accepted")
	}
}

func TestLegacyHouseholdSchedulesPreservesExistingInstallerRules(t *testing.T) {
	legacy := legacyHouseholdSchedules(map[string]string{
		"TRASH_WEEKDAY": "monday", "RECYCLING_WEEKDAY": "tuesday", "RECYCLING_EVERY_WEEKS": "2",
		"PICKUP_HOLIDAY_SHIFT": "1", "PICKUP_SHIFT": "forward", "PICKUP_SHIFT_DAYS": "1",
		"PAYDAY_MODE": "biweekly", "PAYDAY_START": "2025-06-06",
	})
	if len(legacy.Pickups) != 2 || len(legacy.Paydays) != 1 {
		t.Fatalf("legacy migration sizes = pickups %d paydays %d", len(legacy.Pickups), len(legacy.Paydays))
	}
	if legacy.Paydays[0].EveryWeeks != 2 || legacy.Paydays[0].Kind != "every-weeks" {
		t.Fatalf("legacy biweekly rule = %#v", legacy.Paydays[0])
	}
	if legacy.Pickups[0].Adjustment.Mode != "shift-forward" || !legacy.Pickups[0].Adjustment.WeekHoliday {
		t.Fatalf("legacy pickup adjustment = %#v", legacy.Pickups[0].Adjustment)
	}
}
