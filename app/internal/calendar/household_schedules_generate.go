package calendar

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

func householdScheduleFeeds(cfg HouseholdSchedules, start, end time.Time, holidaysByLayer map[string]map[string]bool) map[string][]Event {
	out := map[string][]Event{"trash": {}, "recycling": {}, "payday": {}}
	overrides := scheduleOverrideIndex(cfg.Overrides)
	for _, rule := range cfg.Pickups {
		if !rule.Enabled {
			continue
		}
		key := "trash"
		if rule.ID == "recycling" || strings.Contains(strings.ToLower(rule.Label), "recycl") {
			key = "recycling"
		}
		out[key] = append(out[key], pickupRuleEvents(rule, start, end, holidaysByLayer, overrides)...)
	}
	for _, rule := range cfg.Paydays {
		if !rule.Enabled {
			continue
		}
		out["payday"] = append(out["payday"], paydayRuleEvents(rule, start, end, holidaysByLayer, overrides)...)
	}
	for _, events := range out {
		sort.SliceStable(events, func(i, j int) bool {
			if !events[i].Date.Equal(events[j].Date) {
				return events[i].Date.Before(events[j].Date)
			}
			return events[i].Summary < events[j].Summary
		})
	}
	return out
}

func scheduleHolidayDates(adjustment ScheduleAdjustment, byLayer map[string]map[string]bool) map[string]bool {
	out := map[string]bool{}
	for _, layer := range adjustment.HolidayLayers {
		for day := range byLayer[layer] {
			out[day] = true
		}
	}
	return out
}

func scheduleOverrideIndex(rows []ScheduleOverride) map[string]ScheduleOverride {
	out := map[string]ScheduleOverride{}
	for _, row := range rows {
		out[row.RuleID+"|"+row.NominalDate] = row
	}
	return out
}

func pickupRuleEvents(rule PickupRule, start, end time.Time, holidaysByLayer map[string]map[string]bool, overrides map[string]ScheduleOverride) []Event {
	holidays := scheduleHolidayDates(rule.Adjustment, holidaysByLayer)
	out := []Event{}
	for _, nominal := range pickupNominalDates(rule, start.AddDate(0, 0, -14), end.AddDate(0, 0, 14)) {
		actual, reason, include := scheduleActualDate(rule.ID, nominal, rule.Adjustment, holidays, overrides)
		if !include || actual.Before(start) || !actual.Before(end) {
			continue
		}
		out = append(out, managedScheduleEvent(rule.ID, "pickup", rule.Label, nominal, actual, reason))
	}
	return out
}

func paydayRuleEvents(rule PaydayRule, start, end time.Time, holidaysByLayer map[string]map[string]bool, overrides map[string]ScheduleOverride) []Event {
	holidays := scheduleHolidayDates(rule.Adjustment, holidaysByLayer)
	out := []Event{}
	for _, nominal := range paydayNominalDates(rule, start.AddDate(0, 0, -14), end.AddDate(0, 0, 14)) {
		actual, reason, include := scheduleActualDate(rule.ID, nominal, rule.Adjustment, holidays, overrides)
		if !include || actual.Before(start) || !actual.Before(end) {
			continue
		}
		out = append(out, managedScheduleEvent(rule.ID, "payday", rule.Label, nominal, actual, reason))
	}
	return out
}

func pickupNominalDates(rule PickupRule, start, end time.Time) []time.Time {
	weekday := weekdayIndex(rule.Weekday)
	if weekday < 0 {
		return nil
	}
	day := start
	if anchor, ok := scheduleDate(rule.Start); ok {
		day = anchor
		for day.Before(start) {
			day = day.AddDate(0, 0, 7*rule.EveryWeeks)
		}
		for day.AddDate(0, 0, -7*rule.EveryWeeks).After(start) {
			day = day.AddDate(0, 0, -7*rule.EveryWeeks)
		}
	} else {
		for int(day.Weekday()+6)%7 != weekday {
			day = day.AddDate(0, 0, 1)
		}
	}
	out := []time.Time{}
	for day.Before(end) {
		if !day.Before(start) {
			out = append(out, day)
		}
		day = day.AddDate(0, 0, 7*rule.EveryWeeks)
	}
	return out
}

