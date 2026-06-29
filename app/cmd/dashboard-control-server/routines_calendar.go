package main

import (
	"fmt"
	calendarpkg "github.com/DashDashGoApp/Dash-Go/app/internal/calendar"
)

func (a *app) writeRoutinesCalendar(payload map[string]any) error {
	events := routinesCalendarEvents(payload)
	if len(events) > routinesMaxCalendarSessions {
		return fmt.Errorf("Routines schedule would create more than %d calendar sessions; reduce the calendar horizon, people, or cadence density", routinesMaxCalendarSessions)
	}
	return writeICSFile(routinesCalendarPath(a), "Routines", events)
}
func (a *app) commitRoutinesPayload(payload map[string]any) error {
	events := routinesCalendarEvents(payload)
	if len(events) > routinesMaxCalendarSessions {
		return fmt.Errorf("Routines schedule would create more than %d calendar sessions; reduce the calendar horizon, people, or cadence density", routinesMaxCalendarSessions)
	}
	return a.calendarService().CommitOwnedFeed(calendarpkg.OwnedFeedCommit{
		Owner: "routines", Name: "Routines", Events: events, Enabled: routinesCalendarOutputEnabled(payload), OutputState: a.calendarOutputSnapshot("routines", routinesCalendarOutputEnabled(payload)),
		Save: func() error { return a.routinesService().Write(payload) },
	})
}
func (a *app) saveRoutinesStateOnly(payload map[string]any) error {
	return a.routinesService().Write(payload)
}
