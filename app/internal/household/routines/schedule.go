package routines

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func DateTime(date, clock string, allDay bool) (time.Time, bool) {
	base, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		return time.Time{}, false
	}
	if allDay || clock == "" {
		return base, true
	}
	parsed, err := time.Parse("15:04", clock)
	if err != nil {
		return base, true
	}
	return time.Date(base.Year(), base.Month(), base.Day(), parsed.Hour(), parsed.Minute(), 0, 0, time.Local), true
}
func weekdayCode(day time.Time) string {
	return []string{"SU", "MO", "TU", "WE", "TH", "FR", "SA"}[int(day.In(time.Local).Weekday())]
}
func civilDayIndex(day time.Time) int {
	local := day.In(time.Local)
	return int(time.Date(local.Year(), local.Month(), local.Day(), 12, 0, 0, 0, time.UTC).Unix() / 86400)
}
func dayDiff(from, to time.Time) int { return civilDayIndex(to) - civilDayIndex(from) }
func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 12, 0, 0, 0, time.Local).Day()
}
func scheduledDay(year int, month time.Month, wanted int) int {
	return min(clamp(wanted, 1, 31), daysInMonth(year, month))
}
func monthsDiff(start, day time.Time) int {
	s := start.In(time.Local)
	d := day.In(time.Local)
	return (d.Year()-s.Year())*12 + int(d.Month()) - int(s.Month())
}
func DueOn(schedule map[string]any, day time.Time) bool {
	localDay := day.In(time.Local)
	date := localDay.Format("2006-01-02")
	start := Date(schedule["startOn"])
	end := Date(schedule["endOn"])
	if start == "" || date < start || (end != "" && date > end) {
		return false
	}
	startDay, err := time.ParseInLocation("2006-01-02", start, time.Local)
	if err != nil {
		return false
	}
	every := clamp(jsonutil.Int(schedule["every"], 1), 1, 365)
	switch Text(schedule["kind"], 16) {
	case "days":
		return dayDiff(startDay, localDay)%every == 0
	case "weekdays":
		code := weekdayCode(localDay)
		if code == "SA" || code == "SU" {
			return false
		}
		weeks := dayDiff(startDay, localDay) / 7
		return weeks >= 0 && weeks%every == 0
	case "weekly":
		weeks := dayDiff(startDay, localDay) / 7
		if weeks < 0 || weeks%every != 0 {
			return false
		}
		code := weekdayCode(localDay)
		for _, raw := range jsonutil.List(schedule["weekdays"]) {
			if fmt.Sprint(raw) == code {
				return true
			}
		}
		return false
	case "monthly":
		months := monthsDiff(startDay, localDay)
		return months >= 0 && months%every == 0 && localDay.Day() == scheduledDay(localDay.Year(), localDay.Month(), jsonutil.Int(schedule["day"], 1))
	case "yearly":
		years := localDay.Year() - startDay.Year()
		month := time.Month(clamp(jsonutil.Int(schedule["month"], 1), 1, 12))
		return years >= 0 && years%every == 0 && localDay.Month() == month && localDay.Day() == scheduledDay(localDay.Year(), month, jsonutil.Int(schedule["day"], 1))
	case "once":
		return date == start
	default:
		return false
	}
}
func PersonActive(payload map[string]any, id string) bool {
	for _, raw := range jsonutil.List(payload["people"]) {
		person := jsonutil.Map(raw)
		if ID(person["id"]) == id {
			return person["state"] == "active"
		}
	}
	return false
}
func OccurrencesForDay(payload map[string]any, date string, now time.Time) []map[string]any {
	wanted, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		return nil
	}
	existing := map[string]map[string]any{}
	for _, raw := range jsonutil.List(payload["occurrences"]) {
		occ := jsonutil.Map(raw)
		existing[ID(occ["routineId"])+"/"+ID(occ["assignmentId"])+"/"+date] = occ
	}
	out := []map[string]any{}
	for _, rawRoutine := range jsonutil.List(payload["routines"]) {
		routine := jsonutil.Map(rawRoutine)
		if routine["state"] != "active" {
			continue
		}
		for _, rawAssignment := range jsonutil.List(routine["assignments"]) {
			assignment := jsonutil.Map(rawAssignment)
			personID := ID(assignment["personId"])
			if !PersonActive(payload, personID) {
				continue
			}
			schedule := jsonutil.Map(assignment["schedule"])
			if !DueOn(schedule, wanted) {
				continue
			}
			key := ID(routine["id"]) + "/" + ID(assignment["id"]) + "/" + date
			occ := existing[key]
			if occ == nil {
				occ = map[string]any{"id": "generated-" + key, "routineId": routine["id"], "assignmentId": assignment["id"], "personId": assignment["personId"], "personNameSnapshot": Text(assignment["personNameSnapshot"], 64), "date": date, "time": Clock(schedule["time"]), "allDay": BoolDefault(schedule["allDay"], true), "state": "active", "completedStepIds": []any{}}
			}
			copy := make(map[string]any, len(occ))
			maps.Copy(copy, occ)
			copy["routineTitle"], copy["routineNote"], copy["calendarEnabled"], copy["actionable"], copy["steps"], copy["personName"] = Text(routine["title"], 120), Text(routine["note"], 280), BoolDefault(assignment["calendarEnabled"], true), date <= now.In(time.Local).Format("2006-01-02") && Text(copy["state"], 16) != "skipped", jsonutil.List(routine["steps"]), Text(copy["personNameSnapshot"], 64)
			if copy["personName"] == "" {
				copy["personName"] = Text(assignment["personNameSnapshot"], 64)
			}
			if copy["personName"] == "" {
				copy["personName"] = PersonName(payload, personID)
			}
			out = append(out, copy)
		}
	}
	slices.SortStableFunc(out, func(l, r map[string]any) int {
		return strings.Compare(fmt.Sprint(l["personName"], l["time"], l["routineTitle"]), fmt.Sprint(r["personName"], r["time"], r["routineTitle"]))
	})
	return out
}
