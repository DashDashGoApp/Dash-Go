package todo

import (
	"fmt"
	"strings"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// todo_people.go owns the small Dash-Go-only responsibility layer. Microsoft
// To Do has no native per-task household assignee, so this metadata remains in
// the local task cache and never creates a Graph pending operation.
func todoAssignmentCopy(value *todoTaskAssignment) *todoTaskAssignment {
	if value == nil || strings.TrimSpace(value.PersonID) == "" {
		return nil
	}
	copy := *value
	copy.PersonID = todoID(copy.PersonID)
	copy.PersonNameSnapshot = todoText(copy.PersonNameSnapshot, 64)
	if copy.PersonID == "" {
		return nil
	}
	return &copy
}

func (a *Service) todoTaskAssignmentName(task todoTask) string {
	assignment := todoAssignmentCopy(task.DashGoAssignment)
	if assignment == nil {
		return ""
	}
	if person, ok := a.householdPeopleAssignmentLookup(assignment.PersonID); ok {
		name := a.householdPersonAssignmentName(person)
		if name != "" {
			if person["state"] == "active" {
				return name
			}
			return "Former: " + name
		}
	}
	if assignment.PersonNameSnapshot != "" {
		return "Former: " + assignment.PersonNameSnapshot
	}
	return "Former household member"
}

func (a *Service) todoAssignmentForPerson(personID string) (*todoTaskAssignment, error) {
	personID = todoID(personID)
	if personID == "" {
		return nil, nil
	}
	person, ok := a.householdPeopleActiveAssignment(personID)
	if !ok {
		return nil, fmt.Errorf("choose an active household person or Anyone")
	}
	name := a.householdPersonAssignmentName(person)
	if name == "" {
		return nil, fmt.Errorf("household person is unavailable")
	}
	return &todoTaskAssignment{PersonID: personID, PersonNameSnapshot: name, AssignedAt: todoNowMillis()}, nil
}

func (a *Service) todoAssignTaskPerson(listID, taskID, personID string) (todoListCache, error) {
	assignment, err := a.todoAssignmentForPerson(personID)
	if err != nil {
		return todoListCache{}, err
	}
	unlock := a.todoListLock(listID)
	defer unlock()
	cache := a.readTodoListCache(listID)
	for i := range cache.Tasks {
		if cache.Tasks[i].ID != taskID {
			continue
		}
		cache.Tasks[i].DashGoAssignment = assignment
		cache.Tasks[i].ETag = todoNowMillis()
		if err := a.writeTodoListCache(cache); err != nil {
			return cache, err
		}
		return cache, nil
	}
	return cache, fmt.Errorf("task is no longer available")
}

func (a *Service) todoHouseholdPeople() []any {
	return jsonutil.List(a.householdPeoplePayload()["people"])
}
