package icalwrite

import (
	"errors"
	"strings"
	"time"
)

// SeriesEditable reports the intentionally narrow safe set for moving an
// entire series from Dash-Go. Selector-heavy rules are still displayable and
// support per-occurrence edits, but their cadence remains provider-managed.
func SeriesEditable(src, uid string) bool {
	lines, components, master, err := documentMaster(src, uid)
	if err != nil || !master.recurring {
		return false
	}
	for _, component := range components {
		if component.scheduling || component.recurrenceID != "" {
			return false
		}
	}
	masterLines := componentLines(lines, master)
	direct, err := componentDirect(masterLines)
	if err != nil {
		return false
	}
	rule := ""
	for index := range direct {
		name := propName(masterLines[index])
		if name == "RDATE" {
			return false
		}
		if name == "RRULE" {
			if rule != "" {
				return false
			}
			rule = strings.TrimSpace(propValue(masterLines[index]))
		}
	}
	if rule == "" {
		return false
	}
	allowed := map[string]bool{"FREQ": true, "INTERVAL": true, "COUNT": true, "UNTIL": true}
	freq := ""
	for _, raw := range strings.Split(rule, ";") {
		pair := strings.SplitN(strings.TrimSpace(raw), "=", 2)
		if len(pair) != 2 || !allowed[strings.ToUpper(pair[0])] {
			return false
		}
		if strings.EqualFold(pair[0], "FREQ") {
			freq = strings.ToUpper(strings.TrimSpace(pair[1]))
		}
	}
	switch freq {
	case "DAILY", "WEEKLY", "MONTHLY", "YEARLY":
		return true
	default:
		return false
	}
}

func componentDirect(lines []string) (map[int]bool, error) {
	joined := refold(lines)
	parsed, direct, err := directLines(joined)
	if err != nil {
		return nil, err
	}
	_ = parsed
	return direct, nil
}

// validateOccurrenceTarget is kept close to the writer so callers cannot
// accidentally create detached exceptions for a non-recurring source.
func ValidateOccurrenceTarget(src, uid string, occurrence time.Time) error {
	if strings.TrimSpace(uid) == "" || occurrence.IsZero() {
		return errors.New("calendar occurrence required")
	}
	_, components, master, err := documentMaster(src, uid)
	if err != nil {
		return err
	}
	if !master.recurring {
		return errors.New("event is not a recurring series")
	}
	for _, component := range components {
		if component.scheduling {
			return errors.New("events with attendees are read-only on this dashboard")
		}
	}
	return nil
}

// OccurrenceHasOverride reports whether the recurrence master already carries
// a detached VEVENT for the requested logical instance. EXDATE suppresses only
// the master expansion; it does not suppress a detached override. Callers use
// this guard to avoid promising a successful "Skip this occurrence" action
// that would leave a custom instance visible on provider clients.
func OccurrenceHasOverride(src, uid string, occurrence time.Time) (bool, error) {
	if strings.TrimSpace(uid) == "" || occurrence.IsZero() {
		return false, errors.New("calendar occurrence required")
	}
	lines, components, master, err := documentMaster(src, uid)
	if err != nil {
		return false, err
	}
	if !master.recurring {
		return false, errors.New("event is not a recurring series")
	}
	masterLines := componentLines(lines, master)
	form, err := componentStartForm(masterLines)
	if err != nil {
		return false, err
	}
	for _, component := range components {
		if component.uid != uid || component.recurrenceID == "" {
			continue
		}
		matched, _ := componentRecurrenceIDState(componentLines(lines, component), form, occurrence)
		if matched {
			return true, nil
		}
	}
	return false, nil
}
