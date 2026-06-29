package events

import (
	"sort"
	"strconv"
	"strings"
	"time"
)

type civilDateTime struct {
	year, month, day        int
	hour, minute, sec, nsec int
}

func (c civilDateTime) compare(other civilDateTime) int {
	left := [...]int{c.year, c.month, c.day, c.hour, c.minute, c.sec, c.nsec}
	right := [...]int{other.year, other.month, other.day, other.hour, other.minute, other.sec, other.nsec}
	for i := range left {
		if left[i] < right[i] {
			return -1
		}
		if left[i] > right[i] {
			return 1
		}
	}
	return 0
}

type zoneObservance struct {
	start      civilDateTime
	offsetFrom int
	offsetTo   int
	rule       map[string]string
}

type calendarZone struct {
	name        string
	loc         *time.Location
	observances []zoneObservance
	hasOffset   bool
	fixedOffset int
}

func localZone() *calendarZone { return &calendarZone{name: time.Local.String(), loc: time.Local} }
func utcZone() *calendarZone   { return &calendarZone{name: "UTC", loc: time.UTC} }

func (z *calendarZone) date(year int, month time.Month, day, hour, minute, sec, nsec int) time.Time {
	if z == nil {
		return time.Date(year, month, day, hour, minute, sec, nsec, time.Local)
	}
	if z.loc != nil {
		return time.Date(year, month, day, hour, minute, sec, nsec, z.loc)
	}
	offset := z.offsetAt(civilDateTime{year, int(month), day, hour, minute, sec, nsec})
	return time.Date(year, month, day, hour, minute, sec, nsec, time.FixedZone(z.name, offset))
}

func (z *calendarZone) offsetAt(wall civilDateTime) int {
	if z == nil || z.loc != nil {
		return 0
	}
	offset := z.fixedOffset
	if len(z.observances) > 0 {
		first := z.observances[0]
		for _, obs := range z.observances[1:] {
			if obs.start.compare(first.start) < 0 {
				first = obs
			}
		}
		offset = first.offsetFrom
	}
	best := civilDateTime{}
	haveBest := false
	for year := wall.year - 1; year <= wall.year+1; year++ {
		for _, obs := range z.observances {
			for _, transition := range observanceTransitions(obs, year) {
				if transition.compare(wall) > 0 || (haveBest && transition.compare(best) <= 0) {
					continue
				}
				best, offset, haveBest = transition, obs.offsetTo, true
			}
		}
	}
	return offset
}

func observanceTransitions(obs zoneObservance, year int) []civilDateTime {
	if len(obs.rule) == 0 {
		if year == obs.start.year {
			return []civilDateTime{obs.start}
		}
		return nil
	}
	if !strings.EqualFold(obs.rule["FREQ"], "YEARLY") || year < obs.start.year {
		return nil
	}
	interval := max(1, parseIntDefault(obs.rule["INTERVAL"], 1))
	if (year-obs.start.year)%interval != 0 {
		return nil
	}
	months := parseMonthNumbers(obs.rule["BYMONTH"], obs.start.month)
	out := []civilDateTime{}
	for _, month := range months {
		for _, day := range selectorDays(year, time.Month(month), obs.start.day, obs.rule, true) {
			out = append(out, civilDateTime{year, month, day, obs.start.hour, obs.start.minute, obs.start.sec, obs.start.nsec})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].compare(out[j]) < 0 })
	return out
}

func parseVTimezones(lines []string) map[string]*calendarZone {
	zones := map[string]*calendarZone{}
	var zone *calendarZone
	var observance *zoneObservance
	inZone := false
	for _, line := range lines {
		name, params, value, ok := parseICSLine(line)
		if !ok {
			continue
		}
		switch {
		case name == "BEGIN" && strings.EqualFold(value, "VTIMEZONE"):
			inZone, zone, observance = true, &calendarZone{}, nil
		case !inZone:
			continue
		case name == "BEGIN" && (strings.EqualFold(value, "STANDARD") || strings.EqualFold(value, "DAYLIGHT")):
			observance = &zoneObservance{}
		case name == "END" && (strings.EqualFold(value, "STANDARD") || strings.EqualFold(value, "DAYLIGHT")):
			if observance != nil {
				zone.observances = append(zone.observances, *observance)
			}
			observance = nil
		case name == "END" && strings.EqualFold(value, "VTIMEZONE"):
			if zone != nil && zone.name != "" {
				if loaded, err := time.LoadLocation(zone.name); err == nil {
					zone.loc = loaded
				}
				zones[strings.ToLower(zone.name)] = zone
			}
			inZone, zone, observance = false, nil, nil
		case observance == nil && name == "TZID":
			zone.name = strings.TrimSpace(icsUnescape(value))
		case observance != nil && name == "DTSTART":
			if dt, ok := parseICSWallDate(value); ok {
				observance.start = dt
			}
		case observance != nil && name == "TZOFFSETFROM":
			if offset, ok := parseICSOffset(value); ok {
				observance.offsetFrom, zone.fixedOffset, zone.hasOffset = offset, offset, true
			}
		case observance != nil && name == "TZOFFSETTO":
			if offset, ok := parseICSOffset(value); ok {
				observance.offsetTo, zone.fixedOffset, zone.hasOffset = offset, offset, true
			}
		case observance != nil && name == "RRULE":
			observance.rule = parseRRule(value)
		case observance != nil && name == "RDATE":
			// Normal exported VTIMEZONE data uses DTSTART/RRULE. Keep this parser
			// intentionally bounded rather than inventing a partial PERIOD model.
			_ = params
		}
	}
	return zones
}

