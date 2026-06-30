package maintenance

import (
	"fmt"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

type PersonResolver func(id string) (name string, active bool)
type MutationResult struct {
	Payload map[string]any
	Extra   map[string]any
}

func taskID(now time.Time) string    { return fmt.Sprintf("mt_%d", now.UnixNano()) }
func historyID(now time.Time) string { return fmt.Sprintf("mh_%d", now.UnixNano()) }

func addHistory(payload, task map[string]any, action, occurred, prior, next, priorLastCompleted, undoOf string, now time.Time) map[string]any {
	entry := map[string]any{
		"id":                            historyID(now),
		"taskId":                        task["id"],
		"action":                        action,
		"occurredOn":                    occurred,
		"priorDueOn":                    prior,
		"nextDueOn":                     next,
		"priorLastCompletedOn":          priorLastCompleted,
		"undoOf":                        undoOf,
		"createdAt":                     now.Format(time.RFC3339),
		"responsiblePersonId":           TaskPersonID(task),
		"responsiblePersonNameSnapshot": TaskPersonSnapshot(task),
	}
	history := jsonutil.List(payload["history"])
	payload["history"] = append([]any{entry}, history...)
	return entry
}

// AddHistory retains the original maintenance-history contract for ordinary
// edits. Completion/undo records below add the small amount of context needed
// to safely restore a mistaken completion.
func AddHistory(payload, task map[string]any, action, occurred, prior, next string, now time.Time) {
	addHistory(payload, task, action, occurred, prior, next, "", "", now)
}

func addCompletionHistory(payload, task map[string]any, completed, priorDue, nextDue, priorLastCompleted string, now time.Time) map[string]any {
	return addHistory(payload, task, "completed", completed, priorDue, nextDue, priorLastCompleted, "", now)
}

func historyByID(payload map[string]any, id string) map[string]any {
	for _, raw := range jsonutil.List(payload["history"]) {
		row := jsonutil.Map(raw)
		if ID(row["id"]) == id {
			return row
		}
	}
	return nil
}

// CanUndoCompletion is intentionally strict. A completion may be reverted only
// while it is still the task's latest completion state and the task still has
// the exact due/last-completed values produced by that action.
func CanUndoCompletion(task, completion map[string]any) bool {
	if task == nil || completion == nil || task["state"] != "active" || completion["action"] != "completed" {
		return false
	}
	if ID(task["lastCompletionHistoryId"]) == "" || ID(task["lastCompletionHistoryId"]) != ID(completion["id"]) {
		return false
	}
	if Date(task["lastCompletedOn"]) != Date(completion["occurredOn"]) || Date(task["nextDueOn"]) != Date(completion["nextDueOn"]) {
		return false
	}
	return Date(completion["priorDueOn"]) != ""
}

func personFields(body, existing map[string]any, resolver PersonResolver) (string, string, error) {
	if _, present := body["responsiblePersonId"]; !present && existing != nil {
		return ID(existing["responsiblePersonId"]), Text(existing["responsiblePersonNameSnapshot"], 64), nil
	}
	id := ID(body["responsiblePersonId"])
	if id == "" {
		return "", "", nil
	}
	if name, active := resolver(id); active && name != "" {
		return id, Text(name, 64), nil
	}
	if existing != nil && id == ID(existing["responsiblePersonId"]) {
		return id, Text(existing["responsiblePersonNameSnapshot"], 64), nil
	}
	return "", "", fmt.Errorf("choose an active household person or Anyone")
}
func TaskFromBody(body, defaults, existing map[string]any, now time.Time, resolver PersonResolver) (map[string]any, error) {
	rawTitle := String(body["title"])
	if rawTitle == "" {
		return nil, fmt.Errorf("maintenance task name is required")
	}
	if len([]rune(rawTitle)) > 120 {
		return nil, fmt.Errorf("maintenance task name must be 120 characters or fewer")
	}
	rawNote := String(body["note"])
	if len([]rune(rawNote)) > 280 {
		return nil, fmt.Errorf("maintenance note must be 280 characters or fewer")
	}
	cadence := jsonutil.Map(body["cadence"])
	unit := Unit(cadence["unit"])
	every := Every(unit, cadence["every"])
	next := Date(body["nextDueOn"])
	if next == "" {
		return nil, fmt.Errorf("next due date must be YYYY-MM-DD")
	}
	personID, personName, err := personFields(body, existing, resolver)
	if err != nil {
		return nil, err
	}
	stamp := now.Format(time.RFC3339)
	calendarEnabled := jsonutil.Truthy(defaults["defaultCalendarEnabled"])
	row := map[string]any{"id": taskID(now), "title": Text(rawTitle, 120), "note": Text(rawNote, 280), "state": "active", "cadence": map[string]any{"unit": unit, "every": every}, "lastCompletedOn": "", "lastCompletionHistoryId": "", "nextDueOn": next, "calendarEnabled": calendarEnabled, "createdAt": stamp, "updatedAt": stamp, "archivedAt": "", "responsiblePersonId": personID, "responsiblePersonNameSnapshot": personName}
	if existing != nil {
		for k, v := range existing {
			row[k] = v
		}
		row["title"], row["note"], row["cadence"], row["nextDueOn"] = Text(rawTitle, 120), Text(rawNote, 280), map[string]any{"unit": unit, "every": every}, next
		row["calendarEnabled"], row["updatedAt"] = jsonutil.Truthy(body["calendarEnabled"]), stamp
		row["responsiblePersonId"], row["responsiblePersonNameSnapshot"] = personID, personName
	} else if body["calendarEnabled"] != nil {
		row["calendarEnabled"] = jsonutil.Truthy(body["calendarEnabled"])
	}
	return row, nil
}

func undoCompletion(payload map[string]any, taskIDValue, completionID string, now time.Time) (MutationResult, error) {
	idx, task := Find(payload, ID(taskIDValue))
	if idx < 0 || task == nil {
		return MutationResult{}, fmt.Errorf("unknown maintenance task")
	}
	completion := historyByID(payload, ID(completionID))
	if completion == nil || ID(completion["taskId"]) != ID(task["id"]) || completion["action"] != "completed" {
		return MutationResult{}, fmt.Errorf("completion history was not found")
	}
	if !CanUndoCompletion(task, completion) {
		return MutationResult{}, fmt.Errorf("completion was followed by a later change; open Maintenance to review the task")
	}
	priorLastCompleted := Date(task["lastCompletedOn"])
	priorDue := Date(task["nextDueOn"])
	restoredDue := Date(completion["priorDueOn"])
	task["lastCompletedOn"], task["lastCompletionHistoryId"], task["nextDueOn"], task["updatedAt"] = Date(completion["priorLastCompletedOn"]), "", restoredDue, now.Format(time.RFC3339)
	tasks := jsonutil.List(payload["tasks"])
	tasks[idx] = task
	payload["tasks"] = tasks
	addHistory(payload, task, "undo", now.In(time.Local).Format("2006-01-02"), priorDue, restoredDue, priorLastCompleted, ID(completion["id"]), now)
	return MutationResult{Payload: Normalize(payload, now), Extra: map[string]any{"undoneCompletionId": ID(completion["id"])}}, nil
}

func Apply(payload map[string]any, path string, body map[string]any, now time.Time, resolver PersonResolver) (MutationResult, error) {
	payload = Normalize(payload, now)
	tasks := jsonutil.List(payload["tasks"])
	settings := jsonutil.Map(payload["settings"])
	today := now.In(time.Local).Format("2006-01-02")
	save := func(extra map[string]any) (MutationResult, error) {
		payload["tasks"], payload["settings"] = tasks, settings
		return MutationResult{Payload: Normalize(payload, now), Extra: extra}, nil
	}
	switch path {
	case "/api/maintenance/settings":
		settings["defaultCalendarEnabled"] = jsonutil.Truthy(body["defaultCalendarEnabled"])
		settings["dueSoonDays"] = clamp(jsonutil.Int(body["dueSoonDays"], 30), 1, 90)
		return save(nil)
	case "/api/maintenance/tasks/add":
		if len(tasks) >= MaxTasks {
			return MutationResult{}, fmt.Errorf("maintenance task limit reached")
		}
		task, err := TaskFromBody(body, settings, nil, now, resolver)
		if err != nil {
			return MutationResult{}, err
		}
		tasks = append(tasks, task)
		return save(nil)
	case "/api/maintenance/tasks/update":
		idx, old := Find(payload, ID(body["id"]))
		if idx < 0 {
			return MutationResult{}, fmt.Errorf("unknown maintenance task")
		}
		task, err := TaskFromBody(body, settings, old, now, resolver)
		if err != nil {
			return MutationResult{}, err
		}
		// Any deliberate task edit becomes the newer source of truth and means
		// an older calendar-popup completion must not restore stale values.
		task["lastCompletionHistoryId"] = ""
		tasks[idx] = task
		if DueChanged(old, task) {
			AddHistory(payload, task, "rescheduled", today, Date(old["nextDueOn"]), Date(task["nextDueOn"]), now)
		}
		return save(nil)
	case "/api/maintenance/tasks/complete":
		idx, task := Find(payload, ID(body["id"]))
		if idx < 0 || task["state"] != "active" {
			return MutationResult{}, fmt.Errorf("unknown active maintenance task")
		}
		completed := Date(body["completedOn"])
		if completed == "" {
			return MutationResult{}, fmt.Errorf("completion date must be YYYY-MM-DD")
		}
		prior := Date(task["nextDueOn"])
		if prior > today {
			return MutationResult{}, fmt.Errorf("future maintenance tasks cannot be completed yet")
		}
		priorLastCompleted := Date(task["lastCompletedOn"])
		cadence := jsonutil.Map(task["cadence"])
		task["lastCompletedOn"] = completed
		task["nextDueOn"] = NextDue(completed, Unit(cadence["unit"]), Every(Unit(cadence["unit"]), cadence["every"]))
		task["updatedAt"] = now.Format(time.RFC3339)
		completion := addCompletionHistory(payload, task, completed, prior, Date(task["nextDueOn"]), priorLastCompleted, now)
		task["lastCompletionHistoryId"] = ID(completion["id"])
		tasks[idx] = task
		completedTask := DayItem(task, prior, now)
		completedTask["actionable"], completedTask["status"], completedTask["completedOn"], completedTask["nextDueOn"], completedTask["completionId"], completedTask["undoAvailable"] = true, "completed", completed, Date(task["nextDueOn"]), ID(completion["id"]), true
		return save(map[string]any{"completedTask": completedTask})
	case "/api/maintenance/tasks/undo-complete":
		payload["tasks"], payload["settings"] = tasks, settings
		return undoCompletion(payload, ID(body["id"]), ID(body["completionId"]), now)
	case "/api/maintenance/tasks/reschedule":
		idx, task := Find(payload, ID(body["id"]))
		if idx < 0 {
			return MutationResult{}, fmt.Errorf("unknown maintenance task")
		}
		next := Date(body["nextDueOn"])
		if next == "" {
			return MutationResult{}, fmt.Errorf("next due date must be YYYY-MM-DD")
		}
		prior := Date(task["nextDueOn"])
		task["nextDueOn"], task["lastCompletionHistoryId"], task["updatedAt"] = next, "", now.Format(time.RFC3339)
		tasks[idx] = task
		AddHistory(payload, task, "rescheduled", today, prior, next, now)
		return save(nil)
	case "/api/maintenance/tasks/archive":
		idx, task := Find(payload, ID(body["id"]))
		if idx < 0 {
			return MutationResult{}, fmt.Errorf("unknown maintenance task")
		}
		task["state"], task["archivedAt"], task["lastCompletionHistoryId"], task["updatedAt"] = "archived", now.Format(time.RFC3339), "", now.Format(time.RFC3339)
		tasks[idx] = task
		AddHistory(payload, task, "archived", today, Date(task["nextDueOn"]), "", now)
		return save(nil)
	case "/api/maintenance/tasks/restore":
		idx, task := Find(payload, ID(body["id"]))
		if idx < 0 {
			return MutationResult{}, fmt.Errorf("unknown maintenance task")
		}
		next := Date(body["nextDueOn"])
		if next == "" {
			return MutationResult{}, fmt.Errorf("next due date must be YYYY-MM-DD")
		}
		task["state"], task["archivedAt"], task["nextDueOn"], task["lastCompletionHistoryId"], task["updatedAt"] = "active", "", next, "", now.Format(time.RFC3339)
		tasks[idx] = task
		AddHistory(payload, task, "restored", today, "", next, now)
		return save(nil)
	case "/api/maintenance/tasks/delete":
		idx, task := Find(payload, ID(body["id"]))
		if idx < 0 {
			return MutationResult{}, fmt.Errorf("unknown maintenance task")
		}
		AddHistory(payload, task, "deleted", today, Date(task["nextDueOn"]), "", now)
		tasks = append(tasks[:idx], tasks[idx+1:]...)
		return save(nil)
	default:
		return MutationResult{}, fmt.Errorf("unknown maintenance endpoint")
	}
}

func ReconcilePeople(payload map[string]any, op, personID, targetID, targetName string, now time.Time) (map[string]any, bool) {
	if op != "delete" {
		return Normalize(payload, now), false
	}
	next := Normalize(payload, now)
	changed := false
	tasks := jsonutil.List(next["tasks"])
	for i, raw := range tasks {
		task := jsonutil.Map(raw)
		if task["state"] != "active" || TaskPersonID(task) != ID(personID) {
			continue
		}
		task["responsiblePersonId"], task["responsiblePersonNameSnapshot"], task["lastCompletionHistoryId"] = ID(targetID), Text(targetName, 64), ""
		tasks[i] = task
		changed = true
	}
	next["tasks"] = tasks
	return Normalize(next, now), changed
}
