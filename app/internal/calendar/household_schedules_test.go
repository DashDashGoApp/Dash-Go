package calendar

import (
	"os"
	"path/filepath"
	"strings"
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

func TestMonthlyPaydayDatesDeduplicateAfterMonthEndClamp(t *testing.T) {
	rule := PaydayRule{ID: "payday", Label: "Payday", Enabled: true, Kind: "monthly-dates", Days: []int{15, 30, 31}}
	dates := paydayNominalDates(rule, DateOnly(2026, time.February, 1), DateOnly(2026, time.March, 1))
	if len(dates) != 2 {
		t.Fatalf("February monthly dates = %d, want 2 after 30/31 clamp dedupe", len(dates))
	}
	if got := scheduleDateKey(dates[1]); got != "2026-02-28" {
		t.Fatalf("month-end payday = %s, want 2026-02-28", got)
	}
}

func TestHolidayShiftContinuesPastHolidayLandingAndExplainsFallback(t *testing.T) {
	nominal := DateOnly(2026, time.July, 1)
	adjustment := ScheduleAdjustment{Mode: "shift-forward", Days: 1}
	holiday := map[string]bool{"20260701": true, "20260702": true}
	actual, reason, include := scheduleActualDate("trash", nominal, adjustment, holiday, nil)
	if !include || scheduleDateKey(actual) != "2026-07-03" || reason != "holiday schedule" {
		t.Fatalf("holiday landing = %s / %q / include=%v", scheduleDateKey(actual), reason, include)
	}
	blocked := map[string]bool{}
	for day := nominal; day.Before(nominal.AddDate(0, 0, 15)); day = day.AddDate(0, 0, 1) {
		blocked[day.Format("20060102")] = true
	}
	actual, reason, include = scheduleActualDate("trash", nominal, adjustment, blocked, nil)
	if !include || !actual.Equal(nominal) || reason != "no safe pickup day found" {
		t.Fatalf("blocked holiday shift = %s / %q / include=%v", scheduleDateKey(actual), reason, include)
	}
}

func TestOrphanedOverridesDoNotBlockHouseholdScheduleLoad(t *testing.T) {
	service := testService(t)
	raw := `{"schema":1,"paydays":[{"id":"payday","label":"Payday","enabled":true,"kind":"monthly-dates","days":[15],"adjustment":{"mode":"none"}}],"pickups":[],"overrides":[{"ruleId":"retired-rule","nominalDate":"2026-06-15","action":"skip"}]}`
	if err := os.WriteFile(service.householdSchedulesPath(), []byte(raw), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, migrated, err := service.HouseholdSchedules()
	if err != nil || migrated {
		t.Fatalf("load = %#v migrated=%v err=%v", cfg, migrated, err)
	}
	if len(cfg.Overrides) != 0 {
		t.Fatalf("orphaned override survived load: %#v", cfg.Overrides)
	}
	if _, err := service.GenerateDefaults(false); err != nil {
		t.Fatalf("default calendar generation failed because of an orphaned override: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(service.logDir, "household-schedules.log"))
	if err != nil || !strings.Contains(string(body), "dropped orphaned") {
		t.Fatalf("missing orphaned-override diagnostic: %q err=%v", body, err)
	}
}

func TestScheduleMoveBoundsAndSameRuleCollision(t *testing.T) {
	service := testService(t)
	cfg := HouseholdSchedules{Schema: HouseholdSchedulesSchema, Pickups: []PickupRule{{
		ID: "trash", Label: "Trash pickup", Enabled: true, Weekday: "monday", EveryWeeks: 1, Start: "2026-06-01", Adjustment: ScheduleAdjustment{Mode: "none"},
	}}}
	if _, err := service.SaveHouseholdSchedules(cfg); err != nil {
		t.Fatal(err)
	}
	result, err := service.SetHouseholdScheduleOverrideWithResult(ScheduleOverride{RuleID: "trash", NominalDate: "2026-06-01", Action: "move", ActualDate: "2026-06-08"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Collision || result.ActualDate != "2026-06-08" {
		t.Fatalf("move result = %#v, want same-rule collision", result)
	}
	if _, err := service.SetHouseholdScheduleOverrideWithResult(ScheduleOverride{RuleID: "trash", NominalDate: "2026-06-01", Action: "move", ActualDate: "2062-06-01"}); err == nil {
		t.Fatal("out-of-window move was accepted")
	}
}
