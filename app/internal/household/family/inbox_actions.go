package family

import (
	"fmt"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// ReadInbox records recipient read state only after the route adapter has
// confirmed an active, authorized inbox session.
func (s *Service) ReadInbox(personID string) (map[string]any, *ActionError) {
	s.Lock()
	defer s.Unlock()
	payload := s.PayloadLocked()
	view, changed := s.InboxPayload(payload, personID, true)
	if !changed {
		return view, nil
	}
	payload = Normalize(payload, s.Now())
	if err := s.WriteLocked(payload); err != nil {
		return nil, actionError(500, "could not save inbox read state")
	}
	return view, nil
}

// SendDirect applies all direct-message validation and persistence while
// keeping notification composition in core. It returns the sent note only
// after the private document has been written successfully.
func (s *Service) SendDirect(senderID string, body map[string]any) (map[string]any, map[string]any, *ActionError) {
	senderID = PersonID(senderID)
	sender, senderOK := s.ActivePerson(senderID)
	if !senderOK {
		return nil, nil, actionError(404, "personal inbox is unavailable")
	}
	recipientID := PersonID(body["recipientPersonId"])
	if recipientID == "" || recipientID == senderID {
		return nil, nil, actionError(400, "choose another active household person")
	}
	recipient, recipientOK := s.ActivePerson(recipientID)
	if !recipientOK {
		return nil, nil, actionError(400, "recipient is not an active household person")
	}
	text := Text(body["text"], 320)
	if text == "" {
		return nil, nil, actionError(400, "message text is required")
	}
	if len([]rune(String(body["text"]))) > 320 {
		return nil, nil, actionError(400, "message must be 320 characters or fewer")
	}

	s.Lock()
	defer s.Unlock()
	payload := s.PayloadLocked()
	if DirectCount(payload, recipientID) >= MaxDirectMessagesPerInbox {
		return nil, nil, actionError(409, "recipient inbox has reached its local message limit; delete archived messages before sending more")
	}
	now := ArchiveStamp(s.Now())
	note := map[string]any{
		"id": NewID(s.Now()), "scope": "direct", "text": text, "priority": Priority(body["priority"]),
		"state": "active", "createdAt": now, "updatedAt": now,
		"senderPersonId": senderID, "senderNameSnapshot": s.personName(sender),
		"recipientPersonId": recipientID, "recipientNameSnapshot": s.personName(recipient),
		"recipientReadAt": "", "recipientArchivedAt": "", "recipientDeletedAt": "", "withdrawnAt": "",
	}
	payload["notes"] = append(jsonutil.List(payload["notes"]), note)
	payload = Normalize(payload, s.Now())
	if err := s.WriteLocked(payload); err != nil {
		return nil, nil, actionError(500, "%s", err)
	}
	view, _ := s.InboxPayload(payload, senderID, false)
	return view, note, nil
}

// MutateDirect preserves recipient-scoped archive/delete behavior and sender
// withdrawal rules. The route adapter supplies the authenticated actor ID.
func (s *Service) MutateDirect(actorID, messageID, action string) (map[string]any, *ActionError) {
	actorID = PersonID(actorID)
	messageID = ID(messageID)
	if messageID == "" || (action != "archive" && action != "restore" && action != "delete" && action != "withdraw") {
		return nil, actionError(400, "unknown private message action")
	}
	s.Lock()
	defer s.Unlock()
	payload := s.PayloadLocked()
	index, note := Find(payload, messageID)
	if index < 0 || note == nil || note["scope"] != "direct" {
		return nil, actionError(404, "private message was not found")
	}
	now := ArchiveStamp(s.Now())
	switch action {
	case "archive":
		if PersonID(note["recipientPersonId"]) != actorID {
			return nil, actionError(403, "only the recipient can archive this message")
		}
		if Stamp(note["recipientDeletedAt"]) != "" {
			return nil, actionError(409, "deleted private message cannot be archived")
		}
		if Stamp(note["recipientArchivedAt"]) != "" {
			return nil, actionError(409, "private message is already archived")
		}
		note["recipientArchivedAt"] = now
	case "restore":
		if PersonID(note["recipientPersonId"]) != actorID {
			return nil, actionError(403, "only the recipient can restore this archived message")
		}
		if Stamp(note["recipientDeletedAt"]) != "" {
			return nil, actionError(409, "deleted private message cannot be restored")
		}
		if Stamp(note["recipientArchivedAt"]) == "" {
			return nil, actionError(409, "private message is not archived")
		}
		note["recipientArchivedAt"] = ""
	case "delete":
		if PersonID(note["recipientPersonId"]) != actorID {
			return nil, actionError(403, "only the recipient can delete this archived message")
		}
		if Stamp(note["recipientDeletedAt"]) != "" {
			return nil, actionError(409, "private message is already deleted from this inbox")
		}
		if Stamp(note["recipientArchivedAt"]) == "" {
			return nil, actionError(409, "archive this private message before deleting it")
		}
		note["recipientDeletedAt"] = now
	case "withdraw":
		if PersonID(note["senderPersonId"]) != actorID {
			return nil, actionError(403, "only the sender can withdraw this message")
		}
		if Stamp(note["recipientReadAt"]) != "" {
			return nil, actionError(409, "a read message cannot be withdrawn")
		}
		if Stamp(note["recipientArchivedAt"]) != "" || Stamp(note["recipientDeletedAt"]) != "" {
			return nil, actionError(409, "an archived or deleted message cannot be withdrawn")
		}
		note["withdrawnAt"] = now
	default:
		return nil, actionError(400, "unknown private message action")
	}
	note["updatedAt"] = now
	notes := jsonutil.List(payload["notes"])
	notes[index] = note
	payload["notes"] = notes
	payload = Normalize(payload, s.Now())
	if err := s.WriteLocked(payload); err != nil {
		return nil, actionError(500, "%s", err)
	}
	view, _ := s.InboxPayload(payload, actorID, false)
	return view, nil
}

// UnlockInbox keeps verifier, lockout, and session state together. The result
// is plain domain data so core can keep the unchanged HTTP response shape.
func (s *Service) UnlockInbox(personID, pin string) (map[string]any, *ActionError) {
	personID = PersonID(personID)
	person, ok := s.ActivePerson(personID)
	if !ok {
		return nil, actionError(404, "personal inbox is unavailable")
	}
	if s.PinConfigured(personID) {
		if wait := s.LockoutRemaining(personID); wait > 0 {
			return nil, &ActionError{Status: 429, Message: fmt.Sprintf("too many wrong inbox PIN attempts; try again in %ds", wait), Lockout: true, RetryAfter: wait}
		}
		if !s.VerifyPIN(personID, pin) {
			if wait := s.RecordPINFailure(personID); wait > 0 {
				return nil, &ActionError{Status: 429, Message: fmt.Sprintf("too many wrong inbox PIN attempts; try again in %ds", wait), Lockout: true, RetryAfter: wait}
			}
			return nil, actionError(401, "wrong inbox PIN")
		}
		s.ClearPINFailures(personID)
	}
	return map[string]any{
		"ok": true, "inboxToken": s.IssueSession(personID),
		"ttl":    int(InboxSessionTTL.Seconds()),
		"person": map[string]any{"id": personID, "name": s.personName(person)},
	}, nil
}
