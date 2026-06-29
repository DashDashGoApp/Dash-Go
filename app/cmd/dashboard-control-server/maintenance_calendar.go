package main

import calendarpkg "github.com/DashDashGoApp/Dash-Go/app/internal/calendar"

func (a *app) writeMaintenanceCalendar(payload map[string]any) error {
	return writeICSFile(maintenanceCalendarPath(a), "Maintenance", maintenanceCalendarEvents(payload))
}
func (a *app) commitMaintenancePayload(payload map[string]any) error {
	return a.calendarService().CommitOwnedFeed(calendarpkg.OwnedFeedCommit{
		Owner: "maintenance", Name: "Maintenance", Events: maintenanceCalendarEvents(payload),
		Enabled: maintenanceCalendarOutputEnabled(payload), OutputState: a.calendarOutputSnapshot("maintenance", maintenanceCalendarOutputEnabled(payload)),
		Save: func() error { return a.maintenanceService().Write(payload) },
	})
}
