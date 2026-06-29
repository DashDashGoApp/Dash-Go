package events

import (
	"time"
)

func expandMonthly(ev ICSEvent, winStart, winEnd time.Time, r map[string]string, interval, count int, until time.Time) []ICSEvent {
	start, monthIndex, emitted := ev.Start, 0, 0
	if count == 0 && winStart.After(start) {
		elapsed := max(0, monthDiff(start, winStart))
		monthIndex = max(0, elapsed/interval-1)
	}
	res := []ICSEvent{}
	for range maxRecurrenceSteps {
		base := time.Date(start.Year(), start.Month(), 1, start.Hour(), start.Minute(), start.Second(), start.Nanosecond(), time.UTC).AddDate(0, monthIndex*interval, 0)
		monthStart := eventDate(ev, base.Year(), base.Month(), 1, start.Hour(), start.Minute(), start.Second(), start.Nanosecond())
		if monthStart.After(until) || monthStart.After(winEnd) {
			break
		}
		for _, day := range selectorDays(base.Year(), base.Month(), start.Day(), r, false) {
			d := eventDate(ev, base.Year(), base.Month(), day, start.Hour(), start.Minute(), start.Second(), start.Nanosecond())
			if d.Before(start) {
				continue
			}
			if d.After(until) || d.After(winEnd) || (count > 0 && emitted >= count) {
				return res
			}
			res = pushInstance(res, ev, d, winStart, winEnd, emitted)
			emitted++
		}
		if count > 0 && emitted >= count {
			break
		}
		monthIndex++
	}
	return res
}
