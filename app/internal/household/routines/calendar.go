package routines

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

type CalendarEvent struct {
	Date        string
	Start       *time.Time
	End         *time.Time
	Summary     string
	Description string
	UID         string
	AppOwner    string
}

func CalendarOutputEnabled(payload map[string]any) bool {
	return BoolDefault(jsonutil.Map(payload["settings"])["calendarOutputEnabled"], true)
}
func CalendarRange(payload map[string]any, now time.Time) (time.Time, time.Time) {
	horizon := clamp(jsonutil.Int(jsonutil.Map(payload["settings"])["calendarHorizonDays"], 56), 7, 90)
	local := now.In(time.Local)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.Local)
	return today.AddDate(0, 0, -7), today.AddDate(0, 0, horizon)
}

type bucket struct {
	date, personID, personName, clock string
	allDay                            bool
	sessions                          []map[string]any
}

func CalendarEvents(payload map[string]any, now time.Time) []CalendarEvent {
	start, end := CalendarRange(payload, now)
	buckets := map[string]*bucket{}
	for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
		key := day.Format("2006-01-02")
		for _, occ := range OccurrencesForDay(payload, key, now) {
			if occ["state"] == "skipped" || !BoolDefault(occ["calendarEnabled"], true) {
				continue
			}
			allDay := BoolDefault(occ["allDay"], true)
			clock := Clock(occ["time"])
			personID := ID(occ["personId"])
			bucketKey := strings.Join([]string{key, personID, clock, fmt.Sprint(allDay)}, "|")
			b := buckets[bucketKey]
			if b == nil {
				b = &bucket{date: key, personID: personID, personName: Text(occ["personName"], 64), clock: clock, allDay: allDay}
				buckets[bucketKey] = b
			}
			b.sessions = append(b.sessions, occ)
		}
	}
	keys := slices.Sorted(maps.Keys(buckets))
	events := []CalendarEvent{}
	for _, key := range keys {
		b := buckets[key]
		if _, err := time.ParseInLocation("2006-01-02", b.date, time.Local); err != nil {
			continue
		}
		count := len(b.sessions)
		if count == 0 {
			continue
		}
		title := fmt.Sprintf("Routines — %s · %d", b.personName, count)
		uid := "routine-session-" + strings.ReplaceAll(b.personID, " ", "-") + "-" + b.date + "-" + strings.ReplaceAll(b.clock, ":", "")
		labels := make([]string, 0, count)
		for _, session := range b.sessions {
			labels = append(labels, Text(session["routineTitle"], 120))
		}
		slices.Sort(labels)
		event := CalendarEvent{Date: b.date, Summary: title, Description: "Dash-Go Routines\n" + strings.Join(labels, " · ") + " · " + b.personName, UID: uid, AppOwner: "routines"}
		if !b.allDay && b.clock != "" {
			if stamp, ok := DateTime(b.date, b.clock, false); ok {
				endStamp := stamp.Add(15 * time.Minute)
				event.Start, event.End = &stamp, &endStamp
			}
		}
		events = append(events, event)
		if len(events) > MaxCalendarSessions {
			return events
		}
	}
	return events
}
