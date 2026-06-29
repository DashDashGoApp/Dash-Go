package main

import (
	maintenancepkg "github.com/DashDashGoApp/Dash-Go/app/internal/household/maintenance"
	routinespkg "github.com/DashDashGoApp/Dash-Go/app/internal/household/routines"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// household_people_reconcile.go applies a central roster change to future/open
// household work. Completed/skipped records retain their immutable snapshots.
// The People transaction is outermost; child services serialize only their own
// document and Core takes the calendar commit seam last.
func (a *app) reconcileHouseholdPeople(next map[string]any, op, personID, targetID string) error {
	if err := a.reconcileHouseholdPeopleRoutines(next, op, personID, targetID); err != nil {
		return err
	}
	if err := a.reconcileHouseholdPeopleChores(next, op, personID, targetID); err != nil {
		return err
	}
	if err := a.reconcileHouseholdPeopleMaintenance(op, personID, targetID, next); err != nil {
		return err
	}
	if err := a.reconcileHouseholdPeopleTodo(op, personID, targetID, next); err != nil {
		return err
	}
	return nil
}
func (a *app) reconcileHouseholdPeopleRoutines(next map[string]any, op, personID, targetID string) error {
	return a.routinesService().WithLock(func() error {
		payload := a.routinesService().Payload()
		updated, _ := routinespkg.ReconcilePeople(payload, next, op, personID, targetID, routinesNow())
		// Historic behavior wrote the current canonical People projection for every
		// People mutation, including rename/archive/restore. Keep that contract and
		// let the child service retain its own assignment/history correction rules.
		return a.commitRoutinesPayload(updated)
	})
}
func (a *app) reconcileHouseholdPeopleChores(next map[string]any, op, personID, targetID string) error {
	return a.choreWheelService().WithLock(func() error {
		payload := a.choreWheelService().Payload()
		targetName := ""
		if targetID != "" {
			for _, raw := range jsonutil.List(next["people"]) {
				person := jsonutil.Map(raw)
				if routinesID(person["id"]) == targetID {
					targetName = householdPersonAssignmentName(person)
					break
				}
			}
		}
		updated, err := a.choreWheelService().ReconcilePeople(payload, householdPeopleActive(next), op, personID, targetID, targetName)
		if err != nil {
			return err
		}
		return a.commitChoreWheelPayload(updated)
	})
}
func (a *app) reconcileHouseholdPeopleMaintenance(op, personID, targetID string, next map[string]any) error {
	return a.maintenanceService().WithLock(func() error {
		targetName := ""
		if targetID != "" {
			for _, raw := range jsonutil.List(next["people"]) {
				person := jsonutil.Map(raw)
				if routinesID(person["id"]) == targetID {
					targetName = householdPersonAssignmentName(person)
					break
				}
			}
		}
		payload := a.maintenanceService().Payload()
		updated, changed := maintenancepkg.ReconcilePeople(payload, op, personID, targetID, targetName, maintenanceNow())
		if !changed {
			return nil
		}
		return a.commitMaintenancePayload(updated)
	})
}
func (a *app) reconcileHouseholdPeopleTodo(op, personID, targetID string, next map[string]any) error {
	if op != "delete" {
		return nil
	}
	targetName := ""
	if targetID != "" {
		for _, raw := range jsonutil.List(next["people"]) {
			person := jsonutil.Map(raw)
			if routinesID(person["id"]) == targetID {
				targetName = householdPersonAssignmentName(person)
				break
			}
		}
	}
	for _, list := range a.readTodoListsIndex().Lists {
		unlock := a.todoListLock(list.ID)
		cache := a.readTodoListCache(list.ID)
		changed := false
		for index := range cache.Tasks {
			task := &cache.Tasks[index]
			if task.Status == "completed" || task.DashGoAssignment == nil || routinesID(task.DashGoAssignment.PersonID) != personID {
				continue
			}
			if targetID == "" {
				task.DashGoAssignment = nil
			} else {
				task.DashGoAssignment = &todoTaskAssignment{PersonID: targetID, PersonNameSnapshot: targetName, AssignedAt: todoNowMillis()}
			}
			changed = true
		}
		if changed {
			if err := a.writeTodoListCache(cache); err != nil {
				unlock()
				return err
			}
		}
		unlock()
	}
	return nil
}