func parseICSLine(line string) (string, map[string]string, string, bool) {
	line = strings.TrimRight(line, "\n")
	if !strings.Contains(line, ":") {
		return "", nil, "", false
	}
	parts := strings.SplitN(line, ":", 2)
	name, params := parseICSKey(parts[0])
	return name, params, parts[1], true
}

func parseICSWallDate(raw string) (civilDateTime, bool) {
	m := reICSDate.FindStringSubmatch(strings.TrimSpace(raw))
	if len(m) == 0 {
		return civilDateTime{}, false
	}
	y, _ := strconv.Atoi(m[1])
	mo, _ := strconv.Atoi(m[2])
	d, _ := strconv.Atoi(m[3])
	h, mi, sec := 0, 0, 0
	if m[4] != "" {
		h, _ = strconv.Atoi(m[4])
		mi, _ = strconv.Atoi(m[5])
		sec, _ = strconv.Atoi(m[6])
	}
	return civilDateTime{y, mo, d, h, mi, sec, 0}, true
}

func parseICSOffset(raw string) (int, bool) {
	raw = strings.TrimSpace(raw)
	if len(raw) != 5 && len(raw) != 6 {
		return 0, false
	}
	sign := 1
	if raw[0] == '-' {
		sign = -1
	} else if raw[0] != '+' {
		return 0, false
	}
	digits := strings.ReplaceAll(raw[1:], ":", "")
	if len(digits) != 4 {
		return 0, false
	}
	h, errH := strconv.Atoi(digits[:2])
	m, errM := strconv.Atoi(digits[2:])
	if errH != nil || errM != nil || h > 23 || m > 59 {
		return 0, false
	}
	return sign * (h*3600 + m*60), true
}

func resolveCalendarZone(tzid string, zones map[string]*calendarZone, fallback *calendarZone) *calendarZone {
	tzid = strings.TrimSpace(tzid)
	if tzid == "" {
		if fallback != nil {
			return fallback
		}
		return localZone()
	}
	if loaded, err := time.LoadLocation(tzid); err == nil {
		return &calendarZone{name: tzid, loc: loaded}
	}
	if zone := zones[strings.ToLower(tzid)]; zone != nil {
		return zone
	}
	if fallback != nil {
		return fallback
	}
	return localZone()
}

func parseICSDateInZone(raw string, params map[string]string, zones map[string]*calendarZone, fallback *calendarZone) (time.Time, bool, *calendarZone, bool) {
	m := reICSDate.FindStringSubmatch(strings.TrimSpace(raw))
	if len(m) == 0 {
		return time.Time{}, false, nil, false
	}
	y, _ := strconv.Atoi(m[1])
	mo, _ := strconv.Atoi(m[2])
	d, _ := strconv.Atoi(m[3])
	zone := resolveCalendarZone(params["TZID"], zones, fallback)
	if m[4] == "" || strings.EqualFold(params["VALUE"], "DATE") {
		return zone.date(y, time.Month(mo), d, 0, 0, 0, 0), true, zone, true
	}
	h, _ := strconv.Atoi(m[4])
	mi, _ := strconv.Atoi(m[5])
	sec, _ := strconv.Atoi(m[6])
	if m[7] == "Z" {
		return time.Date(y, time.Month(mo), d, h, mi, sec, 0, time.UTC), false, utcZone(), true
	}
	return zone.date(y, time.Month(mo), d, h, mi, sec, 0), false, zone, true
}
