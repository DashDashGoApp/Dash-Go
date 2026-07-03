package icalwrite

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

func componentStartForm(component []string) (startForm, error) {
	lines, direct, err := directLines(refold(component))
	if err != nil {
		return startForm{}, err
	}
	return masterStartForm(lines, direct)
}

func componentScheduling(component []string) bool {
	lines, direct, err := directLines(refold(component))
	if err != nil {
		return true
	}
	for index := range direct {
		switch propName(lines[index]) {
		case "ORGANIZER", "ATTENDEE":
			return true
		}
	}
	return false
}

func renderTimeForForm(t time.Time, form startForm) string {
	if form.utc {
		return utcStamp(t)
	}
	loc := time.Local
	if form.tzid != "" {
		if loaded, err := time.LoadLocation(form.tzid); err == nil {
			loc = loaded
		}
	}
	return t.In(loc).Format("20060102T150405")
}

func eventTimeLines(e Event, original startForm) (string, string) {
	if e.AllDay {
		return prop("DTSTART;VALUE=DATE", dateStamp(e.Start)), prop("DTEND;VALUE=DATE", dateStamp(e.End))
	}
	form := original
	if form.allDay {
		form = startForm{utc: true}
	}
	name := "DTSTART"
	endName := "DTEND"
	if form.tzid != "" {
		name += ";TZID=" + form.tzid
		endName += ";TZID=" + form.tzid
	}
	return prop(name, renderTimeForForm(e.Start, form)), prop(endName, renderTimeForForm(e.End, form))
}

func editEventComponent(component []string, event Event, now time.Time, preserveStartForm bool) ([]string, error) {
	lines, direct, err := directLines(refold(component))
	if err != nil {
		return nil, err
	}
	form := startForm{utc: true}
	if preserveStartForm {
		if form, err = masterStartForm(lines, direct); err != nil {
			return nil, err
		}
	}
	startLine, endLine := eventTimeLines(event, form)
	stamp := utcStamp(now)
	lines = replaceDirect(lines, "DTSTAMP", prop("DTSTAMP", stamp))
	lines = replaceDirect(lines, "LAST-MODIFIED", prop("LAST-MODIFIED", stamp))
	lines = replaceDirect(lines, "DTSTART", startLine)
	lines = replaceDirect(lines, "DTEND", endLine)
	lines = replaceDirect(lines, "DURATION", "")
	lines = replaceDirect(lines, "SUMMARY", prop("SUMMARY", escapeText(strings.TrimSpace(event.Title))))
	lines = replaceDirect(lines, "DESCRIPTION", textOrEmpty("DESCRIPTION", event.Desc))
	lines = replaceDirect(lines, "LOCATION", textOrEmpty("LOCATION", event.Location))
	direct, _ = directVEVENTIndices(lines)
	lines = bumpSequence(lines, direct)
	return lines, nil
}

func recurrenceIDLine(form startForm, occurrence time.Time) string {
	name := "RECURRENCE-ID"
	if form.allDay {
		name += ";VALUE=DATE"
	} else if form.tzid != "" {
		name += ";TZID=" + form.tzid
	}
	return prop(name, recurrenceStamp(form, occurrence))
}

// componentRecurrenceIDState matches a detached override against the requested
// occurrence by parsed instant rather than exact text, because providers may
// encode the same instant differently from the master (a UTC-form
// RECURRENCE-ID over a TZID master is common). A duplicate override for one
// logical occurrence corrupts the series on every client, so encoding-level
// mismatches must still be recognized as the same occurrence.
func componentRecurrenceIDState(component []string, form startForm, occurrence time.Time) (matched, ranged bool) {
	lines, direct, err := directLines(refold(component))
	if err != nil {
		return false, false
	}
	for index := range direct {
		line := lines[index]
		if propName(line) != "RECURRENCE-ID" {
			continue
		}
		if !recurrenceIDValueMatches(line, form, occurrence) {
			continue
		}
		// RANGE=THISANDFUTURE changes provider recurrence semantics and is
		// deliberately outside Dash-Go's one-occurrence writer boundary.
		if strings.Contains(strings.ToUpper(line), ";RANGE=") {
			return true, true
		}
		return true, false
	}
	return false, false
}

