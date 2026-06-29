package chores

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

var (
	ErrAssignmentAndDate = errors.New("assignment and date are required")
	ErrFutureComplete    = errors.New("future chores cannot be completed from the calendar")
	ErrAssignmentMissing = errors.New("chore assignment was not found for that day")
	ErrAssignmentStatus  = errors.New("only an assigned chore can be completed from the calendar")
)

func DayResponse(payload map[string]any, date string, now time.Time) map[string]any {
	items := []any{}
	today := LocalDateKey(now)
	for _, raw := range jsonutil.List(payload["assignments"]) {
		row := jsonutil.Map(raw)
		if DateKey(row["date"]) != date {
			continue
		}
		status := Text(row["status"], 16)
		if status == "" {
			status = "assigned"
		}
		items = append(items, map[string]any{
			"assignmentId": ID(row["id"]), "date": date,
			"choreName": Text(row["choreName"], 96), "personName": Text(row["personName"], 64),
			"status": status, "actionable": status == "assigned" && date <= today,
		})
	}
	slices.SortStableFunc(items, func(left, right any) int {
		leftRow, rightRow := jsonutil.Map(left), jsonutil.Map(right)
		return strings.Compare(fmt.Sprint(leftRow["choreName"], leftRow["personName"], leftRow["assignmentId"]), fmt.Sprint(rightRow["choreName"], rightRow["personName"], rightRow["assignmentId"]))
	})
	completed := 0
	for _, raw := range items {
		if jsonutil.Map(raw)["status"] == "completed" {
			completed++
		}
	}
	return map[string]any{"date": date, "items": items, "count": len(items), "completed": completed}
}

func (s *Service) DayResponse(payload map[string]any, date string) map[string]any {
	return DayResponse(payload, date, s.Now())
}

func (s *Service) CompleteAssignment(payload map[string]any, assignmentID, date string) (map[string]any, bool, error) {
	assignmentID, date = ID(assignmentID), DateKey(date)
	if assignmentID == "" || date == "" {
		return nil, false, ErrAssignmentAndDate
	}
	if date > s.Today() {
		return nil, false, ErrFutureComplete
	}
	next := NormalizeAt(payload, s.Now())
	assignments := jsonutil.List(next["assignments"])
	index := -1
	var assignment map[string]any
	for i, raw := range assignments {
		candidate := jsonutil.Map(raw)
		if ID(candidate["id"]) == assignmentID {
			index, assignment = i, candidate
			break
		}
	}
	if index < 0 || assignment == nil || DateKey(assignment["date"]) != date {
		return nil, false, ErrAssignmentMissing
	}
	status := Text(assignment["status"], 16)
	if status == "completed" {
		return next, false, nil
	}
	if status != "assigned" {
		return nil, false, ErrAssignmentStatus
	}
	assignment["status"] = "completed"
	assignments[index] = assignment
	next["assignments"] = assignments
	return NextRevision(next), true, nil
}
