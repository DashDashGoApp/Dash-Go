// Package maintenance owns Dash-Go's local Maintenance Tracker model.
// It deliberately has no dependency on package main, People, Calendar, or a
// sibling household service. Core supplies only durable paths, a local clock,
// and narrow person-resolution data when a mutation needs it.
package maintenance

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

const (
	Schema           = 3
	HistoryLimit     = 500
	TaskHistoryLimit = 50
	MaxTasks         = 100
)

func Default() map[string]any {
	return map[string]any{
		"schema":   Schema,
		"settings": map[string]any{"defaultCalendarEnabled": true, "calendarOutputEnabled": true, "dueSoonDays": 30},
		"tasks":    []any{}, "history": []any{},
	}
}

func String(v any) string {
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}
func Text(v any, limit int) string {
	s := String(v)
	if len([]rune(s)) > limit {
		s = string([]rune(s)[:limit])
	}
	return s
}
func ID(v any) string { return strings.ReplaceAll(Text(v, 96), " ", "-") }
func Date(v any) string {
	s := jsonutil.StringValue(v)
	if len(s) != len("2006-01-02") {
		return ""
	}
	if _, err := time.ParseInLocation("2006-01-02", s, time.Local); err != nil {
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
func Unit(v any) string {
	s := strings.ToLower(jsonutil.StringValue(v))
	for _, unit := range []string{"days", "weeks", "months", "years"} {
		if s == unit {
			return unit
		}
	}
	return "months"
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
func Every(unit string, v any) int {
	limits := map[string]int{"days": 365, "weeks": 52, "months": 24, "years": 10}
	return clamp(jsonutil.Int(v, 1), 1, limits[unit])
}
func DateTime(day string) (time.Time, bool) {
	parsed, err := time.ParseInLocation("2006-01-02", day, time.Local)
	return parsed, err == nil
}
func NextDue(completed, unit string, every int) string {
	base, ok := DateTime(completed)
	if !ok {
		return ""
	}
	switch unit {
	case "days":
		base = base.AddDate(0, 0, every)
	case "weeks":
		base = base.AddDate(0, 0, 7*every)
	case "years":
		base = base.AddDate(every, 0, 0)
	default:
		base = base.AddDate(0, every, 0)
	}
	return base.Format("2006-01-02")
}
func state(v any) string {
	if strings.ToLower(jsonutil.StringValue(v)) == "archived" {
		return "archived"
	}
	return "active"
}

func Task(raw any, now time.Time) map[string]any {
	row := jsonutil.Map(raw)
	id, title := ID(row["id"]), Text(row["title"], 120)
	if id == "" || title == "" {
		return nil
	}
	cadence := jsonutil.Map(row["cadence"])
	unit := Unit(cadence["unit"])
	next := Date(row["nextDueOn"])
	if next == "" {
		return nil
	}
	created, updated := Stamp(row["createdAt"]), Stamp(row["updatedAt"])
	if created == "" {
		created = now.Format(time.RFC3339)
	}
	if updated == "" {
		updated = created
	}
	return map[string]any{
		"id": id, "title": title, "note": Text(row["note"], 280), "state": state(row["state"]),
		"cadence":         map[string]any{"unit": unit, "every": Every(unit, cadence["every"])},
		"lastCompletedOn": Date(row["lastCompletedOn"]), "lastCompletionHistoryId": ID(row["lastCompletionHistoryId"]), "nextDueOn": next,
		"calendarEnabled": jsonutil.Truthy(row["calendarEnabled"]), "createdAt": created, "updatedAt": updated, "archivedAt": Stamp(row["archivedAt"]),
		"responsiblePersonId": ID(row["responsiblePersonId"]), "responsiblePersonNameSnapshot": Text(row["responsiblePersonNameSnapshot"], 64),
	}
}

func HistoryRow(raw any) map[string]any {
	row := jsonutil.Map(raw)
	id, taskID := ID(row["id"]), ID(row["taskId"])
	action := strings.ToLower(jsonutil.StringValue(row["action"]))
	allowed := map[string]bool{"completed": true, "rescheduled": true, "archived": true, "restored": true, "deleted": true, "undo": true}
	if id == "" || taskID == "" || !allowed[action] {
		return nil
	}
	occurred := Date(row["occurredOn"])
	if occurred == "" {
		return nil
	}
	return map[string]any{
		"id": id, "taskId": taskID, "action": action, "occurredOn": occurred,
		"priorDueOn": Date(row["priorDueOn"]), "nextDueOn": Date(row["nextDueOn"]),
		"priorLastCompletedOn": Date(row["priorLastCompletedOn"]), "undoOf": ID(row["undoOf"]),
		"createdAt": Stamp(row["createdAt"]), "responsiblePersonId": ID(row["responsiblePersonId"]), "responsiblePersonNameSnapshot": Text(row["responsiblePersonNameSnapshot"], 64),
	}
}

func Normalize(raw map[string]any, now time.Time) map[string]any {
	out := Default()
	settings := jsonutil.Map(raw["settings"])
	defaultCalendarEnabled := true
	if _, present := settings["defaultCalendarEnabled"]; present {
		defaultCalendarEnabled = jsonutil.Truthy(settings["defaultCalendarEnabled"])
	}
	calendarOutputEnabled := true
	if _, present := settings["calendarOutputEnabled"]; present {
		calendarOutputEnabled = jsonutil.Truthy(settings["calendarOutputEnabled"])
	}
	out["settings"] = map[string]any{"defaultCalendarEnabled": defaultCalendarEnabled, "calendarOutputEnabled": calendarOutputEnabled, "dueSoonDays": clamp(jsonutil.Int(settings["dueSoonDays"], 30), 1, 90)}
	tasks, seen := []any{}, map[string]bool{}
	for _, rawTask := range jsonutil.List(raw["tasks"]) {
		task := Task(rawTask, now)
		if task == nil || seen[task["id"].(string)] {
			continue
		}
		seen[task["id"].(string)] = true
		tasks = append(tasks, task)
	}
	slices.SortStableFunc(tasks, func(left, right any) int {
		return strings.Compare(fmt.Sprint(jsonutil.Map(left)["nextDueOn"]), fmt.Sprint(jsonutil.Map(right)["nextDueOn"]))
	})
	if len(tasks) > MaxTasks {
		tasks = tasks[:MaxTasks]
	}
	out["tasks"] = tasks
	history := []any{}
	perTask := map[string]int{}
	for _, rawHistory := range jsonutil.List(raw["history"]) {
		row := HistoryRow(rawHistory)
		if row == nil || perTask[row["taskId"].(string)] >= TaskHistoryLimit {
			continue
		}
		perTask[row["taskId"].(string)]++
		history = append(history, row)
		if len(history) >= HistoryLimit {
			break
		}
	}
	slices.SortStableFunc(history, func(left, right any) int {
		return -strings.Compare(fmt.Sprint(jsonutil.Map(left)["createdAt"]), fmt.Sprint(jsonutil.Map(right)["createdAt"]))
	})
	out["history"] = history
	return out
}

func Find(payload map[string]any, id string) (int, map[string]any) {
	for i, raw := range jsonutil.List(payload["tasks"]) {
		task := jsonutil.Map(raw)
		if task["id"] == id {
			return i, task
		}
	}
	return -1, nil
}

func Status(task map[string]any, today string, dueSoon int) string {
	day := Date(task["nextDueOn"])
	if day == "" {
		return "later"
	}
	if day < today {
		return "overdue"
	}
	if day == today {
		return "today"
	}
	current, _ := DateTime(today)
	due, _ := DateTime(day)
	if !due.After(current.AddDate(0, 0, dueSoon)) {
		return "soon"
	}
	return "later"
}
func Summary(payload map[string]any, now time.Time) map[string]any {
	settings := jsonutil.Map(payload["settings"])
	today := now.In(time.Local).Format("2006-01-02")
	dueSoon := clamp(jsonutil.Int(settings["dueSoonDays"], 30), 1, 90)
	counts := map[string]int{"overdue": 0, "today": 0, "soon": 0, "later": 0}
	for _, raw := range jsonutil.List(payload["tasks"]) {
		task := jsonutil.Map(raw)
		if task["state"] != "active" {
			continue
		}
		counts[Status(task, today, dueSoon)]++
	}
	return map[string]any{"today": today, "dueSoonDays": dueSoon, "counts": counts}
}

func TaskPersonID(task map[string]any) string { return ID(task["responsiblePersonId"]) }
func TaskPersonSnapshot(task map[string]any) string {
	return Text(task["responsiblePersonNameSnapshot"], 64)
}
func DueChanged(left, right map[string]any) bool {
	return Date(left["nextDueOn"]) != Date(right["nextDueOn"])
}