func recurrenceIDValueMatches(line string, form startForm, occurrence time.Time) bool {
	params := strings.ToUpper(line)
	if split := strings.Index(params, ":"); split >= 0 {
		params = params[:split]
	}
	value := strings.TrimSpace(propValue(line))
	if value == "" {
		return false
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
	// Date-form override identity compares by the occurrence's civil day.
	if strings.Contains(params, "VALUE=DATE") || len(value) == 8 {
		day, err := time.Parse("20060102", value)
		if err != nil {
			return false
		}
		civil := occurrence.In(loc)
		return day.Year() == civil.Year() && day.Month() == civil.Month() && day.Day() == civil.Day()
	}
	valueLoc := loc
	if p := strings.Index(params, "TZID="); p >= 0 {
		raw := line
		if split := strings.Index(raw, ":"); split >= 0 {
			raw = raw[:split]
		}
		after := raw[p+5:]
		if end := strings.Index(after, ";"); end >= 0 {
			after = after[:end]
		}
		if loaded, err := time.LoadLocation(strings.Trim(after, "\"")); err == nil {
			valueLoc = loaded
		}
	}
	if strings.HasSuffix(value, "Z") {
		parsed, err := time.Parse("20060102T150405Z", value)
		if err != nil {
			return false
		}
		return parsed.Equal(occurrence.UTC())
	}
	parsed, err := time.ParseInLocation("20060102T150405", value, valueLoc)
	if err != nil {
		return false
	}
	return parsed.UTC().Equal(occurrence.UTC())
}

func componentSequence(component []string) int {
	lines, direct, err := directLines(refold(component))
	if err != nil {
		return 0
	}
	for index := range direct {
		if propName(lines[index]) != "SEQUENCE" {
			continue
		}
		value, err := strconv.Atoi(strings.TrimSpace(propValue(lines[index])))
		if err == nil && value > 0 {
			return value
		}
		return 0
	}
	return 0
}

func newOccurrenceComponent(uid string, recurrenceLine string, event Event, masterForm startForm, masterSequence int, now time.Time) []string {
	startLine, endLine := eventTimeLines(event, masterForm)
	stamp := utcStamp(now)
	lines := []string{
		"BEGIN:VEVENT",
		prop("UID", uid),
		recurrenceLine,
		prop("DTSTAMP", stamp),
		prop("CREATED", stamp),
		prop("LAST-MODIFIED", stamp),
		"SEQUENCE:" + strconv.Itoa(masterSequence),
		startLine,
		endLine,
		prop("SUMMARY", escapeText(strings.TrimSpace(event.Title))),
	}
	if strings.TrimSpace(event.Desc) != "" {
		lines = append(lines, prop("DESCRIPTION", escapeText(event.Desc)))
	}
	if strings.TrimSpace(event.Location) != "" {
		lines = append(lines, prop("LOCATION", escapeText(event.Location)))
	}
	return append(lines, "END:VEVENT")
}

func insertBeforeCalendarEnd(lines []string, component []string) ([]string, error) {
	for i := len(lines) - 1; i >= 0; i-- {
		if propName(lines[i]) == "END" && strings.EqualFold(strings.TrimSpace(propValue(lines[i])), "VCALENDAR") {
			out := make([]string, 0, len(lines)+len(component))
			out = append(out, lines[:i]...)
			out = append(out, component...)
			out = append(out, lines[i:]...)
			return out, nil
		}
	}
	return nil, errors.New("VCALENDAR end is missing")
}

// ApplySeriesEdit updates only a simple recurrence master. Its rule, EXDATE
// days, alarms, and provider fields are left in place. A series with detached
// occurrence changes stays provider-managed because moving DTSTART would make
// those provider-owned exception identities ambiguous.
func ApplySeriesEdit(src string, event Event, now time.Time) (string, error) {
	if err := event.Validate(); err != nil {
		return "", err
	}
	lines, components, master, err := documentMaster(src, event.UID)
	if err != nil {
		return "", err
	}
	if !master.recurring {
		return "", errors.New("event is not a recurring series")
	}
	for _, component := range components {
		if component.scheduling {
			return "", errors.New("events with attendees are read-only on this dashboard")
		}
		if component.recurrenceID != "" {
			return "", errors.New("series with existing occurrence changes must be managed in its calendar app")
		}
	}
	// Route-level checks already gate this, but the writer itself is the final
	// boundary: an advanced repeat pattern (selectors, RDATE, multiple rules)
	// must never be rewritten here even by a direct caller.
	if !SeriesEditable(src, event.UID) {
		return "", errors.New("this repeating schedule uses an advanced repeat pattern; manage the series in its calendar app")
	}
	// A series edit must leave its recurrence instructions intact. This keeps
	// advanced provider rules provider-owned even when the basic fields are safe.
	masterLines := componentLines(lines, master)
	oldForm, err := componentStartForm(masterLines)
	if err != nil {
		return "", err
	}
	edited, err := editEventComponent(masterLines, event, now, true)
	if err != nil {
		return "", err
	}
	newForm, err := componentStartForm(edited)
	if err != nil {
		return "", err
	}
	oldStart, newStart := time.Time{}, time.Time{}
	if oldForm.utc && !oldForm.allDay && newForm.utc && !newForm.allDay {
		oldStart, err = utcMasterStartInstant(masterLines)
		if err != nil {
			return "", err
		}
		newStart, err = utcMasterStartInstant(edited)
		if err != nil {
			return "", err
		}
	}
	// EXDATE values identify recurrence instances. Local/TZID and floating
	// masters retain their civil occurrence day as their wall time changes. UTC
	// masters retain the matching absolute offset from DTSTART instead, so a
	// legitimate edit across UTC midnight cannot resurrect a skipped instance.
	edited, err = remapExdates(edited, oldForm, newForm, oldStart, newStart)
	if err != nil {
		return "", err
	}
	return refold(replaceComponent(lines, master, edited)), nil
}

// ApplyOccurrenceEdit changes exactly one displayed occurrence. It creates a
// detached VEVENT when needed, or revises the existing detached VEVENT for the
// same RECURRENCE-ID. The recurrence master and future occurrences remain
// unchanged.
func ApplyOccurrenceEdit(src string, uid string, occurrence time.Time, event Event, now time.Time) (string, error) {
	uid = strings.TrimSpace(uid)
	event.UID = uid
	if err := event.Validate(); err != nil {
		return "", err
	}
	if occurrence.IsZero() {
		return "", errors.New("recurrence occurrence required")
	}
	lines, components, master, err := documentMaster(src, uid)
	if err != nil {
		return "", err
	}
	if !master.recurring {
		return "", errors.New("event is not a recurring series")
	}
	for _, component := range components {
		if component.scheduling {
			return "", errors.New("events with attendees are read-only on this dashboard")
		}
	}
	masterLines := componentLines(lines, master)
	form, err := componentStartForm(masterLines)
	if err != nil {
		return "", err
	}
	for _, component := range components {
		if component.uid != uid || component.recurrenceID == "" {
			continue
		}
		current := componentLines(lines, component)
		matched, ranged := componentRecurrenceIDState(current, form, occurrence)
		if !matched {
			continue
		}
		if ranged {
			return "", errors.New("this occurrence is part of a this-and-future change; manage it in its calendar app")
		}
		edited, err := editEventComponent(current, event, now, true)
		if err != nil {
			return "", err
		}
		return refold(replaceComponent(lines, component, edited)), nil
	}
	created := newOccurrenceComponent(uid, recurrenceIDLine(form, occurrence), event, form, componentSequence(masterLines), now)
	out, err := insertBeforeCalendarEnd(lines, created)
	if err != nil {
		return "", err
	}
	return refold(out), nil
}
