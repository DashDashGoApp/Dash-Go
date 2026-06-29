package routines

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

type DomainError struct {
	Status  int
	Message string
}

func (e *DomainError) Error() string { return e.Message }
func bad(message string) error       { return &DomainError{Status: 400, Message: message} }
func missing(message string) error   { return &DomainError{Status: 404, Message: message} }
func Status(err error) int {
	var de *DomainError
	if errors.As(err, &de) && de.Status > 0 {
		return de.Status
	}
	return 400
}

type MutationResult struct {
	Payload         map[string]any
	DayDate         string
	RebuildCalendar bool
	StateOnly       bool
}

func PeopleIDs(payload map[string]any) map[string]string {
	out := map[string]string{}
	for _, raw := range jsonutil.List(payload["people"]) {
		row := jsonutil.Map(raw)
		if row["state"] == "active" {
			out[ID(row["id"])] = Text(row["name"], 64)
		}
	}
	return out
}
func StepsFromBody(raw any, now time.Time) ([]any, error) {
	values := []any{}
	seen := map[string]bool{}
	for index, item := range jsonutil.List(raw) {
		row := jsonutil.Map(item)
		text := Text(row["text"], 140)
		if text == "" {
			text = Text(item, 140)
		}
		if text == "" {
			continue
		}
		id := ID(row["id"])
		if id == "" {
			id = fmt.Sprintf("rs_%d_%d", now.UnixNano(), index)
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		values = append(values, map[string]any{"id": id, "text": text, "position": (index + 1) * 10})
		if len(values) >= MaxSteps {
			break
		}
	}
	if len(values) == 0 {
		return nil, bad("add at least one checklist step")
	}
	return values, nil
}
func AssignmentsFromBody(raw any, people map[string]string, defaults, existing map[string]any, now time.Time) ([]any, error) {
	values := []any{}
	seen := map[string]bool{}
	oldByPerson := map[string]map[string]any{}
	if existing != nil {
		for _, rawOld := range jsonutil.List(existing["assignments"]) {
			old := jsonutil.Map(rawOld)
			oldByPerson[ID(old["personId"])] = old
		}
	}
	for index, item := range jsonutil.List(raw) {
		row := jsonutil.Map(item)
		pid := ID(row["personId"])
		name, active := people[pid]
		if pid == "" || !active || seen[pid] {
			continue
		}
		seen[pid] = true
		id := ID(row["id"])
		if id == "" && oldByPerson[pid] != nil {
			id = ID(oldByPerson[pid]["id"])
		}
		if id == "" {
			id = fmt.Sprintf("ra_%d_%d", now.UnixNano(), index)
		}
		calendarEnabled := BoolDefault(row["calendarEnabled"], BoolDefault(defaults["defaultCalendarEnabled"], true))
		values = append(values, map[string]any{"id": id, "personId": pid, "personNameSnapshot": name, "calendarEnabled": calendarEnabled, "schedule": Schedule(row["schedule"], now.In(time.Local).Format("2006-01-02"), now)})
	}
	if len(values) == 0 {
		return nil, bad("choose at least one active household person")
	}
	return values, nil
}
func ItemFromBody(body, payload, existing map[string]any, now time.Time) (map[string]any, error) {
	title := Text(body["title"], 120)
	if title == "" {
		return nil, bad("routine name is required")
	}
	if len([]rune(jsonutil.BodyString(body, "title"))) > 120 {
		return nil, bad("routine name must be 120 characters or fewer")
	}
	note := Text(body["note"], 280)
	if len([]rune(jsonutil.BodyString(body, "note"))) > 280 {
		return nil, bad("routine note must be 280 characters or fewer")
	}
	steps, err := StepsFromBody(body["steps"], now)
	if err != nil {
		return nil, err
	}
	assignments, err := AssignmentsFromBody(body["assignments"], PeopleIDs(payload), jsonutil.Map(payload["settings"]), existing, now)
	if err != nil {
		return nil, err
	}
	stamp := now.Format(time.RFC3339)
	row := map[string]any{"id": NewID("routine", now), "title": title, "note": note, "state": "active", "steps": steps, "assignments": assignments, "createdAt": stamp, "updatedAt": stamp, "archivedAt": ""}
	if existing != nil {
		for k, v := range existing {
			row[k] = v
		}
		row["title"], row["note"], row["steps"], row["assignments"], row["updatedAt"] = title, note, steps, assignments, stamp
	}
	return row, nil
}
func ApplySettings(payload, body map[string]any, now time.Time) MutationResult {
	next := Normalize(payload, now)
	settings := jsonutil.Map(next["settings"])
	if _, ok := body["calendarOutputEnabled"]; ok {
		settings["calendarOutputEnabled"] = jsonutil.Truthy(body["calendarOutputEnabled"])
	}
	if _, ok := body["defaultCalendarEnabled"]; ok {
		settings["defaultCalendarEnabled"] = jsonutil.Truthy(body["defaultCalendarEnabled"])
	}
	if _, ok := body["calendarHorizonDays"]; ok {
		settings["calendarHorizonDays"] = clamp(jsonutil.Int(body["calendarHorizonDays"], 56), 7, 90)
	}
	next["settings"] = settings
	return MutationResult{Payload: NextRevision(next, now), DayDate: Date(body["dayDate"]), RebuildCalendar: true}
}
func ApplyItem(payload, body map[string]any, now time.Time) (MutationResult, error) {
	next := Normalize(payload, now)
	op := strings.ToLower(Text(body["op"], 16))
	id := ID(body["id"])
	index, existing := Find(next, id)
	items := jsonutil.List(next["routines"])
	stamp := now.Format(time.RFC3339)
	switch op {
	case "create":
		row, err := ItemFromBody(body, next, nil, now)
		if err != nil {
			return MutationResult{}, err
		}
		items = append(items, row)
	case "update":
		if existing == nil {
			return MutationResult{}, missing("routine was not found")
		}
		row, err := ItemFromBody(body, next, existing, now)
		if err != nil {
			return MutationResult{}, err
		}
		items[index] = row
	case "archive", "restore":
		if existing == nil {
			return MutationResult{}, missing("routine was not found")
		}
		if op == "archive" {
			existing["state"], existing["archivedAt"] = "archived", stamp
		} else {
			existing["state"], existing["archivedAt"] = "active", ""
		}
		existing["updatedAt"] = stamp
		items[index] = existing
	case "delete":
		if existing == nil {
			return MutationResult{}, missing("routine was not found")
		}
		out := []any{}
		for _, raw := range items {
			if ID(jsonutil.Map(raw)["id"]) != id {
				out = append(out, raw)
			}
		}
		items = out
	default:
		return MutationResult{}, bad("unknown routine action")
	}
	next["routines"] = items
	return MutationResult{Payload: NextRevision(next, now), DayDate: Date(body["dayDate"]), RebuildCalendar: true}, nil
}
func StepIDs(routine map[string]any) []string {
	ids := []string{}
	for _, raw := range jsonutil.List(routine["steps"]) {
		if id := ID(jsonutil.Map(raw)["id"]); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}
func occurrenceAt(payload map[string]any, id string) (int, map[string]any) {
	for i, raw := range jsonutil.List(payload["occurrences"]) {
		row := jsonutil.Map(raw)
		if ID(row["id"]) == id {
			return i, row
		}
	}
	return -1, nil
}
func EnsureStoredOccurrence(payload map[string]any, routineID, assignmentID, date string, now time.Time) (map[string]any, error) {
	_, routine := Find(payload, routineID)
	if routine == nil {
		return nil, missing("routine is no longer available")
	}
	var assignment map[string]any
	for _, raw := range jsonutil.List(routine["assignments"]) {
		candidate := jsonutil.Map(raw)
		if ID(candidate["id"]) == assignmentID {
			assignment = candidate
			break
		}
	}
	if assignment == nil {
		return nil, missing("routine assignment is no longer available")
	}
	for _, raw := range jsonutil.List(payload["occurrences"]) {
		occ := jsonutil.Map(raw)
		if ID(occ["routineId"]) == routineID && ID(occ["assignmentId"]) == assignmentID && Date(occ["date"]) == date {
			return occ, nil
		}
	}
	schedule := jsonutil.Map(assignment["schedule"])
	if !DueOn(schedule, mustDate(date)) {
		return nil, bad("routine is not scheduled for that day")
	}
	return map[string]any{"id": NewID("ro", now), "routineId": routineID, "assignmentId": assignmentID, "personId": ID(assignment["personId"]), "personNameSnapshot": Text(assignment["personNameSnapshot"], 64), "date": date, "time": Clock(schedule["time"]), "allDay": BoolDefault(schedule["allDay"], true), "state": "active", "completedStepIds": []any{}, "completedAt": "", "skippedAt": ""}, nil
}
func mustDate(date string) time.Time {
	value, _ := time.ParseInLocation("2006-01-02", date, time.Local)
	return value
}
func ApplyStep(payload, occ map[string]any, stepID string, checked bool, now time.Time) (bool, error) {
	_, routine := Find(payload, ID(occ["routineId"]))
	if routine == nil {
		return false, missing("routine is no longer available")
	}
	valid := map[string]bool{}
	for _, id := range StepIDs(routine) {
		valid[id] = true
	}
	if !valid[stepID] {
		return false, bad("checklist step is no longer available")
	}
	completed := []any{}
	seen := map[string]bool{}
	for _, raw := range jsonutil.List(occ["completedStepIds"]) {
		id := ID(raw)
		if id != "" && valid[id] && id != stepID && !seen[id] {
			seen[id] = true
			completed = append(completed, id)
		}
	}
	if checked {
		completed = append(completed, stepID)
	}
	prior := Text(occ["state"], 16)
	occ["completedStepIds"] = completed
	if len(completed) == len(valid) && len(valid) > 0 {
		occ["state"], occ["completedAt"], occ["skippedAt"] = "completed", now.Format(time.RFC3339), ""
	} else {
		occ["state"], occ["completedAt"], occ["skippedAt"] = "active", "", ""
	}
	return prior != Text(occ["state"], 16), nil
}
func recordHistory(payload, occ map[string]any, prior string, now time.Time) {
	next := Text(occ["state"], 16)
	if prior == next {
		return
	}
	switch next {
	case "completed":
		AppendHistory(payload, "completed", occ, now)
	case "skipped":
		AppendHistory(payload, "skipped", occ, now)
	case "active":
		if prior == "completed" || prior == "skipped" {
			AppendHistory(payload, "reopened", occ, now)
		}
	}
}
func ApplyOccurrence(payload, body map[string]any, now time.Time) (MutationResult, error) {
	next := Normalize(payload, now)
	routineID, assignmentID, date := ID(body["routineId"]), ID(body["assignmentId"]), Date(body["date"])
	if routineID == "" || assignmentID == "" || date == "" {
		return MutationResult{}, bad("routine, assignment, and date are required")
	}
	if date > now.In(time.Local).Format("2006-01-02") {
		return MutationResult{}, bad("future routine sessions cannot be completed from the calendar")
	}
	occ, err := EnsureStoredOccurrence(next, routineID, assignmentID, date, now)
	if err != nil {
		return MutationResult{}, err
	}
	occurrences := jsonutil.List(next["occurrences"])
	index, _ := occurrenceAt(next, ID(occ["id"]))
	op := strings.ToLower(Text(body["op"], 16))
	stamp := now.Format(time.RFC3339)
	prior := Text(occ["state"], 16)
	rebuild := prior == "skipped"
	switch op {
	case "step":
		stepID := ID(body["stepId"])
		if stepID == "" {
			return MutationResult{}, bad("checklist step is required")
		}
		changed, err := ApplyStep(next, occ, stepID, jsonutil.Truthy(body["checked"]), now)
		if err != nil {
			return MutationResult{}, err
		}
		rebuild = rebuild || (changed && prior == "skipped")
	case "steps":
		changes := jsonutil.List(body["steps"])
		if len(changes) == 0 {
			return MutationResult{}, bad("at least one checklist step is required")
		}
		seen := map[string]bool{}
		for _, rawChange := range changes {
			change := jsonutil.Map(rawChange)
			stepID := ID(change["stepId"])
			if stepID == "" || seen[stepID] {
				continue
			}
			seen[stepID] = true
			changed, err := ApplyStep(next, occ, stepID, jsonutil.Truthy(change["checked"]), now)
			if err != nil {
				return MutationResult{}, err
			}
			rebuild = rebuild || (changed && prior == "skipped")
		}
		if len(seen) == 0 {
			return MutationResult{}, bad("at least one checklist step is required")
		}
	case "complete":
		_, routine := Find(next, routineID)
		if routine == nil {
			return MutationResult{}, missing("routine is no longer available")
		}
		completed := []any{}
		for _, stepID := range StepIDs(routine) {
			completed = append(completed, stepID)
		}
		occ["completedStepIds"], occ["state"], occ["completedAt"], occ["skippedAt"] = completed, "completed", stamp, ""
	case "skip":
		occ["state"], occ["skippedAt"], occ["completedAt"] = "skipped", stamp, ""
		rebuild = true
	default:
		return MutationResult{}, bad("unknown routine occurrence action")
	}
	recordHistory(next, occ, prior, now)
	if index >= 0 {
		occurrences[index] = occ
	} else {
		occurrences = append(occurrences, occ)
	}
	next["occurrences"] = occurrences
	return MutationResult{Payload: NextRevision(next, now), DayDate: date, RebuildCalendar: rebuild, StateOnly: !rebuild}, nil
}
