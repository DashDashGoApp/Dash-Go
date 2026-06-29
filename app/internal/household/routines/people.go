package routines

import (
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
	"time"
)

// ProjectRoster keeps the canonical People roster as the source for current
// Routine assignment names. It is pure so core can hold the outer People lock
// without the Routines service ever calling back into People.
func ProjectRoster(payload, roster map[string]any, now time.Time) map[string]any {
	next := Normalize(payload, now)
	if len(jsonutil.List(roster["people"])) > 0 {
		next["people"] = jsonutil.List(roster["people"])
	}
	return Normalize(next, now)
}
func targetName(roster map[string]any, targetID string) string {
	for _, raw := range jsonutil.List(roster["people"]) {
		person := jsonutil.Map(raw)
		if ID(person["id"]) == ID(targetID) {
			return Text(person["name"], 64)
		}
	}
	return ""
}
func ReconcilePeople(payload, roster map[string]any, op, personID, targetID string, now time.Time) (map[string]any, bool) {
	// Start from the pre-change normalized document. For a deletion, ProjectRoster
	// would otherwise discard the removed person's assignments before this method
	// has a chance to apply the existing reassign/unassign correction rule.
	next := Normalize(payload, now)
	if op != "delete" {
		return ProjectRoster(next, roster, now), false
	}
	changed := false
	targetID = ID(targetID)
	targetName := targetName(roster, targetID)
	items := jsonutil.List(next["routines"])
	for _, rawRoutine := range items {
		routine := jsonutil.Map(rawRoutine)
		assignments := jsonutil.List(routine["assignments"])
		targetPresent := false
		for _, rawAssignment := range assignments {
			if ID(jsonutil.Map(rawAssignment)["personId"]) == targetID && targetID != "" {
				targetPresent = true
			}
		}
		out := []any{}
		for _, rawAssignment := range assignments {
			assignment := jsonutil.Map(rawAssignment)
			if ID(assignment["personId"]) != ID(personID) {
				out = append(out, assignment)
				continue
			}
			changed = true
			if targetID != "" && !targetPresent {
				assignment["personId"], assignment["personNameSnapshot"] = targetID, targetName
				out = append(out, assignment)
				targetPresent = true
			}
		}
		routine["assignments"] = out
	}
	occurrences := []any{}
	for _, raw := range jsonutil.List(next["occurrences"]) {
		occ := jsonutil.Map(raw)
		if ID(occ["personId"]) == ID(personID) && Date(occ["date"]) >= now.In(time.Local).Format("2006-01-02") {
			changed = true
			continue
		}
		occurrences = append(occurrences, occ)
	}
	next["occurrences"] = occurrences
	if changed {
		next = NextRevision(next, now)
	}
	return ProjectRoster(next, roster, now), changed
}

// ApplyLegacyPeopleAction preserves the old Routines-specific people endpoint
// until browser callers have all moved to Dashboard Control People. The method
// returns a changed canonical-roster candidate; core remains its only writer.
func ApplyLegacyPeopleAction(payload, roster, body map[string]any, now time.Time) (map[string]any, map[string]any, error) {
	next := ProjectRoster(payload, roster, now)
	outRoster := make(map[string]any, len(roster))
	for k, v := range roster {
		outRoster[k] = v
	}
	people := jsonutil.List(outRoster["people"])
	op := Text(body["op"], 16)
	id := ID(body["id"])
	index, person := findRosterPerson(people, id)
	stamp := now.Format(time.RFC3339)
	switch op {
	case "add":
		name := Text(body["name"], 64)
		if name == "" {
			return nil, nil, bad("person name is required")
		}
		if len(people) >= MaxPeople {
			return nil, nil, bad("Routines supports up to 20 people")
		}
		people = append(people, map[string]any{"id": NewID("person", now), "name": name, "state": "active", "createdAt": stamp, "updatedAt": stamp, "archivedAt": ""})
	case "update":
		if person == nil {
			return nil, nil, missing("person was not found")
		}
		name := Text(body["name"], 64)
		if name == "" {
			return nil, nil, bad("person name is required")
		}
		person["name"], person["updatedAt"] = name, stamp
		people[index] = person
	case "archive", "restore":
		if person == nil {
			return nil, nil, missing("person was not found")
		}
		reassign := ID(body["reassignTo"])
		if op == "archive" && reassign != "" && reassign != id {
			if !canReassign(people, reassign) {
				return nil, nil, bad("reassignment person was not found")
			}
			reassignFuture(next, id, reassign, now)
		}
		if op == "archive" {
			person["state"], person["archivedAt"] = "archived", stamp
		} else {
			person["state"], person["archivedAt"] = "active", ""
		}
		person["updatedAt"] = stamp
		people[index] = person
	case "delete":
		if person == nil {
			return nil, nil, missing("person was not found")
		}
		out := []any{}
		for _, raw := range people {
			if ID(jsonutil.Map(raw)["id"]) != id {
				out = append(out, raw)
			}
		}
		people = out
		deleteReferences(next, id)
	default:
		return nil, nil, bad("unknown people action")
	}
	outRoster["people"] = people
	outRoster["revision"] = jsonutil.Int(roster["revision"], 0) + 1
	next = ProjectRoster(next, outRoster, now)
	return NextRevision(next, now), outRoster, nil
}
func findRosterPerson(people []any, id string) (int, map[string]any) {
	for i, raw := range people {
		person := jsonutil.Map(raw)
		if ID(person["id"]) == id {
			return i, person
		}
	}
	return -1, nil
}
func canReassign(people []any, id string) bool {
	for _, raw := range people {
		candidate := jsonutil.Map(raw)
		if ID(candidate["id"]) == ID(id) && candidate["state"] == "active" {
			return true
		}
	}
	return false
}
func reassignFuture(payload map[string]any, fromID, targetID string, now time.Time) {
	for _, rawRoutine := range jsonutil.List(payload["routines"]) {
		routine := jsonutil.Map(rawRoutine)
		targetAlready := false
		for _, rawAssignment := range jsonutil.List(routine["assignments"]) {
			if ID(jsonutil.Map(rawAssignment)["personId"]) == targetID {
				targetAlready = true
				break
			}
		}
		assignments := []any{}
		for _, rawAssignment := range jsonutil.List(routine["assignments"]) {
			assignment := jsonutil.Map(rawAssignment)
			if ID(assignment["personId"]) == fromID {
				if targetAlready {
					continue
				}
				assignment["personId"] = targetID
				targetAlready = true
			}
			assignments = append(assignments, assignment)
		}
		routine["assignments"] = assignments
	}
	filtered := []any{}
	for _, raw := range jsonutil.List(payload["occurrences"]) {
		occ := jsonutil.Map(raw)
		if ID(occ["personId"]) == fromID && Date(occ["date"]) >= now.In(time.Local).Format("2006-01-02") {
			continue
		}
		filtered = append(filtered, occ)
	}
	payload["occurrences"] = filtered
}
func deleteReferences(payload map[string]any, id string) {
	for _, rawRoutine := range jsonutil.List(payload["routines"]) {
		routine := jsonutil.Map(rawRoutine)
		assignments := []any{}
		for _, rawAssignment := range jsonutil.List(routine["assignments"]) {
			if ID(jsonutil.Map(rawAssignment)["personId"]) != id {
				assignments = append(assignments, rawAssignment)
			}
		}
		routine["assignments"] = assignments
	}
	filtered := []any{}
	for _, raw := range jsonutil.List(payload["occurrences"]) {
		if ID(jsonutil.Map(raw)["personId"]) != id {
			filtered = append(filtered, raw)
		}
	}
	payload["occurrences"] = filtered
}