func paydayNominalDates(rule PaydayRule, start, end time.Time) []time.Time {
	out := []time.Time{}
	switch rule.Kind {
	case "every-weeks":
		anchor, ok := scheduleDate(rule.Start)
		if !ok {
			return out
		}
		day := anchor
		for day.Before(start) {
			day = day.AddDate(0, 0, 7*rule.EveryWeeks)
		}
		for day.Before(end) {
			out = append(out, day)
			day = day.AddDate(0, 0, 7*rule.EveryWeeks)
		}
	case "monthly-dates":
		for cursor := DateOnly(start.Year(), start.Month(), 1); cursor.Before(end); cursor = cursor.AddDate(0, 1, 0) {
			for _, requested := range rule.Days {
				last := cursor.AddDate(0, 1, -1).Day()
				day := requested
				if day > last {
					day = last
				}
				value := DateOnly(cursor.Year(), cursor.Month(), day)
				if !value.Before(start) && value.Before(end) {
					out = append(out, value)
				}
			}
		}
	case "nth-weekday":
		weekday := weekdayIndex(rule.Weekday)
		if weekday < 0 {
			return out
		}
		for cursor := DateOnly(start.Year(), start.Month(), 1); cursor.Before(end); cursor = cursor.AddDate(0, 1, 0) {
			value := nthWeekdayOfMonth(cursor.Year(), cursor.Month(), weekday, rule.Nth)
			if !value.IsZero() && !value.Before(start) && value.Before(end) {
				out = append(out, value)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Before(out[j]) })
	return out
}

func nthWeekdayOfMonth(year int, month time.Month, weekday int, nth int) time.Time {
	if nth == -1 {
		day := DateOnly(year, month, 1).AddDate(0, 1, -1)
		for int(day.Weekday()+6)%7 != weekday {
			day = day.AddDate(0, 0, -1)
		}
		return day
	}
	day := DateOnly(year, month, 1)
	for int(day.Weekday()+6)%7 != weekday {
		day = day.AddDate(0, 0, 1)
	}
	day = day.AddDate(0, 0, 7*(nth-1))
	if day.Month() != month {
		return time.Time{}
	}
	return day
}

func scheduleActualDate(ruleID string, nominal time.Time, adjustment ScheduleAdjustment, holidays map[string]bool, overrides map[string]ScheduleOverride) (time.Time, string, bool) {
	key := ruleID + "|" + scheduleDateKey(nominal)
	if override, ok := overrides[key]; ok {
		if override.Action == "skip" {
			return nominal, "manually skipped", false
		}
		if actual, valid := scheduleDate(override.ActualDate); valid {
			return actual, "manually moved", true
		}
	}
	mode := adjustment.Mode
	if mode == "none" {
		return nominal, "", true
	}
	if mode == "shift-forward" || mode == "shift-backward" {
		if holidayApplies(nominal, adjustment, holidays) {
			days := adjustment.Days
			if days < 1 {
				days = 1
			}
			if mode == "shift-backward" {
				return nominal.AddDate(0, 0, -days), "holiday schedule", true
			}
			return nominal.AddDate(0, 0, days), "holiday schedule", true
		}
		return nominal, "", true
	}
	if mode == "previous-business-day" || mode == "next-business-day" {
		if !scheduleNonBusinessDay(nominal, adjustment, holidays) {
			return nominal, "", true
		}
		direction := 1
		if mode == "previous-business-day" {
			direction = -1
		}
		day := nominal
		for steps := 0; steps < 14; steps++ {
			day = day.AddDate(0, 0, direction)
			if !scheduleNonBusinessDay(day, adjustment, holidays) {
				reason := "holiday"
				if adjustment.Weekends && (nominal.Weekday() == time.Saturday || nominal.Weekday() == time.Sunday) {
					reason = "weekend"
				}
				return day, reason, true
			}
		}
	}
	return nominal, "", true
}

func scheduleNonBusinessDay(day time.Time, adjustment ScheduleAdjustment, holidays map[string]bool) bool {
	if adjustment.Weekends && (day.Weekday() == time.Saturday || day.Weekday() == time.Sunday) {
		return true
	}
	return holidayApplies(day, adjustment, holidays)
}

func holidayApplies(day time.Time, adjustment ScheduleAdjustment, holidays map[string]bool) bool {
	if len(holidays) == 0 {
		return false
	}
	if holidays[day.UTC().Format("20060102")] {
		return true
	}
	if !adjustment.WeekHoliday {
		return false
	}
	weekStart := day.AddDate(0, 0, -((int(day.Weekday()) + 6) % 7))
	for holiday := range holidays {
		value, err := time.Parse("20060102", holiday)
		if err != nil {
			continue
		}
		if !value.Before(weekStart) && !value.After(day) {
			return true
		}
	}
	return false
}

func managedScheduleEvent(ruleID, kind, label string, nominal, actual time.Time, reason string) Event {
	description := ""
	if !actual.Equal(nominal) {
		verb := "moved"
		if kind == "payday" && reason != "manually moved" {
			verb = "paid"
		}
		description = fmt.Sprintf("Normally %s · %s %s because of %s", nominal.Format("Monday, January 2"), verb, actual.Format("Monday, January 2"), reason)
	}
	meta := map[string]string{
		"X-DASHGO-MANAGED-SCHEDULE":     kind,
		"X-DASHGO-SCHEDULE-RULE-ID":     ruleID,
		"X-DASHGO-NOMINAL-DATE":         scheduleDateKey(nominal),
		"X-DASHGO-SCHEDULE-ACTUAL-DATE": scheduleDateKey(actual),
	}
	if reason != "" {
		meta["X-DASHGO-SCHEDULE-REASON"] = reason
	}
	return Event{Date: actual, Summary: label, Description: description, UID: "schedule-" + ruleID + "-" + scheduleDateKey(nominal), Meta: meta}
}

func (s *Service) HouseholdSchedulePreview(cfg HouseholdSchedules, limit int) map[string][]map[string]string {
	if limit < 1 {
		limit = 1
	}
	today := DateOnly(s.now().Year(), s.now().Month(), s.now().Day())
	end := today.AddDate(1, 2, 0)
	events := householdScheduleFeeds(cfg, today, end, s.HolidayDatesByLayer(nextHolidayLayers(cfg)))
	out := map[string][]map[string]string{}
	for _, feed := range events {
		for _, event := range feed {
			ruleID := event.Meta["X-DASHGO-SCHEDULE-RULE-ID"]
			if ruleID == "" || len(out[ruleID]) >= limit {
				continue
			}
			out[ruleID] = append(out[ruleID], map[string]string{"date": scheduleDateKey(event.Date), "nominalDate": event.Meta["X-DASHGO-NOMINAL-DATE"], "reason": event.Meta["X-DASHGO-SCHEDULE-REASON"]})
		}
	}
	return out
}
