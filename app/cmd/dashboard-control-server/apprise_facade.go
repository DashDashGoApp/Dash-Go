package main

import (
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
	notifypkg "github.com/DashDashGoApp/Dash-Go/app/internal/notify"
)

// Notifications now live in internal/notify. Core retains this narrow facade
// so installer CLI commands, backup/restore hooks, Household routes, Family
// Board composition, and existing integration tests keep their stable public
// contracts while the service owns private route files and delivery state.
type appriseRouteStore = notifypkg.RouteStore
type apprisePersonPreferences = notifypkg.PersonPreferences
type apprisePreferencesStore = notifypkg.PreferencesStore
type appriseNotificationEvent = notifypkg.Event

const (
	appriseRouteSchema        = notifypkg.RouteSchema
	apprisePreferencesSchema  = notifypkg.PreferencesSchema
	appriseMaxRoutesPerPerson = notifypkg.MaxRoutesPerPerson
	appriseMaxRouteLength     = notifypkg.MaxRouteLength
)

func (a *app) newNotifyService(send notifypkg.SendFunc) *notifypkg.Service {
	if send == nil {
		send = notifypkg.DefaultSend
	}
	return notifypkg.New(notifypkg.ServiceConfig{
		Home:                a.home,
		ConfigDir:           a.configDir,
		NormalizePersonID:   routinesID,
		People:              a.apprisePeopleSnapshot,
		ActivePerson:        a.appriseCLIActivePerson,
		MessageStillCurrent: a.appriseMessageStillCurrent,
		Send:                send,
	})
}

func (a *app) notifyService() *notifypkg.Service {
	a.notifyInitMu.Lock()
	defer a.notifyInitMu.Unlock()
	if a.notify == nil {
		a.notify = a.newNotifyService(nil)
	}
	return a.notify
}

func (a *app) apprisePeopleSnapshot() []notifypkg.Person {
	people := []notifypkg.Person{}
	for _, raw := range jsonutil.List(a.householdPeoplePayload()["people"]) {
		person := jsonutil.Map(raw)
		if id := routinesID(person["id"]); id != "" {
			people = append(people, notifypkg.Person{
				ID:    id,
				Name:  householdPersonAssignmentName(person),
				State: jsonutil.StringValue(person["state"]),
			})
		}
	}
	return people
}

func (a *app) appriseRoutesFile() string      { return a.notifyService().RoutesFile() }
func (a *app) apprisePreferencesFile() string { return a.notifyService().PreferencesFile() }

func (a *app) saveAppriseRoutes(value appriseRouteStore) error {
	return a.notifyService().SaveRoutes(value)
}
func (a *app) saveApprisePreferences(value apprisePreferencesStore) error {
	return a.notifyService().SavePreferences(value)
}

func (a *app) appriseConfiguredForPerson(personID string) bool {
	return a.notifyService().ConfiguredForPerson(personID)
}
func (a *app) apprisePersonPreferences(personID string) apprisePersonPreferences {
	return a.notifyService().PersonPreferences(personID)
}
func (a *app) apprisePersonControlStatus(personID string) map[string]any {
	return a.notifyService().PersonControlStatus(personID)
}
func (a *app) setApprisePersonPreferences(personID string, pref apprisePersonPreferences) error {
	return a.notifyService().SetPersonPreferences(personID, pref)
}

func (a *app) restoreAppriseRoutes(stage string) error {
	return a.notifyService().RestoreRoutes(stage)
}
func (a *app) startAppriseNotifier() { a.notifyService().Start() }

func (a *app) enqueueAppriseEvent(event appriseNotificationEvent) {
	a.notifyService().Enqueue(event)
}

func (a *app) runAppriseCLI(command string, args []string) int {
	return a.notifyService().RunCLI(command, args)
}

func (a *app) appriseCLIActivePerson(personID string) bool {
	_, ok := a.familyBoardActivePerson(personID)
	return ok
}

// appriseMessageStillCurrent prevents an asynchronously queued board event
// from delivering after its household note was removed or a direct message was
// withdrawn. The local board remains the source of truth for every delivery.
func (a *app) appriseMessageStillCurrent(event appriseNotificationEvent) bool {
	if event.MessageID == "" {
		return true
	}
	return a.familyBoardService().MessageStillCurrent(event.MessageID, event.PersonID, event.Private)
}

func apprisePreviewText(sender, text string) string { return notifypkg.PreviewText(sender, text) }
