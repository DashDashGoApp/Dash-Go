package events

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

var reICSDate = regexp.MustCompile(`^(\d{4})(\d{2})(\d{2})(?:T(\d{2})(\d{2})(\d{2})(Z)?)?$`)

type icsProperty struct {
	name   string
	params map[string]string
	value  string
}

func parseICS(text string, cal CalendarSource) []ICSEvent {
	lines := unfoldICS(text)
	zones := parseVTimezones(lines)
	events := []ICSEvent{}
	var props []icsProperty
	inEvent := false
	for _, line := range lines {
		name, params, value, ok := parseICSLine(line)
		if !ok {
			continue
		}
		switch {
		case name == "BEGIN" && strings.EqualFold(value, "VEVENT"):
			inEvent, props = true, nil
		case name == "END" && strings.EqualFold(value, "VEVENT"):
			if inEvent {
				if event, ok := parseICSEvent(props, cal, zones); ok {
					events = append(events, event)
				}
			}
			inEvent, props = false, nil
		case inEvent:
			props = append(props, icsProperty{name: name, params: params, value: value})
		}
	}
	markMidnightAllDay(events)
	markRecurrenceOverrides(events)
	return events
}

func parseICSEvent(props []icsProperty, cal CalendarSource, zones map[string]*calendarZone) (ICSEvent, bool) {
	ev := ICSEvent{Cal: cal}
	for _, prop := range props {
		if prop.name != "DTSTART" {
			continue
		}
		if dt, allDay, zone, ok := parseICSDateInZone(prop.value, prop.params, zones, nil); ok {
			ev.Start, ev.AllDay, ev.zone = dt, allDay, zone
		}
		break
	}
	if ev.Start.IsZero() {
		return ICSEvent{}, false
	}
	for _, prop := range props {
		switch prop.name {
		case "DTSTART":
			// Parsed first so properties may appear in any legal order.
		case "DTEND":
			if dt, _, _, ok := parseICSDateInZone(prop.value, prop.params, zones, ev.zone); ok {
				ev.End = &dt
			}
		case "SUMMARY":
			ev.Title = icsUnescape(prop.value)
		case "DESCRIPTION":
			ev.Desc = icsUnescape(prop.value)
		case "LOCATION":
			ev.Location = icsUnescape(prop.value)
		case "RRULE":
			ev.RRule = strings.TrimSpace(prop.value)
		case "UID":
			ev.UID = prop.value
		case "X-DASHGO-APP-OWNER":
			ev.AppOwner = strings.TrimSpace(icsUnescape(prop.value))
		case "EXDATE":
			parseExdates(&ev, prop, zones)
		case "RDATE":
			parseRdates(&ev, prop, zones)
		case "RECURRENCE-ID":
			if dt, dateOnly, _, ok := parseICSDateInZone(prop.value, prop.params, zones, ev.zone); ok {
				ms := epochMs(dt)
				ev.RecurID, ev.RecurIDDateOnly = &ms, dateOnly
				if dateOnly {
					ev.RecurIDDay = recurrenceDayKey(dt)
				}
			}
		}
	}
	return ev, true
}

func parseExdates(ev *ICSEvent, prop icsProperty, zones map[string]*calendarZone) {
	for raw := range strings.SplitSeq(prop.value, ",") {
		dt, dateOnly, _, ok := parseICSDateInZone(raw, prop.params, zones, ev.zone)
		if !ok {
			continue
		}
		if ev.Exdates == nil {
			ev.Exdates = map[int64]bool{}
		}
		ev.Exdates[epochMs(dt)] = true
		if dateOnly {
			if ev.ExdateDays == nil {
				ev.ExdateDays = map[string]bool{}
			}
			ev.ExdateDays[recurrenceDayKey(dt)] = true
		}
	}
}

