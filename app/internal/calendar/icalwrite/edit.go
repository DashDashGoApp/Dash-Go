package icalwrite

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// unfold accepts CRLF or LF and returns logical iCalendar lines.
func unfold(src string) []string {
	raw := strings.Split(strings.ReplaceAll(strings.ReplaceAll(src, "\r\n", "\n"), "\r", "\n"), "\n")
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		if line == "" {
			continue
		}
		if (line[0] == ' ' || line[0] == '\t') && len(out) > 0 {
			out[len(out)-1] += line[1:]
			continue
		}
		out = append(out, line)
	}
	return out
}

func propName(line string) string {
	end := len(line)
	if i := strings.IndexAny(line, ";:"); i >= 0 {
		end = i
	}
	return strings.ToUpper(line[:end])
}
func propValue(line string) string {
	if i := strings.Index(line, ":"); i >= 0 {
		return line[i+1:]
	}
	return ""
}

// directVEVENTIndices identifies only properties directly inside the first
// VEVENT. Nested VALARM/VTODO fields must never be touched by a kiosk edit.
func directVEVENTIndices(lines []string) (map[int]bool, bool) {
	depth, eventDepth := 0, 0
	direct := map[int]bool{}
	found := false
	for i, line := range lines {
		name, value := propName(line), strings.ToUpper(strings.TrimSpace(propValue(line)))
		switch {
		case name == "BEGIN":
			depth++
			if value == "VEVENT" && !found {
				found, eventDepth = true, depth
			}
		case name == "END":
			if found && depth == eventDepth && value == "VEVENT" {
				return direct, true
			}
			if depth > 0 {
				depth--
			}
		case found && depth == eventDepth:
			direct[i] = true
		}
	}
	return direct, false
}

func directLines(src string) ([]string, map[int]bool, error) {
	lines := unfold(src)
	count := 0
	for _, line := range lines {
		if propName(line) == "BEGIN" && strings.EqualFold(strings.TrimSpace(propValue(line)), "VEVENT") {
			count++
		}
	}
	if count != 1 {
		return nil, nil, errors.New("single VEVENT document required")
	}
	direct, ok := directVEVENTIndices(lines)
	if !ok {
		return nil, nil, errors.New("single VEVENT document required")
	}
	return lines, direct, nil
}

func HasSchedulingProperties(src string) bool {
	lines, direct, err := directLines(src)
	if err != nil {
		return false
	}
	for i := range direct {
		switch propName(lines[i]) {
		case "ORGANIZER", "ATTENDEE":
			return true
		}
	}
	return false
}
func IsRecurring(src string) bool {
	lines, direct, err := directLines(src)
	if err != nil {
		return false
	}
	for i := range direct {
		switch propName(lines[i]) {
		case "RRULE", "RDATE":
			return true
		}
	}
	return false
}
func HasRecurrenceID(src string) bool {
	lines, direct, err := directLines(src)
	if err != nil {
		return false
	}
	for i := range direct {
		if propName(lines[i]) == "RECURRENCE-ID" {
			return true
		}
	}
	return false
}
func UID(src string) string {
	lines, direct, err := directLines(src)
	if err != nil {
		return ""
	}
	for i := range direct {
		if propName(lines[i]) == "UID" {
			return strings.TrimSpace(propValue(lines[i]))
		}
	}
	return ""
}

func bumpSequence(lines []string, direct map[int]bool) []string {
	for i := range direct {
		if propName(lines[i]) != "SEQUENCE" {
			continue
		}
		n, _ := strconv.Atoi(strings.TrimSpace(propValue(lines[i])))
		lines[i] = "SEQUENCE:" + strconv.Itoa(n+1)
		return lines
	}
	for i := range direct {
		if propName(lines[i]) == "UID" {
			out := append([]string{}, lines[:i+1]...)
			out = append(out, "SEQUENCE:1")
			return append(out, lines[i+1:]...)
		}
	}
	return lines
}

func replaceDirect(lines []string, name, rendered string) []string {
	direct, ok := directVEVENTIndices(lines)
	if !ok {
		return lines
	}
	out := make([]string, 0, len(lines)+1)
	replaced := false
	for i, line := range lines {
		if direct[i] && propName(line) == name {
			if !replaced && rendered != "" {
				out = append(out, rendered)
			}
			replaced = true
			continue
		}
		out = append(out, line)
	}
	if replaced || rendered == "" {
		return out
	}
	for i := len(out) - 1; i >= 0; i-- {
		if strings.EqualFold(out[i], "END:VEVENT") {
			return append(out[:i], append([]string{rendered}, out[i:]...)...)
		}
	}
	return out
}

