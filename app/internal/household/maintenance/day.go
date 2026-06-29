package maintenance

import (
	"fmt"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
	"slices"
	"strings"
	"time"
)

func CadenceText(task map[string]any) string {
	cadence := jsonutil.Map(task["cadence"])
	unit := Unit(cadence["unit"])
	return fmt.Sprintf("Every %d %s", Every(unit, cadence["every"]), unit)
}
func DayItem(task map[string]any, date string, now time.Time) map[string]any {
	return map[string]any{"id": ID(task["id"]), "title": Text(task["title"], 120), "note": Text(task["note"], 280), "dueOn": date, "cadence": CadenceText(task), "responsiblePersonId": TaskPersonID(task), "responsiblePersonNameSnapshot": TaskPersonSnapshot(task), "actionable": date <= now.In(time.Local).Format("2006-01-02")}
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
	return map[string]any{"date": date, "items": items, "count": len(items)}
}
