package events

import (
	"regexp"
	"slices"
	"strings"
	"time"
)

var reSignedInteger = regexp.MustCompile(`^[+-]?\d+`)

func expand(ev ICSEvent, winStart, winEnd time.Time) []ICSEvent {
	base := []ICSEvent{}
	if ev.RRule == "" {
		if !skipped(ev, ev.Start) {
			base = append(base, makeInstance(ev, ev.Start, 0))
		}
	} else {
		r := parseRRule(ev.RRule)
		interval := max(1, parseIntDefault(r["INTERVAL"], 1))
		count := parseIntDefault(r["COUNT"], 0)
		until := recurrenceUntil(ev, r, winEnd)
		switch r["FREQ"] {
		case "DAILY":
			base = expandDaily(ev, winStart, winEnd, interval, count, until)
		case "WEEKLY":
			base = expandWeekly(ev, winStart, winEnd, r, interval, count, until)
		case "MONTHLY":
			base = expandMonthly(ev, winStart, winEnd, r, interval, count, until)
		case "YEARLY":
			base = expandYearly(ev, winStart, winEnd, r, interval, count, until)
		default:
			if !skipped(ev, ev.Start) {
				base = append(base, makeInstance(ev, ev.Start, 0))
			}
		}
	}
	return mergeRdates(ev, base, winStart, winEnd)
}

func recurrenceUntil(ev ICSEvent, r map[string]string, fallback time.Time) time.Time {
	until := fallback
	raw := r["UNTIL"]
	if raw == "" {
		return until
	}
	dt, allDay, _, ok := parseICSDateInZone(raw, nil, nil, ev.zone)
	if !ok {
		return until
	}
	if allDay {
		dt = eventDate(ev, dt.Year(), dt.Month(), dt.Day(), 23, 59, 59, 999999999)
	}
	if dt.Before(until) {
		return dt
	}
	return until
}

func makeInstance(ev ICSEvent, start time.Time, seq int) ICSEvent {
	inst := ev
	inst.Start, inst.End = start, eventEnd(ev, start)
	inst.Recur = ev.RRule != "" || len(ev.Rdates) > 0
	inst.Seq = seq
	return inst
}

func eventDate(ev ICSEvent, year int, month time.Month, day, hour, minute, second, nsec int) time.Time {
	if ev.zone != nil {
		return ev.zone.date(year, month, day, hour, minute, second, nsec)
	}
	return time.Date(year, month, day, hour, minute, second, nsec, ev.Start.Location())
}

func eventAddDays(ev ICSEvent, start time.Time, days int) time.Time {
	civil := time.Date(start.Year(), start.Month(), start.Day(), start.Hour(), start.Minute(), start.Second(), start.Nanosecond(), time.UTC).AddDate(0, 0, days)
	return eventDate(ev, civil.Year(), civil.Month(), civil.Day(), civil.Hour(), civil.Minute(), civil.Second(), civil.Nanosecond())
}

func eventEnd(ev ICSEvent, instStart time.Time) *time.Time {
	if ev.End == nil {
		return nil
	}
	if ev.AllDay {
		days := calendarDayDiff(ev.Start, *ev.End)
		if days < 0 {
			days = 0
		}
		end := eventDate(ev, instStart.Year(), instStart.Month(), instStart.Day(), 0, 0, 0, 0)
		end = eventAddDays(ev, end, days)
		return &end
	}
	duration := ev.End.Sub(ev.Start)
	end := instStart.Add(duration)
	return &end
}

func eventInWindow(ev ICSEvent, start, end time.Time) bool {
	finish := ev.Start
	if ev.End != nil {
		finish = *ev.End
	}
	return !finish.Before(start) && !ev.Start.After(end)
}

func recurrenceDayKey(t time.Time) string { return t.Format("20060102") }

func skipped(ev ICSEvent, start time.Time) bool {
	if ev.Skip != nil && ev.Skip[epochMs(start)] {
		return true
	}
	return ev.SkipDays != nil && ev.SkipDays[recurrenceDayKey(start)]
}

func pushInstance(res []ICSEvent, ev ICSEvent, start, winStart, winEnd time.Time, seq int) []ICSEvent {
	inst := makeInstance(ev, start, seq)
	if eventInWindow(inst, winStart, winEnd) && !skipped(ev, start) {
		return append(res, inst)
	}
	return res
}

