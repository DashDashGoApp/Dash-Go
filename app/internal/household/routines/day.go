package routines

import (
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
	"slices"
	"strings"
	"time"
)

func DayResponse(payload map[string]any, date string, now time.Time) map[string]any {
	items := []any{}
	for _, item := range OccurrencesForDay(payload, date, now) {
		copy := make(map[string]any, len(item))
		for k, v := range item {
			copy[k] = v
		}
		items = append(items, copy)
	}
	slices.SortStableFunc(items, func(l, r any) int {
		lr, rr := jsonutil.Map(l), jsonutil.Map(r)
		return strings.Compare(Text(lr["personName"], 64)+"|"+Clock(lr["time"])+"|"+Text(lr["routineTitle"], 120), Text(rr["personName"], 64)+"|"+Clock(rr["time"])+"|"+Text(rr["routineTitle"], 120))
	})
	return map[string]any{"date": date, "items": items, "count": len(items)}
}