func ApplyEdit(src string, e Event, now time.Time) (string, error) {
	if err := e.Validate(); err != nil {
		return "", err
	}
	if HasSchedulingProperties(src) {
		return "", errors.New("events with attendees are read-only on this dashboard")
	}
	if IsRecurring(src) || HasRecurrenceID(src) {
		return "", errors.New("recurring events can only have single dates removed on this dashboard")
	}
	if got := UID(src); got != e.UID {
		return "", fmt.Errorf("uid mismatch: file has %q", got)
	}
	lines, direct, err := directLines(src)
	if err != nil {
		return "", err
	}
	stamp := utcStamp(now)
	lines = replaceDirect(lines, "DTSTAMP", prop("DTSTAMP", stamp))
	lines = replaceDirect(lines, "LAST-MODIFIED", prop("LAST-MODIFIED", stamp))
	if e.AllDay {
		lines = replaceDirect(lines, "DTSTART", prop("DTSTART;VALUE=DATE", dateStamp(e.Start)))
		lines = replaceDirect(lines, "DTEND", prop("DTEND;VALUE=DATE", dateStamp(e.End)))
	} else {
		lines = replaceDirect(lines, "DTSTART", prop("DTSTART", utcStamp(e.Start)))
		lines = replaceDirect(lines, "DTEND", prop("DTEND", utcStamp(e.End)))
	}
	lines = replaceDirect(lines, "DURATION", "")
	lines = replaceDirect(lines, "SUMMARY", prop("SUMMARY", escapeText(strings.TrimSpace(e.Title))))
	lines = replaceDirect(lines, "DESCRIPTION", textOrEmpty("DESCRIPTION", e.Desc))
	lines = replaceDirect(lines, "LOCATION", textOrEmpty("LOCATION", e.Location))
	direct, _ = directVEVENTIndices(lines)
	lines = bumpSequence(lines, direct)
	return refold(lines), nil
}
func textOrEmpty(name, value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return prop(name, escapeText(value))
}

type startForm struct {
	allDay, utc, floating bool
	tzid                  string
	hour, minute, second  int
}

func masterStartForm(lines []string, direct map[int]bool) (startForm, error) {
	for i := range direct {
		line := lines[i]
		if propName(line) != "DTSTART" {
			continue
		}
		key, value := line, propValue(line)
		if split := strings.Index(key, ":"); split >= 0 {
			key = key[:split]
		}
		form := startForm{allDay: strings.Contains(strings.ToUpper(key), "VALUE=DATE") || len(value) == 8}
		if form.allDay {
			return form, nil
		}
		form.utc = strings.HasSuffix(value, "Z")
		form.floating = !form.utc && !strings.Contains(strings.ToUpper(key), "TZID=")
		upper := strings.ToUpper(key)
		if p := strings.Index(upper, "TZID="); p >= 0 {
			after := key[p+5:]
			if end := strings.Index(after, ";"); end >= 0 {
				after = after[:end]
			}
			form.tzid = strings.Trim(after, "\"")
		}
		raw := strings.TrimSuffix(value, "Z")
		if len(raw) >= 15 {
			form.hour, _ = strconv.Atoi(raw[9:11])
			form.minute, _ = strconv.Atoi(raw[11:13])
			form.second, _ = strconv.Atoi(raw[13:15])
		}
		return form, nil
	}
	return startForm{}, errors.New("recurring event DTSTART not found")
}
func recurrenceStamp(form startForm, occurrence time.Time) string {
	if form.allDay {
		return dateStamp(occurrence)
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
	civil := occurrence.In(loc)
	value := time.Date(civil.Year(), civil.Month(), civil.Day(), form.hour, form.minute, form.second, 0, loc).Format("20060102T150405")
	if form.utc {
		return value + "Z"
	}
	return value
}

// AppendExdate removes exactly one occurrence while retaining DTSTART's value
// form: DATE, UTC, TZID, or floating time. Detached instances are refused.
func AppendExdate(src string, occurrence time.Time, allDayIgnored bool, now time.Time) (string, error) {
	if !IsRecurring(src) {
		return "", errors.New("event is not recurring")
	}
	if HasSchedulingProperties(src) {
		return "", errors.New("events with attendees are read-only on this dashboard")
	}
	if HasRecurrenceID(src) {
		return "", errors.New("detached recurring instances are read-only on this dashboard")
	}
	lines, direct, err := directLines(src)
	if err != nil {
		return "", err
	}
	form, err := masterStartForm(lines, direct)
	if err != nil {
		return "", err
	}
	exName := "EXDATE"
	if form.allDay {
		exName += ";VALUE=DATE"
	} else if form.tzid != "" {
		exName += ";TZID=" + form.tzid
	}
	ex := exName + ":" + recurrenceStamp(form, occurrence)
	for i := range direct {
		if propName(lines[i]) == "EXDATE" && strings.Contains(propValue(lines[i]), recurrenceStamp(form, occurrence)) {
			return "", errors.New("occurrence already removed")
		}
	}
	inserted := false
	out := make([]string, 0, len(lines)+1)
	for i, line := range lines {
		out = append(out, line)
		if direct[i] && !inserted && (propName(line) == "RRULE" || propName(line) == "RDATE") {
			out = append(out, ex)
			inserted = true
		}
	}
	if !inserted {
		return "", errors.New("recurrence rule not found")
	}
	out = replaceDirect(out, "DTSTAMP", prop("DTSTAMP", utcStamp(now)))
	out = replaceDirect(out, "LAST-MODIFIED", prop("LAST-MODIFIED", utcStamp(now)))
	direct, _ = directVEVENTIndices(out)
	out = bumpSequence(out, direct)
	return refold(out), nil
}
