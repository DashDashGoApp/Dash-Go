package chores

import (
	"errors"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// ReconcilePeople applies the existing central People mutation to Chore Wheel
// state. Historical assignment snapshots remain untouched. Only current/today
// and future assigned work changes when a person is permanently removed,
// matching the pre-extraction behavior and existing People control contract.
func (s *Service) ReconcilePeople(payload map[string]any, activePeople []any, op, personID, targetID, targetName string) (map[string]any, error) {
	next := NormalizeAt(payload, s.Now())
	next["people"] = activePeople
	personID, targetID = ID(personID), ID(targetID)
	if op != "delete" {
		return NextRevision(next), nil
	}
	if targetID != "" && Text(targetName, 64) == "" {
		return nil, errors.New("reassignment person was not found")
	}
	for _, rawChore := range jsonutil.List(next["chores"]) {
		chore := jsonutil.Map(rawChore)
		eligible, seen := []any{}, map[string]bool{}
		for _, rawID := range jsonutil.List(chore["eligible"]) {
			current := ID(rawID)
			if current == personID {
				current = targetID
			}
			if current != "" && !seen[current] {
				seen[current] = true
				eligible = append(eligible, current)
			}
		}
		chore["eligible"] = eligible
	}
	assignments := []any{}
	for _, rawAssignment := range jsonutil.List(next["assignments"]) {
		assignment := jsonutil.Map(rawAssignment)
		if ID(assignment["personId"]) != personID || DateKey(assignment["date"]) < s.Today() || assignment["status"] != "assigned" {
			assignments = append(assignments, assignment)
			continue
		}
		if targetID != "" {
			assignment["personId"], assignment["personName"] = targetID, Text(targetName, 64)
			assignments = append(assignments, assignment)
		}
	}
	next["assignments"] = assignments
	return NextRevision(next), nil
}
