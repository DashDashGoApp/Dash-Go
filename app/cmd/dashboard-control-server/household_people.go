package main

import (
	householdpkg "github.com/DashDashGoApp/Dash-Go/app/internal/household"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// Household People now live in internal/household. Core keeps this narrow
// facade so existing household apps, routes, CLI-adjacent flows, and broad
// integration tests retain stable helpers while dependent domains are moved in
// later refactor betas.
const householdPeopleSchema = householdpkg.Schema

func (a *app) householdService() *householdpkg.Service {
	a.householdInitMu.Lock()
	defer a.householdInitMu.Unlock()
	if a.household == nil {
		a.household = householdpkg.New(householdpkg.ServiceConfig{ConfigDir: a.configDir, Now: routinesNow})
	}
	return a.household
}

func (a *app) householdPeopleFile() string { return a.householdService().File() }

func normalizeHouseholdPeople(raw map[string]any) map[string]any {
	return householdpkg.Normalize(raw, routinesNow())
}

func (a *app) householdPeoplePayload() map[string]any { return a.householdService().Payload() }

// writeHouseholdRoster is the narrow core orchestration seam for the one
// legacy household-app route that atomically updates canonical People before
// its own child-service document. Roster ownership and atomic persistence
// remain in internal/household.
func (a *app) writeHouseholdRoster(roster map[string]any) error {
	roster = normalizeHouseholdPeople(roster)
	roster["revision"] = max(0, jsonutil.Int(roster["revision"], 0)) + 1
	return a.householdService().Write(roster)
}
func householdPeopleActive(payload map[string]any) []any { return householdpkg.Active(payload) }

func (a *app) ensureHouseholdPeople(candidates ...[]any) map[string]any {
	return a.householdService().Ensure(candidates...)
}

func (a *app) householdPeopleAssignmentLookup(id string) (map[string]any, bool) {
	return a.householdService().AssignmentLookup(id)
}
func (a *app) householdPeopleActiveAssignment(id string) (map[string]any, bool) {
	return a.householdService().ActiveAssignment(id)
}
func householdPersonAssignmentName(person map[string]any) string {
	return householdpkg.PersonName(person)
}
func householdPeopleFind(rows []any, id string) (int, map[string]any) {
	return householdpkg.Find(rows, id)
}
func householdPeopleDeleteTarget(next map[string]any, body map[string]any, removedID string) (string, error) {
	return householdpkg.DeleteTarget(next, body, removedID)
}
