package maintenance

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func CadenceText(task map[string]any) string {
	cadence := jsonutil.Map(task["cadence"])
	unit := Unit(cadence["unit"])
	return fmt.Sprintf("Every %d %s", Every(unit, cadence["every"]), unit)
}
func DayItem(task map[string]any, date string, now time.Time) map[string]any {
	return map[string]any{"id": ID(task["id"]), "title": Text(task["title"], 120), "note": Text(task["note"], 280), "dueOn": date, "cadence": CadenceText(task), "responsiblePersonId": TaskPersonID(task), "responsiblePersonNameSnapshot": TaskPersonSnapshot(task), "actionable": date <= now.In(time.Local).Format("2006-01-02")}
}

func completedItemsForDay(payload map[string]any, date string, now time.Time) []any {
	undone, seenTasks, items := map[string]bool{}, map[string]bool{}, []any{}
	for _, raw := range jsonutil.List(payload["history"]) {
		row := jsonutil.Map(raw)
		if row["action"] == "undo" && ID(row["undoOf"]) != "" {
			undone[ID(row["undoOf"])] = true
		}
	}
	for _, raw := range jsonutil.List(payload["history"]) {
		history := jsonutil.Map(raw)
		if history["action"] != "completed" || Date(history["priorDueOn"]) != date || undone[ID(history["id"])] {
			continue
		}
		taskID := ID(history["taskId"])
		if taskID == "" || seenTasks[taskID] {
			continue
		}
		_, task := Find(payload, taskID)
		if task == nil {
			continue
		}
		seenTasks[taskID] = true
		item := DayItem(task, date, now)
		item["actionable"] = CanUndoCompletion(task, history)
		item["status"] = "completed"
		item["completedOn"] = Date(history["occurredOn"])
		item["nextDueOn"] = Date(history["nextDueOn"])
		item["completionId"] = ID(history["id"])
		item["undoAvailable"] = jsonutil.Truthy(item["actionable"])
		if !jsonutil.Truthy(item["undoAvailable"]) {
			item["correctionMessage"] = "Completion was followed by a later change. Open Maintenance to review the task."
		}
		items = append(items, item)
	}
	slices.SortStableFunc(items, func(l, r any) int {
		lr, rr := jsonutil.Map(l), jsonutil.Map(r)
		return strings.Compare(fmt.Sprint(lr["title"], lr["id"]), fmt.Sprint(rr["title"], rr["id"]))
	})
	return items
}

func DayResponse(payload map[string]any, date string, now time.Time) map[string]any {
	items := []any{}
	for _, raw := range jsonutil.List(payload["tasks"]) {
		task := jsonutil.Map(raw)
		if task["state"] != "active" || Date(task["nextDueOn"]) != date {
			continue
		}
		items = append(items, DayItem(task, date, now))
	}
	slices.SortStableFunc(items, func(l, r any) int {
		lr, rr := jsonutil.Map(l), jsonutil.Map(r)
		return strings.Compare(fmt.Sprint(lr["title"], lr["id"]), fmt.Sprint(rr["title"], rr["id"]))
	})
	completed := completedItemsForDay(payload, date, now)
	return map[string]any{"date": date, "items": items, "completedItems": completed, "count": len(items), "completedCount": len(completed)}
}
