package main

import (
	"net/http"
	"strings"
)

func (a *app) familyBoardInboxToken(r *http.Request) string {
	return strings.TrimSpace(r.Header.Get("X-DashGo-Inbox-Token"))
}

func (a *app) familyBoardRequireInbox(w http.ResponseWriter, r *http.Request, personID string) bool {
	if _, ok := a.familyBoardActivePerson(personID); !ok {
		a.err(w, "personal inbox is unavailable", http.StatusNotFound)
		return false
	}
	if !a.familyBoardInboxSessionOK(a.familyBoardInboxToken(r), personID) {
		a.err(w, "personal inbox is locked", http.StatusUnauthorized)
		return false
	}
	return true
}

func (a *app) familyBoardInboxGet(w http.ResponseWriter, r *http.Request, path string) bool {
	const prefix = "/api/family-board/inboxes/"
	if !strings.HasPrefix(path, prefix) || path == prefix {
		return false
	}
	personID := routinesID(strings.TrimPrefix(path, prefix))
	if !a.familyBoardRequireInbox(w, r, personID) {
		return true
	}
	view, actionErr := a.familyBoardService().ReadInbox(personID)
	if actionErr != nil {
		a.familyBoardActionError(w, actionErr)
		return true
	}
	a.json(w, view)
	return true
}

func (a *app) familyBoardInboxUnlock(w http.ResponseWriter, body map[string]any) bool {
	result, actionErr := a.familyBoardService().UnlockInbox(routinesID(body["personId"]), familyBoardString(body["pin"]))
	if actionErr != nil {
		a.familyBoardActionError(w, actionErr)
		return true
	}
	a.json(w, result)
	return true
}

func (a *app) familyBoardDirectSend(w http.ResponseWriter, r *http.Request, body map[string]any) bool {
	token := a.familyBoardInboxToken(r)
	entry, ok := a.familyBoardService().Session(token)
	senderID := entry.PersonID
	if !ok || !a.familyBoardRequireInbox(w, r, senderID) {
		return true
	}
	view, note, actionErr := a.familyBoardService().SendDirect(senderID, body)
	if actionErr != nil {
		a.familyBoardActionError(w, actionErr)
		return true
	}
	a.notifyPrivateFamilyMessage(note)
	a.json(w, view)
	return true
}

func (a *app) familyBoardDirectMutate(w http.ResponseWriter, r *http.Request, path string, body map[string]any) bool {
	const prefix = "/api/family-board/messages/"
	if !strings.HasPrefix(path, prefix) || path == "/api/family-board/messages" {
		return false
	}
	rest := strings.TrimPrefix(path, prefix)
	parts := strings.Split(rest, "/")
	if len(parts) != 2 {
		return false
	}
	messageID, action := familyBoardID(parts[0]), parts[1]
	if messageID == "" || (action != "archive" && action != "restore" && action != "delete" && action != "withdraw") {
		a.err(w, "unknown private message action", http.StatusBadRequest)
		return true
	}
	entry, ok := a.familyBoardService().Session(a.familyBoardInboxToken(r))
	if !ok || !a.familyBoardRequireInbox(w, r, entry.PersonID) {
		return true
	}
	view, actionErr := a.familyBoardService().MutateDirect(entry.PersonID, messageID, action)
	if actionErr != nil {
		a.familyBoardActionError(w, actionErr)
		return true
	}
	a.json(w, view)
	return true
}

func (a *app) handleFamilyBoardInboxPost(w http.ResponseWriter, r *http.Request, path string, body map[string]any) bool {
	switch path {
	case "/api/family-board/inboxes/unlock":
		return a.familyBoardInboxUnlock(w, body)
	case "/api/family-board/inboxes/lock":
		a.revokeFamilyBoardInboxSession(a.familyBoardInboxToken(r))
		a.json(w, map[string]any{"ok": true})
		return true
	case "/api/family-board/messages":
		return a.familyBoardDirectSend(w, r, body)
	}
	return a.familyBoardDirectMutate(w, r, path, body)
}