func mergeRdates(ev ICSEvent, base []ICSEvent, winStart, winEnd time.Time) []ICSEvent {
	out := append([]ICSEvent(nil), base...)
	for index, value := range ev.Rdates {
		if skipped(ev, value.Start) {
			continue
		}
		inst := makeInstance(ev, value.Start, len(base)+index)
		if eventInWindow(inst, winStart, winEnd) {
			out = append(out, inst)
		}
	}
	if len(out) < 2 {
		return out
	}
	slices.SortFunc(out, func(left, right ICSEvent) int {
		if left.Start.Before(right.Start) {
			return -1
		}
		if left.Start.After(right.Start) {
			return 1
		}
		return 0
	})
	unique := out[:0]
	for _, inst := range out {
		if len(unique) == 0 || !inst.Start.Equal(unique[len(unique)-1].Start) {
			unique = append(unique, inst)
		}
	}
	return unique
}

func expandDaily(ev ICSEvent, winStart, winEnd time.Time, interval, count int, until time.Time) []ICSEvent {
	start, occurrence := ev.Start, 0
	if winStart.After(start) {
		elapsed := max(0, calendarDayDiff(start, winStart))
		occurrence = max(0, elapsed/interval-1)
	}
	if count > 0 && occurrence >= count {
		return nil
	}
	res := []ICSEvent{}
	for range maxRecurrenceSteps {
		if count > 0 && occurrence >= count {
			break
		}
		d := eventAddDays(ev, start, occurrence*interval)
		if d.After(until) || d.After(winEnd) {
			break
		}
		res = pushInstance(res, ev, d, winStart, winEnd, occurrence)
		occurrence++
	}
	return res
}

func weeklyDayOffsets(raw string, fallbackWeekday, weekStart int) []int {
	ical := map[string]int{"SU": 0, "MO": 1, "TU": 2, "WE": 3, "TH": 4, "FR": 5, "SA": 6}
	offsets := []int{}
	for _, part := range strings.Split(raw, ",") {
		day := reSignedInteger.ReplaceAllString(strings.TrimSpace(part), "")
		if weekday, ok := ical[day]; ok {
			offsets = append(offsets, (weekday-weekStart+7)%7)
		}
	}
	if len(offsets) == 0 {
		offsets = []int{(fallbackWeekday - weekStart + 7) % 7}
	}
	slices.Sort(offsets)
	return uniqueInts(offsets)
}

func weeklyCandidate(ev ICSEvent, week time.Time, offset int) time.Time {
	day := eventAddDays(ev, week, offset)
	return eventDate(ev, day.Year(), day.Month(), day.Day(), ev.Start.Hour(), ev.Start.Minute(), ev.Start.Second(), ev.Start.Nanosecond())
}

func weeklyOccurrencesBefore(ev ICSEvent, start, firstWeek time.Time, offsets []int, cycles int) int {
	if cycles <= 0 {
		return 0
	}
	first := 0
	for _, offset := range offsets {
		if !weeklyCandidate(ev, firstWeek, offset).Before(start) {
			first++
		}
	}
	return first + (cycles-1)*len(offsets)
}

func recurrenceWeekStart(ev ICSEvent, start time.Time, weekStart int) time.Time {
	delta := (int(start.Weekday()) - weekStart + 7) % 7
	midnight := eventDate(ev, start.Year(), start.Month(), start.Day(), 0, 0, 0, 0)
	return eventAddDays(ev, midnight, -delta)
}

func expandWeekly(ev ICSEvent, winStart, winEnd time.Time, r map[string]string, interval, count int, until time.Time) []ICSEvent {
	start := ev.Start
	ical := map[string]int{"SU": 0, "MO": 1, "TU": 2, "WE": 3, "TH": 4, "FR": 5, "SA": 6}
	weekStart, ok := ical[strings.ToUpper(strings.TrimSpace(r["WKST"]))]
	if !ok {
		weekStart = 1
	} // RFC 5545 default: Monday.
	offsets := weeklyDayOffsets(r["BYDAY"], int(start.Weekday()), weekStart)
	firstWeek := recurrenceWeekStart(ev, start, weekStart)
	cycles := 0
	if winStart.After(firstWeek) {
		elapsed := max(0, calendarDayDiff(firstWeek, winStart)/7)
		cycles = max(0, elapsed/interval-1)
	}
	weeks := cycles * interval
	emitted := weeklyOccurrencesBefore(ev, start, firstWeek, offsets, cycles)
	if count > 0 && emitted >= count {
		return nil
	}
	res := []ICSEvent{}
	for range maxRecurrenceSteps {
		week := eventAddDays(ev, firstWeek, weeks*7)
		if week.After(until) || week.After(winEnd) {
			break
		}
		for _, offset := range offsets {
			d := weeklyCandidate(ev, week, offset)
			if d.Before(start) {
				continue
			}
			if count > 0 && emitted >= count || d.After(until) || d.After(winEnd) {
				return res
			}
			res = pushInstance(res, ev, d, winStart, winEnd, emitted)
			emitted++
		}
		weeks += interval
	}
	return res
}