func parseRdates(ev *ICSEvent, prop icsProperty, zones map[string]*calendarZone) {
	for raw := range strings.SplitSeq(prop.value, ",") {
		dt, dateOnly, _, ok := parseICSDateInZone(raw, prop.params, zones, ev.zone)
		if !ok {
			continue
		}
		if dateOnly && !ev.AllDay {
			dt = ev.zone.date(dt.Year(), dt.Month(), dt.Day(), ev.Start.Hour(), ev.Start.Minute(), ev.Start.Second(), ev.Start.Nanosecond())
		}
		ev.Rdates = append(ev.Rdates, rdate{Start: dt, DateOnly: dateOnly})
	}
}

func unfoldICS(text string) []string {
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	raw := strings.Split(text, "\n")
	out := []string{}
	for _, line := range raw {
		if (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")) && len(out) > 0 {
			out[len(out)-1] += strings.TrimLeft(line, " \t")
		} else {
			out = append(out, line)
		}
	}
	return out
}

func parseICSKey(key string) (string, map[string]string) {
	bits := strings.Split(key, ";")
	name := strings.ToUpper(bits[0])
	params := map[string]string{}
	for _, bit := range bits[1:] {
		if !strings.Contains(bit, "=") {
			continue
		}
		pair := strings.SplitN(bit, "=", 2)
		params[strings.ToUpper(pair[0])] = strings.Trim(pair[1], `"`)
	}
	return name, params
}

func icsUnescape(v string) string {
	v = strings.ReplaceAll(v, `\n`, "\n")
	v = strings.ReplaceAll(v, `\N`, "\n")
	v = strings.ReplaceAll(v, `\,`, ",")
	v = strings.ReplaceAll(v, `\;`, ";")
	return strings.ReplaceAll(v, `\\`, `\`)
}

// parseICSDate keeps the exported compatibility helper. Full ICS parsing uses
// parseICSDateInZone so recurring source events retain their source civil zone.
func parseICSDate(raw string, params map[string]string) (time.Time, bool, bool) {
	dt, allDay, _, ok := parseICSDateInZone(raw, params, nil, nil)
	return dt, allDay, ok
}

func markMidnightAllDay(events []ICSEvent) {
	for i := range events {
		if events[i].End == nil || events[i].AllDay || events[i].Start.Hour() != 0 || events[i].Start.Minute() != 0 || events[i].Start.Second() != 0 || !events[i].End.After(events[i].Start) || events[i].End.Hour() != 0 || events[i].End.Minute() != 0 || events[i].End.Second() != 0 {
			continue
		}
		events[i].AllDay = true
	}
}

func markRecurrenceOverrides(events []ICSEvent) {
	overrides := map[string][]int64{}
	overrideDays := map[string][]string{}
	for _, ev := range events {
		if ev.RecurID == nil || ev.UID == "" {
			continue
		}
		overrides[ev.UID] = append(overrides[ev.UID], *ev.RecurID)
		if ev.RecurIDDateOnly && ev.RecurIDDay != "" {
			overrideDays[ev.UID] = append(overrideDays[ev.UID], ev.RecurIDDay)
		}
	}
	for i := range events {
		if events[i].RRule == "" && len(events[i].Rdates) == 0 {
			continue
		}
		skip := map[int64]bool{}
		for key := range events[i].Exdates {
			skip[key] = true
		}
		for _, ms := range overrides[events[i].UID] {
			skip[ms] = true
		}
		skipDays := map[string]bool{}
		for day := range events[i].ExdateDays {
			skipDays[day] = true
		}
		for _, day := range overrideDays[events[i].UID] {
			skipDays[day] = true
		}
		if len(skip) > 0 {
			events[i].Skip = skip
		}
		if len(skipDays) > 0 {
			events[i].SkipDays = skipDays
		}
	}
}

func parseRRule(rrule string) map[string]string {
	out := map[string]string{}
	for part := range strings.SplitSeq(rrule, ";") {
		if !strings.Contains(part, "=") {
			continue
		}
		pair := strings.SplitN(part, "=", 2)
		out[strings.ToUpper(strings.TrimSpace(pair[0]))] = strings.ToUpper(strings.TrimSpace(pair[1]))
	}
	return out
}

func parseIntDefault(s string, def int) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return def
	}
	return n
}
