package icalwrite

import (
	"strings"
	"time"
)

// remapExdates rewrites each direct EXDATE property so its values keep their
// civil dates while adopting the new DTSTART's wall time, zone, and value
// form. A value that cannot be parsed is preserved unchanged rather than lost.
func remapExdates(component []string, oldForm, newForm startForm) ([]string, error) {
	lines, direct, err := directLines(refold(component))
	if err != nil {
		return nil, err
	}
	exName := "EXDATE"
	if newForm.allDay {
		exName += ";VALUE=DATE"
	} else if newForm.tzid != "" {
		exName += ";TZID=" + newForm.tzid
	}
	out := make([]string, 0, len(lines))
	for i, line := range lines {
		if !direct[i] || propName(line) != "EXDATE" {
			out = append(out, line)
			continue
		}
		values := []string{}
		changed := true
		for _, raw := range strings.Split(propValue(line), ",") {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			day, ok := exdateCivilDay(raw, oldForm)
			if !ok {
				// Unknown encoding: keep this whole property untouched.
				changed = false
				break
			}
			values = append(values, renderExdateValue(day, newForm))
		}
		if !changed || len(values) == 0 {
			out = append(out, line)
			continue
		}
		out = append(out, exName+":"+strings.Join(values, ","))
	}
	return out, nil
}

// exdateCivilDay reads one EXDATE value in the series' previous form and
// returns its civil calendar day in that form's location.
func exdateCivilDay(value string, form startForm) (time.Time, bool) {
	if len(value) == 8 {
		day, err := time.Parse("20060102", value)
		if err != nil {
			return time.Time{}, false
		}
		return day, true
	}
	loc := time.UTC
	if form.tzid != "" {
		if loaded, err := time.LoadLocation(form.tzid); err == nil {
			loc = loaded
		}
	}
	if form.floating {
		loc = time.Local
	}
	if strings.HasSuffix(value, "Z") {
		parsed, err := time.Parse("20060102T150405Z", value)
		if err != nil {
			return time.Time{}, false
		}
		civil := parsed.In(loc)
		return time.Date(civil.Year(), civil.Month(), civil.Day(), 0, 0, 0, 0, time.UTC), true
	}
	parsed, err := time.ParseInLocation("20060102T150405", value, loc)
	if err != nil {
		return time.Time{}, false
	}
	return time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, time.UTC), true
}

func renderExdateValue(day time.Time, form startForm) string {
	if form.allDay {
		return day.Format("20060102")
	}
	loc := time.UTC
	if form.tzid != "" {
		if loaded, err := time.LoadLocation(form.tzid); err == nil {
			loc = loaded
		}
	}
	if form.floating {
		loc = time.Local
	}
	value := time.Date(day.Year(), day.Month(), day.Day(), form.hour, form.minute, form.second, 0, loc)
	if form.utc {
		return value.UTC().Format("20060102T150405") + "Z"
	}
	return value.Format("20060102T150405")
}
