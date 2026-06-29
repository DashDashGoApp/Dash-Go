// Package routines owns Dash-Go's local Routines model, schedule expansion,
// occurrence state, correction history, and generated-calendar projection.
// The package receives no app pointer and never imports package main, People,
// Calendar persistence, or a sibling domain service.
package routines

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

const (
	Schema              = 1
	MaxPeople           = 20
	MaxItems            = 100
	MaxSteps            = 30
	MaxOccurrences      = 1000
	MaxHistory          = 1000
	HistoryDays         = 180
	MaxCalendarSessions = 500
)

func Default() map[string]any {
	return map[string]any{"schema": Schema, "revision": 0, "settings": map[string]any{"calendarOutputEnabled": true, "calendarHorizonDays": 56, "defaultCalendarEnabled": true}, "people": []any{}, "routines": []any{}, "occurrences": []any{}, "history": []any{}}
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
func Text(v any, limit int) string {
	s := strings.TrimSpace(jsonutil.StringValue(v))
	if len([]rune(s)) > limit {
		s = string([]rune(s)[:limit])
	}
	return s
}
func ID(v any) string { return strings.ReplaceAll(Text(v, 96), " ", "-") }
func Date(v any) string {
	s := jsonutil.StringValue(v)
	if len(s) != 10 {
		return ""
	}
	if _, err := time.ParseInLocation("2006-01-02", s, time.Local); err != nil {
		return ""
	}
	return s
}
func Clock(v any) string {
	s := jsonutil.StringValue(v)
	if s == "" {
		return ""
	}
	if _, err := time.Parse("15:04", s); err != nil {
		return ""
	}
	return s
}
func Stamp(v any) string {
	s := jsonutil.StringValue(v)
	if _, err := time.Parse(time.RFC3339, s); err != nil {
		return ""
	}
	return s
}
func State(v any) string {
	if strings.EqualFold(jsonutil.StringValue(v), "archived") {
		return "archived"
	}
	return "active"
}
func BoolDefault(v any, fallback bool) bool {
	if v == nil {
		return fallback
	}
	return jsonutil.Truthy(v)
}
func Weekdays(v any) []any {
	allowed := map[string]bool{"MO": true, "TU": true, "WE": true, "TH": true, "FR": true, "SA": true, "SU": true}
	seen := map[string]bool{}
	out := []any{}
	for _, raw := range jsonutil.List(v) {
		day := strings.ToUpper(Text(raw, 2))
		if allowed[day] && !seen[day] {
			seen[day] = true
			out = append(out, day)
		}
	}
	if len(out) == 0 {
		out = []any{"MO", "TU", "WE", "TH", "FR"}
	}
	return out
}
func Schedule(raw any, fallbackDate string, now time.Time) map[string]any {
	row := jsonutil.Map(raw)
	kind := strings.ToLower(Text(row["kind"], 16))
	allowed := map[string]bool{"days": true, "weekdays": true, "weekly": true, "monthly": true, "yearly": true, "once": true}
	if !allowed[kind] {
		kind = "weekdays"
	}
	start := Date(row["startOn"])
	if start == "" {
		start = Date(fallbackDate)
	}
	if start == "" {
		start = now.In(time.Local).Format("2006-01-02")
	}
	end := Date(row["endOn"])
	if end != "" && end < start {
		end = ""
	}
	everyMax := 365
	if kind == "monthly" {
		everyMax = 60
	}
	if kind == "yearly" {
		everyMax = 20
	}
	return map[string]any{"kind": kind, "every": clamp(jsonutil.Int(row["every"], 1), 1, everyMax), "weekdays": Weekdays(row["weekdays"]), "startOn": start, "endOn": end, "month": clamp(jsonutil.Int(row["month"], 1), 1, 12), "day": clamp(jsonutil.Int(row["day"], 1), 1, 31), "time": Clock(row["time"]), "allDay": BoolDefault(row["allDay"], true)}
}
func person(raw any, now time.Time) map[string]any {
	row := jsonutil.Map(raw)
	id, name := ID(row["id"]), Text(row["name"], 64)
	if id == "" || name == "" {
		return nil
	}
	created := Stamp(row["createdAt"])
	if created == "" {
		created = now.Format(time.RFC3339)
	}
	updated := Stamp(row["updatedAt"])
	if updated == "" {
		updated = created
	}
	return map[string]any{"id": id, "name": name, "state": State(row["state"]), "createdAt": created, "updatedAt": updated, "archivedAt": Stamp(row["archivedAt"])}
}
func step(raw any) map[string]any {
	row := jsonutil.Map(raw)
	id, text := ID(row["id"]), Text(row["text"], 140)
	if id == "" || text == "" {
		return nil
	}
	return map[string]any{"id": id, "text": text, "position": clamp(jsonutil.Int(row["position"], 10), 1, 100000)}
}
func assignment(raw any, people map[string]string, fallback string, now time.Time) map[string]any {
	row := jsonutil.Map(raw)
	id, pid := ID(row["id"]), ID(row["personId"])
	name, exists := people[pid]
	if id == "" || pid == "" || !exists {
		return nil
	}
	return map[string]any{"id": id, "personId": pid, "personNameSnapshot": name, "calendarEnabled": BoolDefault(row["calendarEnabled"], true), "schedule": Schedule(row["schedule"], fallback, now)}
}
func item(raw any, people map[string]string, now time.Time) map[string]any {
	row := jsonutil.Map(raw)
	id, title := ID(row["id"]), Text(row["title"], 120)
	if id == "" || title == "" {
		return nil
	}
	created := Stamp(row["createdAt"])
	if created == "" {
		created = now.Format(time.RFC3339)
	}
	updated := Stamp(row["updatedAt"])
	if updated == "" {
		updated = created
	}
	steps, seenSteps := []any{}, map[string]bool{}
	for _, rawStep := range jsonutil.List(row["steps"]) {
		s := step(rawStep)
		stepID := jsonutil.StringValue(s["id"])
		if s != nil && stepID != "" && !seenSteps[stepID] {
			seenSteps[stepID] = true
			steps = append(steps, s)
		}
		if len(steps) >= MaxSteps {
			break
		}
	}
	if len(steps) == 0 {
		return nil
	}
	slices.SortStableFunc(steps, func(l, r any) int {
		return jsonutil.Int(jsonutil.Map(l)["position"], 0) - jsonutil.Int(jsonutil.Map(r)["position"], 0)
	})
	assignments, seenAssignments := []any{}, map[string]bool{}
	for _, rawAssignment := range jsonutil.List(row["assignments"]) {
		a := assignment(rawAssignment, people, created, now)
		assignmentID := jsonutil.StringValue(a["id"])
		if a != nil && assignmentID != "" && !seenAssignments[assignmentID] {
			seenAssignments[assignmentID] = true
			assignments = append(assignments, a)
		}
	}
	return map[string]any{"id": id, "title": title, "note": Text(row["note"], 280), "state": State(row["state"]), "steps": steps, "assignments": assignments, "createdAt": created, "updatedAt": updated, "archivedAt": Stamp(row["archivedAt"])}
}
func occurrence(raw any, routineIDs, assignmentIDs map[string]bool, people map[string]string) map[string]any {
	row := jsonutil.Map(raw)
	id, rid, aid, pid := ID(row["id"]), ID(row["routineId"]), ID(row["assignmentId"]), ID(row["personId"])
	date := Date(row["date"])
	name, exists := people[pid]
	if id == "" || rid == "" || aid == "" || pid == "" || date == "" || !routineIDs[rid] || !assignmentIDs[aid] || !exists {
		return nil
	}
	state := strings.ToLower(Text(row["state"], 16))
	if state != "completed" && state != "skipped" {
		state = "active"
	}
	completed, seen := []any{}, map[string]bool{}
	for _, sid := range jsonutil.List(row["completedStepIds"]) {
		value := ID(sid)
		if value != "" && !seen[value] {
			seen[value] = true
			completed = append(completed, value)
		}
	}
	nameSnap := Text(row["personNameSnapshot"], 64)
	if nameSnap == "" {
		nameSnap = name
	}
	return map[string]any{"id": id, "routineId": rid, "assignmentId": aid, "personId": pid, "personNameSnapshot": nameSnap, "date": date, "time": Clock(row["time"]), "allDay": BoolDefault(row["allDay"], true), "state": state, "completedStepIds": completed, "completedAt": Stamp(row["completedAt"]), "skippedAt": Stamp(row["skippedAt"])}
}

func Normalize(raw map[string]any, now time.Time) map[string]any {
	out := Default()
	out["revision"] = max(0, jsonutil.Int(raw["revision"], 0))
	settings := jsonutil.Map(raw["settings"])
	out["settings"] = map[string]any{"calendarOutputEnabled": BoolDefault(settings["calendarOutputEnabled"], true), "calendarHorizonDays": clamp(jsonutil.Int(settings["calendarHorizonDays"], 56), 7, 90), "defaultCalendarEnabled": BoolDefault(settings["defaultCalendarEnabled"], true)}
	people, names := []any{}, map[string]string{}
	for _, rawPerson := range jsonutil.List(raw["people"]) {
		p := person(rawPerson, now)
		personID := jsonutil.StringValue(p["id"])
		if p != nil && personID != "" && names[personID] == "" {
			names[personID] = Text(p["name"], 64)
			people = append(people, p)
		}
		if len(people) >= MaxPeople {
			break
		}
	}
	out["people"] = people
	routines, routineIDs, assignmentIDs := []any{}, map[string]bool{}, map[string]bool{}
	for _, rawRoutine := range jsonutil.List(raw["routines"]) {
		r := item(rawRoutine, names, now)
		routineID := jsonutil.StringValue(r["id"])
		if r != nil && routineID != "" && !routineIDs[routineID] {
			routineIDs[routineID] = true
			for _, rawAssignment := range jsonutil.List(r["assignments"]) {
				assignmentIDs[ID(jsonutil.Map(rawAssignment)["id"])] = true
			}
			routines = append(routines, r)
		}
		if len(routines) >= MaxItems {
			break
		}
	}
	out["routines"] = routines
	occurrences := []any{}
	for _, rawOccurrence := range jsonutil.List(raw["occurrences"]) {
		o := occurrence(rawOccurrence, routineIDs, assignmentIDs, names)
		if o != nil {
			occurrences = append(occurrences, o)
		}
		if len(occurrences) >= MaxOccurrences {
			break
		}
	}
	slices.SortStableFunc(occurrences, func(l, r any) int {
		lr, rr := jsonutil.Map(l), jsonutil.Map(r)
		return -strings.Compare(fmt.Sprint(lr["date"], lr["time"], lr["id"]), fmt.Sprint(rr["date"], rr["time"], rr["id"]))
	})
	out["occurrences"] = occurrences
	out["history"] = normalizeHistory(raw["history"], now)
	return out
}
func normalizeHistory(raw any, now time.Time) []any {
	cutoff := now.AddDate(0, 0, -HistoryDays)
	out, seen := []any{}, map[string]bool{}
	for _, item := range jsonutil.List(raw) {
		row := jsonutil.Map(item)
		id := ID(row["id"])
		when := Stamp(row["occurredAt"])
		if id == "" || when == "" || seen[id] {
			continue
		}
		stamp, err := time.Parse(time.RFC3339, when)
		if err != nil || stamp.In(time.Local).Before(cutoff) {
			continue
		}
		action := Text(row["action"], 32)
		if action == "" {
			continue
		}
		seen[id] = true
		out = append(out, map[string]any{"id": id, "occurredAt": when, "date": Date(row["date"]), "action": action, "routineId": ID(row["routineId"]), "routineTitle": Text(row["routineTitle"], 120), "assignmentId": ID(row["assignmentId"]), "personId": ID(row["personId"]), "personName": Text(row["personName"], 64)})
		if len(out) >= MaxHistory {
			break
		}
	}
	slices.SortStableFunc(out, func(l, r any) int {
		return -strings.Compare(fmt.Sprint(jsonutil.Map(l)["occurredAt"]), fmt.Sprint(jsonutil.Map(r)["occurredAt"]))
	})
	return out
}

func NextRevision(payload map[string]any, now time.Time) map[string]any {
	next := Normalize(payload, now)
	next["revision"] = max(0, jsonutil.Int(next["revision"], 0)) + 1
	return next
}
func NewID(prefix string, now time.Time) string { return fmt.Sprintf("%s_%d", prefix, now.UnixNano()) }
func PersonName(payload map[string]any, id string) string {
	for _, raw := range jsonutil.List(payload["people"]) {
		row := jsonutil.Map(raw)
		if row["id"] == id {
			return Text(row["name"], 64)
		}
	}
	return ""
}
func Find(payload map[string]any, id string) (int, map[string]any) {
	for i, raw := range jsonutil.List(payload["routines"]) {
		row := jsonutil.Map(raw)
		if row["id"] == id {
			return i, row
		}
	}
	return -1, nil
}

func AppendHistory(payload map[string]any, action string, occ map[string]any, now time.Time) {
	if occ == nil {
		return
	}
	_, routine := Find(payload, ID(occ["routineId"]))
	entry := map[string]any{"id": NewID("rh", now), "occurredAt": now.Format(time.RFC3339), "date": Date(occ["date"]), "action": action, "routineId": ID(occ["routineId"]), "assignmentId": ID(occ["assignmentId"]), "personId": ID(occ["personId"]), "personName": Text(occ["personNameSnapshot"], 64)}
	if entry["personName"] == "" {
		entry["personName"] = PersonName(payload, ID(occ["personId"]))
	}
	if routine != nil {
		entry["routineTitle"] = Text(routine["title"], 120)
	}
	payload["history"] = normalizeHistory(append(jsonutil.List(payload["history"]), entry), now)
}
