package main

import (
	"path/filepath"
	"time"

	calendarpkg "github.com/DashDashGoApp/Dash-Go/app/internal/calendar"
)

func (a *app) choreWheelCalendarEvents(payload map[string]any) []calendarGenEvent {
	projected := a.choreWheelService().CalendarEvents(payload)
	events := make([]calendarGenEvent, 0, len(projected))
	for _, row := range projected {
		parsed, err := time.Parse("2006-01-02", row.Date)
		if err != nil {
			continue
		}
		event := allDayEvent(parsed.Year(), parsed.Month(), parsed.Day(), row.Summary, row.UID)
		event.AppOwner = row.AppOwner
		events = append(events, event)
	}
	return events
}
func (a *app) writeChoreWheelCalendar(payload map[string]any) error {
	return writeICSFile(filepath.Join(a.calDir, "chore-wheel.ics"), "Chores", a.choreWheelCalendarEvents(payload))
}

// commitChoreWheelPayload is the Calendar service port. Chore Wheel supplies a
// durable normalized model and event projection; Calendar serializes its ICS
// feed, manifest update, cache wake-up, and rollback around the app-state save.
func (a *app) commitChoreWheelPayload(payload map[string]any) error {
	return a.calendarService().CommitOwnedFeed(calendarpkg.OwnedFeedCommit{
		Owner: "chore-wheel", Name: "Chores", Events: a.choreWheelCalendarEvents(payload),
		Enabled: choreWheelCalendarOutputEnabled(payload), OutputState: a.calendarOutputSnapshot("chore-wheel", choreWheelCalendarOutputEnabled(payload)),
		Save: func() error { return a.choreWheelService().Write(payload) },
	})
}
