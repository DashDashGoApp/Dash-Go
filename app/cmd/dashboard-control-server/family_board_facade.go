package main

import (
	"time"

	familypkg "github.com/DashDashGoApp/Dash-Go/app/internal/household/family"
)

// Family Board now lives in internal/household/family. Core retains this
// facade so existing HTTP routes, People Control wiring, backup/restore, and
// notification composition preserve their public contracts while private board
// persistence, PIN verifiers, and session/lock state remain in the child
// service.
const (
	familyBoardSchema                    = familypkg.Schema
	familyBoardArchiveDays               = familypkg.ArchiveDays
	familyBoardMaxDirectMessagesPerInbox = familypkg.MaxDirectMessagesPerInbox
	familyBoardInboxVisibleLimit         = familypkg.InboxVisibleLimit
	familyBoardInboxSessionTTL           = familypkg.InboxSessionTTL
	familyBoardInboxPinsSchema           = familypkg.InboxPinsSchema
)

var familyBoardClock = time.Now

func familyBoardNow() time.Time { return familyBoardClock().In(time.Local) }

func (a *app) familyBoardService() *familypkg.Service {
	a.familyInitMu.Lock()
	defer a.familyInitMu.Unlock()
	if a.family == nil {
		a.family = familypkg.New(familypkg.ServiceConfig{
			Home:  a.home,
			Now:   familyBoardNow,
			Token: randToken,
			People: func() []map[string]any {
				rows := []map[string]any{}
				for _, raw := range householdPeopleActive(a.householdPeoplePayload()) {
					person, ok := raw.(map[string]any)
					if ok {
						rows = append(rows, person)
					}
				}
				return rows
			},
			PersonName: householdPersonAssignmentName,
		})
	}
	return a.family
}

func familyBoardDefault() map[string]any      { return familypkg.Default() }
func familyBoardString(v any) string          { return familypkg.String(v) }
func familyBoardText(v any, limit int) string { return familypkg.Text(v, limit) }
func familyBoardID(v any) string              { return familypkg.ID(v) }

func familyBoardStamp(v any) string { return familypkg.Stamp(v) }

func familyBoardExpiryAtEndOfLocalDate(day string) string {
	return familypkg.ExpiryAtEndOfLocalDate(day)
}
func familyBoardArchiveStamp(now time.Time) string { return familypkg.ArchiveStamp(now) }

func familyBoardPriority(v any) string { return familypkg.Priority(v) }

func familyBoardPersonSnapshot(v any) string                { return familypkg.PersonSnapshot(v) }
func familyBoardNote(raw any, now time.Time) map[string]any { return familypkg.Note(raw, now) }
func normalizeFamilyBoardPayload(raw map[string]any) map[string]any {
	return familypkg.Normalize(raw, familyBoardNow())
}

func familyBoardPublicPayload(payload map[string]any) map[string]any {
	return familypkg.PublicPayload(payload)
}
func familyBoardActive(payload map[string]any) []map[string]any { return familypkg.Active(payload) }
func familyBoardSummary(payload map[string]any) map[string]any  { return familypkg.Summary(payload) }
func familyBoardFind(payload map[string]any, id string) (int, map[string]any) {
	return familypkg.Find(payload, id)
}
func familyBoardDirectCount(payload map[string]any, personID string) int {
	return familypkg.DirectCount(payload, personID)
}

func familyBoardExpiration(note, body map[string]any, create bool, now time.Time) (string, error) {
	return familypkg.Expiration(note, body, create, now)
}
func familyBoardMutableNote(note, body map[string]any, create bool) error {
	return familypkg.MutableNote(note, body, create, familyBoardNow())
}

func (a *app) familyBoardFile() string              { return a.familyBoardService().StorePath() }
func (a *app) familyBoardInboxPinsFile() string     { return a.familyBoardService().PinsPath() }
func (a *app) ensureFamilyBoardPrivateStore() error { return a.familyBoardService().Ensure() }
func (a *app) writeFamilyBoardPrivatePayload(payload map[string]any) error {
	return a.familyBoardService().Write(payload)
}

func (a *app) familyBoardPayload() map[string]any              { return a.familyBoardService().Payload() }
func (a *app) familyBoardReadPayload() (map[string]any, error) { return a.familyBoardService().Read() }

func familyBoardInboxPinRecord(raw any) map[string]any { return familypkg.PinRecord(raw) }
func normalizeFamilyBoardInboxPins(raw map[string]any) map[string]any {
	return familypkg.NormalizePins(raw)
}
func (a *app) familyBoardInboxPinsPayload() map[string]any { return a.familyBoardService().Pins() }
func (a *app) writeFamilyBoardInboxPins(payload map[string]any) error {
	return a.familyBoardService().WritePins(payload)
}

func (a *app) setFamilyBoardInboxPIN(personID, pin string) error {
	return a.familyBoardService().SetPIN(personID, pin)
}
func (a *app) removeFamilyBoardInboxPIN(personID string) error {
	return a.familyBoardService().RemovePIN(personID)
}
func (a *app) verifyFamilyBoardInboxPIN(personID, pin string) bool {
	return a.familyBoardService().VerifyPIN(personID, pin)
}

type familyBoardInboxSession = familypkg.InboxSession

func (a *app) issueFamilyBoardInboxSession(personID string) string {
	return a.familyBoardService().IssueSession(personID)
}
func (a *app) familyBoardInboxSessionOK(token, personID string) bool {
	return a.familyBoardService().SessionOK(token, personID)
}
func (a *app) revokeFamilyBoardInboxSession(token string) {
	a.familyBoardService().RevokeSession(token)
}
func (a *app) revokeFamilyBoardInboxSessions(personID string) {
	a.familyBoardService().RevokeSessions(personID)
}

func (a *app) familyBoardActivePerson(personID string) (map[string]any, bool) {
	return a.familyBoardService().ActivePerson(personID)
}
func (a *app) familyBoardInboxDirectory() []any { return a.familyBoardService().InboxDirectory() }

func (a *app) familyBoardInboxPayload(payload map[string]any, personID string, markRead bool) (map[string]any, bool) {
	return a.familyBoardService().InboxPayload(payload, personID, markRead)
}
