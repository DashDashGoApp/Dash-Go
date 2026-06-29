package main

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func (a *app) handlePublicPost(w http.ResponseWriter, r *http.Request, path string, body map[string]any) bool {
	if a.handleFontPost(w, r, path, body) {
		return true
	}
	if a.handleTodoPost(w, r, path, body) {
		return true
	}
	if a.handleChoreWheelPost(w, r, path, body) {
		return true
	}
	if a.handleFamilyBoardPost(w, r, path, body) {
		return true
	}
	if a.handleMaintenancePost(w, r, path, body) {
		return true
	}
	if a.handleRoutinesPost(w, r, path, body) {
		return true
	}
	switch path {
	case "/api/health/warnings/silence":
		minutes := jsonutil.Int(body["minutes"], 0)
		if minutes == 0 {
			// Beta.55 accepted whole hours. Retain that loopback-only shape so a
			// cached older browser bundle cannot turn a temporary silence into an error.
			minutes = jsonutil.Int(body["hours"], 0) * 60
		}
		warningSilences, err := a.setHealthWarningSilence(jsonutil.BodyString(body, "key"), minutes, time.Now())
		if err != nil {
			a.err(w, err.Error(), 400)
			return true
		}
		a.json(w, map[string]any{"ok": true, "warningSilences": warningSilences})
		return true
	case "/api/lock/unlock":
		wait := a.pinLockoutRemaining()
		if wait > 0 {
			a.json(w, map[string]any{"error": fmt.Sprintf("too many wrong passcode attempts; try again in %ss", strconv.Itoa(wait)), "lockout": true, "lockoutRemaining": wait, "retryAfter": wait, "detail": fmt.Sprintf("Try again in %d seconds.", wait)}, 429)
			return true
		}
		if a.verifyPin(jsonutil.BodyString(body, "pin")) {
			a.clearPinFailures()
			cfg := a.lockConfig()
			a.json(w, map[string]any{"ok": true, "token": a.issueToken(), "timeout": cfg["timeout"], "timeoutLabel": cfg["timeoutLabel"], "ttl": cfg["ttl"]})
			return true
		}
		a.recordPinFailure()
		a.err(w, "wrong passcode", 401)
		return true
	case "/api/lock/revoke":
		a.revoke(r.Header.Get("X-Dashboard-Token"))
		a.json(w, map[string]any{"ok": true})
		return true
	case "/api/lock/message-action":
		target := jsonutil.BodyString(body, "path")
		allowed := map[string]bool{"/api/compliments/defaults/toggle": true, "/api/message-sources/item/delete": true, "/api/compliments/delete": true, "/api/temporary-messages/delete": true, "/api/scheduled-messages/delete": true}
		if !allowed[target] {
			a.err(w, "message action is not allowed", 400)
			return true
		}
		if a.verifyPin(jsonutil.BodyString(body, "pin")) {
			a.clearPinFailures()
			a.json(w, map[string]any{"ok": true, "token": a.issueOneShot(target), "path": target, "ttl": int(oneShotTTL.Seconds())})
			return true
		}
		a.recordPinFailure()
		a.err(w, "wrong passcode", 401)
		return true
	case "/api/lock/set":
		if a.lockConfig()["enabled"] == false {
			cfg, err := a.setPin(jsonutil.BodyString(body, "pin"), body["timeout"])
			if err != nil {
				a.err(w, err.Error(), 400)
				return true
			}
			cfg["ok"] = true
			cfg["token"] = a.issueToken()
			a.json(w, cfg)
			return true
		}
	}
	return false
}
