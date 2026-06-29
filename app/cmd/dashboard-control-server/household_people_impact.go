package main

import (
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// household_people_impact.go counts active/future uses for the People card.
// It does not fetch providers, mutate app state, or inspect completed history.
func householdPeopleImpactZero() map[string]int {
	return map[string]int{"routines": 0, "chores": 0, "maintenance": 0, "todo": 0, "grocery": 0, "messages": 0}
}

func (a *app) householdPeopleImpactForRoster(id string, roster map[string]any) map[string]int {
	counts := householdPeopleImpactZero()
	if id == "" {
		return counts
	}
	routines := a.routinesPayloadForRoster(roster)
	for _, raw := range jsonutil.List(routines["routines"]) {
		for _, assignment := range jsonutil.List(jsonutil.Map(raw)["assignments"]) {
			if routinesID(jsonutil.Map(assignment)["personId"]) == id {
				counts["routines"]++
			}
		}
	}
	// Impact counting needs only Chore-owned eligibility. Reading the raw child
	// payload avoids re-entering the People service while its mutation lock is
	// held by the Dashboard Control People response path.
	chores := a.choreWheelService().Payload()
	for _, raw := range jsonutil.List(chores["chores"]) {
		for _, eligible := range jsonutil.List(jsonutil.Map(raw)["eligible"]) {
			if choreWheelID(eligible) == id {
				counts["chores"]++
				break
			}
		}
	}
	maintenance := a.maintenancePayload()
	for _, raw := range jsonutil.List(maintenance["tasks"]) {
		task := jsonutil.Map(raw)
		if task["state"] == "active" && maintenanceTaskPersonID(task) == id {
			counts["maintenance"]++
		}
	}
	counts["messages"] = a.familyBoardPersonMessageCount(id)
	for _, list := range a.readTodoListsIndex().Lists {
		cache := a.readTodoListCache(list.ID)
		for _, task := range cache.Tasks {
			if task.Status == "completed" || task.DashGoAssignment == nil || routinesID(task.DashGoAssignment.PersonID) != id {
				continue
			}
			if list.ID == a.todoMap()["grocery"] {
				counts["grocery"]++
			} else {
				counts["todo"]++
			}
		}
	}
	return counts
}
