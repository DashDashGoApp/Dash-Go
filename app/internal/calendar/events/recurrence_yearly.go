package events

import "time"

func expandYearly(ev ICSEvent, winStart, winEnd time.Time, r map[string]string, interval, count int, until time.Time) []ICSEvent {
	start, yearIndex, emitted := ev.Start, 0, 0
	if count == 0 && winStart.After(start) {
		elapsed := max(0, winStart.Year()-start.Year())
		yearIndex = max(0, elapsed/interval-1)
	}
	res := []ICSEvent{}
	for range maxRecurrenceSteps {
		year := start.Year() + yearIndex*interval
		if eventDate(ev, year, time.January, 1, 0, 0, 0, 0).After(until) || year > winEnd.Year()+1 {
			break
		}
		for _, candidate := range yearlyCandidates(ev, year, r) {
			if candidate.Before(start) {
				continue
			}
			if candidate.After(until) || candidate.After(winEnd) {
				return res
			}
			if count > 0 && emitted >= count {
				return res
			}
			res = pushInstance(res, ev, candidate, winStart, winEnd, emitted)
			emitted++
		}
		if count > 0 && emitted >= count {
			break
		}
		yearIndex++
	}
	return res
}

func yearlyCandidates(ev ICSEvent, year int, r map[string]string) []time.Time {
	start := ev.Start
	months := parseMonthNumbers(r["BYMONTH"], int(start.Month()))
	strict := r["BYMONTH"] != "" || r["BYMONTHDAY"] != "" || r["BYDAY"] != ""
	out := []time.Time{}
	for _, month := range months {
		for _, day := range selectorDays(year, time.Month(month), start.Day(), r, strict) {
			out = append(out, eventDate(ev, year, time.Month(month), day, start.Hour(), start.Minute(), start.Second(), start.Nanosecond()))
		}
	}
	return out
}
