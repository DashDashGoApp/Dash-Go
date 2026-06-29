package main

import (
	"net/http"
	"slices"
	"strings"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func routinesResponse(payload map[string]any) map[string]any {
	return map[string]any{"state": payload, "today": routinesToday(), "summary": routinesSummary(payload)}
}
func routinesSummary(payload map[string]any) map[string]any {
	today := routinesToday()
	sessions := routinesOccurrencesForDay(payload, today)
	people := map[string]int{}
	complete := 0
	for _, session := range sessions {
		people[routinesID(session["personId"])]++
		if session["state"] == "completed" {
			complete++
		}
	}
	return map[string]any{"today": today, "due": len(sessions), "completed": complete, "people": len(people)}
}
func routinesDayResponse(payload map[string]any, date string) map[string]any {
	groups := map[string]map[string]any{}
	sessions := routinesOccurrencesForDay(payload, date)
	for _, session := range sessions {
		pid := routinesID(session["personId"])
		group := groups[pid]
		if group == nil {
			group = map[string]any{"id": pid, "name": routinesText(session["personName"], 64), "sessions": []any{}}
			groups[pid] = group
		}
		group["sessions"] = append(jsonutil.List(group["sessions"]), session)
	}
	people := []any{}
	for _, group := range groups {
		people = append(people, group)
	}
	slices.SortStableFunc(people, func(l, r any) int {
		return strings.Compare(routinesText(jsonutil.Map(l)["name"], 64), routinesText(jsonutil.Map(r)["name"], 64))
	})
	return map[string]any{"date": date, "people": people, "count": len(sessions)}
}
func (a *app) handleRoutinesGet(w http.ResponseWriter, r *http.Request, path string) bool {
	switch path {
	case "/api/routines":
		a.json(w, routinesResponse(a.routinesPayload()))
		return true
	case "/api/routines/day":
		date := routinesDate(r.URL.Query().Get("date"))
		if date == "" {
			a.err(w, "date must be YYYY-MM-DD", http.StatusBadRequest)
			return true
		}
		a.json(w, routinesDayResponse(a.routinesPayload(), date))
		return true
	default:
		return false
	}
}
