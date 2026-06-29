package chores

import (
	"fmt"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

type CalendarEvent struct {
	Date     string
	Summary  string
	UID      string
	AppOwner string
}

func EventTitle(chore, person string) string {
	return strings.TrimSpace(fmt.Sprintf("%s — %s", chore, person))
}

func CalendarRange(payload map[string]any, now time.Time) (time.Time, time.Time) {
	settings := jsonutil.Map(payload["settings"])
	horizon := clamp(jsonutil.Int(settings["horizonDays"], 14), 1, 30)
	local := now.In(time.Local)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.Local)
	return today.AddDate(0, 0, -30), today.AddDate(0, 0, horizon)
}

func CalendarOutputEnabled(payload map[string]any) bool {
	settings := jsonutil.Map(payload["settings"])
	if _, present := settings["calendarOutputEnabled"]; !present {
		return true
	}
	return jsonutil.Truthy(settings["calendarOutputEnabled"])
}

func (s *Service) CalendarEvents(payload map[string]any) []CalendarEvent {
	payload = NormalizeAt(payload, s.Now())
	start, end := CalendarRange(payload, s.Now())
	now := s.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	activeChores := IDs(jsonutil.List(payload["chores"]))
	activePeople := IDs(jsonutil.List(payload["people"]))
	events := []CalendarEvent{}
	for _, item := range jsonutil.List(payload["assignments"]) {
		row := jsonutil.Map(item)
		day := DateKey(row["date"])
		local, ok := DateFromKey(day)
		if !ok || local.Before(start) || local.After(end) {
			continue
		}
		status := Text(row["status"], 16)
		if (status == "completed" || status == "skipped") && local.After(today) {
			continue
		}
		if !activeChores[ID(row["choreId"])] || !activePeople[ID(row["personId"])] {
			continue
		}
		title := EventTitle(Text(row["choreName"], 96), Text(row["personName"], 64))
		if title == "—" || title == "" {
			continue
		}
		switch status {
		case "completed":
			title = "✓ " + title
		case "skipped":
			title = "↷ " + title
		}
		events = append(events, CalendarEvent{Date: day, Summary: title, UID: "chore-wheel-" + strings.ReplaceAll(ID(row["id"]), " ", "-"), AppOwner: "chore-wheel"})
	}
	return events
}
