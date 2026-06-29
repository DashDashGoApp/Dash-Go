package family

import (
	"slices"
	"strings"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func (s *Service) ActivePerson(personID string) (map[string]any, bool) {
	personID = PersonID(personID)
	if personID == "" {
		return nil, false
	}
	for _, person := range s.people() {
		if PersonID(person["id"]) == personID && person["state"] == "active" {
			return person, true
		}
	}
	return nil, false
}

func (s *Service) InboxDirectory() []any {
	pins := jsonutil.Map(s.Pins()["pins"])
	out := []any{}
	for _, person := range s.people() {
		if person["state"] != "active" {
			continue
		}
		id := PersonID(person["id"])
		if id == "" {
			continue
		}
		_, protected := pins[id]
		out = append(out, map[string]any{"id": id, "name": s.personName(person), "protected": protected})
	}
	slices.SortStableFunc(out, func(left, right any) int {
		return compareFoldedText(jsonutil.Map(left)["name"], jsonutil.Map(right)["name"])
	})
	return out
}

func DirectForPerson(note map[string]any, personID, direction string) bool {
	if note["scope"] != "direct" || PersonID(personID) == "" {
		return false
	}
	personID = PersonID(personID)
	if direction == "sent" {
		return PersonID(note["senderPersonId"]) == personID
	}
	if PersonID(note["recipientPersonId"]) != personID || Stamp(note["recipientDeletedAt"]) != "" || Stamp(note["withdrawnAt"]) != "" {
		return false
	}
	switch direction {
	case "inbox":
		return Stamp(note["recipientArchivedAt"]) == ""
	case "archive":
		return Stamp(note["recipientArchivedAt"]) != ""
	default:
		return false
	}
}

func DirectResponseRow(note map[string]any, direction string) map[string]any {
	out := map[string]any{}
	for _, key := range []string{"id", "scope", "priority", "createdAt", "updatedAt", "senderPersonId", "senderNameSnapshot", "recipientPersonId", "recipientNameSnapshot", "recipientReadAt", "withdrawnAt"} {
		out[key] = note[key]
	}
	if direction != "sent" {
		out["recipientArchivedAt"] = note["recipientArchivedAt"]
		out["recipientDeletedAt"] = note["recipientDeletedAt"]
	}
	out["archived"] = direction == "archive"
	if direction == "sent" && Stamp(note["withdrawnAt"]) != "" {
		out["text"] = "Message withdrawn"
		out["withdrawn"] = true
	} else {
		out["text"] = note["text"]
		out["withdrawn"] = false
	}
	return out
}

func directSort(rows []any) {
	slices.SortStableFunc(rows, func(left, right any) int {
		return -compareText(jsonutil.Map(left)["updatedAt"], jsonutil.Map(right)["updatedAt"])
	})
}

func (s *Service) InboxPayload(payload map[string]any, personID string, markRead bool) (map[string]any, bool) {
	inbox := []any{}
	archive := []any{}
	sent := []any{}
	changed := false
	now := ArchiveStamp(s.Now())
	notes := jsonutil.List(payload["notes"])
	for index, raw := range notes {
		note := jsonutil.Map(raw)
		if note["scope"] != "direct" {
			continue
		}
		if DirectForPerson(note, personID, "inbox") {
			if markRead && Stamp(note["recipientReadAt"]) == "" {
				note["recipientReadAt"] = now
				notes[index] = note
				changed = true
			}
			inbox = append(inbox, DirectResponseRow(note, "inbox"))
		}
		if DirectForPerson(note, personID, "archive") {
			archive = append(archive, DirectResponseRow(note, "archive"))
		}
		if DirectForPerson(note, personID, "sent") {
			sent = append(sent, DirectResponseRow(note, "sent"))
		}
	}
	payload["notes"] = notes
	directSort(inbox)
	directSort(archive)
	directSort(sent)
	if len(inbox) > InboxVisibleLimit {
		inbox = inbox[:InboxVisibleLimit]
	}
	if len(archive) > InboxVisibleLimit {
		archive = archive[:InboxVisibleLimit]
	}
	if len(sent) > InboxVisibleLimit {
		sent = sent[:InboxVisibleLimit]
	}
	person, _ := s.ActivePerson(personID)
	return map[string]any{
		"person": map[string]any{"id": PersonID(personID), "name": s.personName(person)},
		"inbox":  inbox, "archive": archive, "sent": sent, "ttl": int(InboxSessionTTL.Seconds()),
	}, changed
}

func compareFoldedText(left, right any) int {
	a, b := foldText(left), foldText(right)
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func foldText(v any) string {
	return string([]rune(strings.ToLower(String(v))))
}
