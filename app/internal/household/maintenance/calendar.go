package maintenance

import (
	"fmt"
	"strings"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

type CalendarEvent struct {
	Date        string
	Summary     string
	Description string
	UID         string
	AppOwner    string
}

func CalendarOutputEnabled(payload map[string]any) bool {
	settings := jsonutil.Map(payload["settings"])
	if _, ok := settings["calendarOutputEnabled"]; !ok {
		return true
	}
	return jsonutil.Truthy(settings["calendarOutputEnabled"])
}
func CalendarEvents(payload map[string]any) []CalendarEvent {
	events := []CalendarEvent{}
	for _, raw := range jsonutil.List(payload["tasks"]) {
		task := jsonutil.Map(raw)
		if task["state"] != "active" || !jsonutil.Truthy(task["calendarEnabled"]) {
			continue
		}
		day := Date(task["nextDueOn"])
		if _, ok := DateTime(day); !ok {
			continue
		}
		cadence := jsonutil.Map(task["cadence"])
		description := fmt.Sprintf("Dash-Go Maintenance Tracker\nEvery %d %s", Every(Unit(cadence["unit"]), cadence["every"]), Unit(cadence["unit"]))
		if person := TaskPersonSnapshot(task); person != "" {
			description += "\nResponsible: " + person
		}
		if last := Date(task["lastCompletedOn"]); last != "" {
			description += "\nLast completed: " + last
		}
		events = append(events, CalendarEvent{Date: day, Summary: Text(task["title"], 120), Description: description, UID: "maintenance-" + strings.ReplaceAll(ID(task["id"]), " ", "-") + "-" + day, AppOwner: "maintenance"})
	}
	return events
}
