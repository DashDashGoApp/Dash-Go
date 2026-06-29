package chores

import (
	"slices"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func Default() map[string]any {
	return map[string]any{
		"schema": Schema, "revision": 0, "people": []any{}, "chores": []any{}, "assignments": []any{},
		"settings": map[string]any{"horizonDays": 14, "calendarOutputEnabled": true},
	}
}

func ID(v any) string {
	s := jsonutil.StringValue(v)
	if len(s) > 96 {
		s = s[:96]
	}
	return s
}

func Text(v any, limit int) string {
	s := jsonutil.StringValue(v)
	if len(s) > limit {
		s = s[:limit]
	}
	return s
}

func DateKey(v any) string {
	s := jsonutil.StringValue(v)
	if len(s) != len("2006-01-02") {
		return ""
	}
	if _, err := time.Parse("2006-01-02", s); err != nil {
		return ""
	}
	return s
}

func Anchor(v, createdAt any, now time.Time) string {
	if key := DateKey(v); key != "" {
		return key
	}
	if stamp, err := time.Parse(time.RFC3339, jsonutil.StringValue(createdAt)); err == nil {
		return LocalDateKey(stamp)
	}
	return LocalDateKey(now)
}

func IDs(items []any) map[string]bool {
	out := map[string]bool{}
	for _, item := range items {
		if id := ID(jsonutil.Map(item)["id"]); id != "" {
			out[id] = true
		}
	}
	return out
}

func Eligible(raw any, people map[string]bool) []any {
	seen, out := map[string]bool{}, []any{}
	for _, value := range jsonutil.List(raw) {
		id := ID(value)
		if id != "" && people[id] && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

func Normalize(raw map[string]any) map[string]any {
	return NormalizeAt(raw, time.Now())
}

func NormalizeAt(raw map[string]any, now time.Time) map[string]any {
	out := Default()
	out["revision"] = max(0, jsonutil.Int(raw["revision"], 0))

	people, seenPeople := []any{}, map[string]bool{}
	for _, item := range jsonutil.List(raw["people"]) {
		row := jsonutil.Map(item)
		id, name := ID(row["id"]), Text(row["name"], 64)
		if id == "" || name == "" || seenPeople[id] {
			continue
		}
		seenPeople[id] = true
		people = append(people, map[string]any{"id": id, "name": name})
	}
	out["people"] = people
	peopleIDs := IDs(people)

	chores, seenChores := []any{}, map[string]bool{}
	for _, item := range jsonutil.List(raw["chores"]) {
		row := jsonutil.Map(item)
		id, name := ID(row["id"]), Text(row["name"], 96)
		if id == "" || name == "" || seenChores[id] {
			continue
		}
		seenChores[id] = true
		created := Text(row["createdAt"], 48)
		cadence := jsonutil.Map(row["cadence"])
		typ := Text(cadence["type"], 16)
		if typ != "daily" && typ != "weekdays" && typ != "weekly" && typ != "days" {
			typ = "daily"
		}
		chores = append(chores, map[string]any{
			"id": id, "name": name, "createdAt": created,
			"cadence": map[string]any{
				"type": typ, "day": clamp(jsonutil.Int(cadence["day"], 0), 0, 6),
				"every":      clamp(jsonutil.Int(cadence["every"], 1), 1, 365),
				"anchorDate": Anchor(cadence["anchorDate"], created, now),
			},
			"effort":   clamp(jsonutil.Int(row["effort"], 2), 1, 3),
			"eligible": Eligible(row["eligible"], peopleIDs),
		})
	}
	out["chores"] = chores
	choreIDs := IDs(chores)

	assignments, seenAssignments := []any{}, map[string]bool{}
	for _, item := range jsonutil.List(raw["assignments"]) {
		row := jsonutil.Map(item)
		id, date := ID(row["id"]), DateKey(row["date"])
		choreID, personID := ID(row["choreId"]), ID(row["personId"])
		if id == "" || date == "" || choreID == "" || personID == "" || seenAssignments[id] {
			continue
		}
		seenAssignments[id] = true
		status := Text(row["status"], 16)
		if status != "completed" && status != "skipped" {
			status = "assigned"
		}
		choreName, personName := Text(row["choreName"], 96), Text(row["personName"], 64)
		if choreName == "" && choreIDs[choreID] {
			for _, candidate := range chores {
				if m := jsonutil.Map(candidate); m["id"] == choreID {
					choreName = Text(m["name"], 96)
					break
				}
			}
		}
		if personName == "" && peopleIDs[personID] {
			for _, candidate := range people {
				if m := jsonutil.Map(candidate); m["id"] == personID {
					personName = Text(m["name"], 64)
					break
				}
			}
		}
		assignments = append(assignments, map[string]any{
			"id": id, "date": date, "choreId": choreID, "choreName": choreName,
			"personId": personID, "personName": personName, "status": status, "source": Text(row["source"], 24),
		})
	}
	slices.SortFunc(assignments, func(left, right any) int {
		return strings.Compare(stringValue(jsonutil.Map(left)["date"]), stringValue(jsonutil.Map(right)["date"]))
	})
	out["assignments"] = assignments

	settings := jsonutil.Map(raw["settings"])
	out["settings"] = map[string]any{
		"horizonDays":           clamp(jsonutil.Int(settings["horizonDays"], 14), 1, 30),
		"calendarOutputEnabled": settings["calendarOutputEnabled"] == nil || jsonutil.Truthy(settings["calendarOutputEnabled"]),
	}
	return out
}

func NextRevision(payload map[string]any) map[string]any {
	next := Normalize(payload)
	next["revision"] = max(0, jsonutil.Int(next["revision"], 0)) + 1
	return next
}

func stringValue(v any) string {
	if value, ok := v.(string); ok {
		return value
	}
	return ""
}

func clamp(v, low, high int) int {
	if v < low {
		return low
	}
	if v > high {
		return high
	}
	return v
}
