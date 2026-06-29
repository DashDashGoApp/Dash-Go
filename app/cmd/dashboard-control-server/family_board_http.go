package main

import (
	"net/http"
	"strings"

	familypkg "github.com/DashDashGoApp/Dash-Go/app/internal/household/family"
)

// HTTP remains a thin core adapter: routes, request decoding, response codes,
// and notification composition stay here while the Family Board child service
// owns the private document, canonical state, inbox behavior, PIN verifiers,
// and session state.
func (a *app) familyBoardResponse(payload map[string]any) map[string]any {
	return map[string]any{
		"state": familyBoardPublicPayload(payload), "summary": familyBoardSummary(payload),
		"inboxes": a.familyBoardInboxDirectory(),
	}
}

func (a *app) familyBoardActionError(w http.ResponseWriter, err *familypkg.ActionError) {
	if err.Status == http.StatusTooManyRequests && err.Lockout {
		a.json(w, map[string]any{"error": err.Message, "lockout": true, "retryAfter": err.RetryAfter}, err.Status)
		return
	}
	a.err(w, err.Message, err.Status)
}

func (a *app) handleFamilyBoardGet(w http.ResponseWriter, r *http.Request, path string) bool {
	switch path {
	case "/api/family-board":
		payload, err := a.familyBoardReadPayload()
		if err != nil {
			a.err(w, "save expired family notes: "+err.Error(), http.StatusInternalServerError)
			return true
		}
		a.json(w, a.familyBoardResponse(payload))
		return true
	case "/api/family-board/summary":
		payload, err := a.familyBoardReadPayload()
		if err != nil {
			a.err(w, "save expired family notes: "+err.Error(), http.StatusInternalServerError)
			return true
		}
		a.json(w, familyBoardSummary(payload))
		return true
	case "/api/family-board/inboxes":
		a.json(w, map[string]any{"inboxes": a.familyBoardInboxDirectory()})
		return true
	}
	return a.familyBoardInboxGet(w, r, path)
}

func familyBoardHouseholdAction(path string) (string, bool) {
	switch path {
	case "/api/family-board/notes/acknowledge":
		return "acknowledge", true
	case "/api/family-board/settings":
		return "settings", true
	case "/api/family-board/notes/add":
		return "add", true
	case "/api/family-board/notes/update":
		return "update", true
	case "/api/family-board/notes/archive":
		return "archive", true
	case "/api/family-board/notes/restore":
		return "restore", true
	case "/api/family-board/notes/delete":
		return "delete", true
	default:
		return "", false
	}
}

func (a *app) handleFamilyBoardPost(w http.ResponseWriter, r *http.Request, path string, body map[string]any) bool {
	if !strings.HasPrefix(path, "/api/family-board/") {
		return false
	}
	if a.handleFamilyBoardInboxPost(w, r, path, body) {
		return true
	}
	action, ok := familyBoardHouseholdAction(path)
	if !ok {
		return false
	}
	payload, dispatchHousehold, actionErr := a.familyBoardService().MutateHousehold(action, body)
	if actionErr != nil {
		a.familyBoardActionError(w, actionErr)
		return true
	}
	if dispatchHousehold != nil {
		a.notifyUrgentHouseholdMessage(dispatchHousehold)
	}
	a.json(w, a.familyBoardResponse(payload))
	return true
}
