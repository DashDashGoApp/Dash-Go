package main

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func (a *app) handlePost(w http.ResponseWriter, r *http.Request, path string) {
	body, err := a.readBody(r)
	if err != nil {
		switch {
		case errors.Is(err, errRequestBodyTooLarge):
			a.err(w, "request body too large", http.StatusRequestEntityTooLarge)
		case errors.Is(err, errRequestFieldLimit):
			a.err(w, "request fields exceed supported limits", http.StatusBadRequest)
		default:
			a.err(w, "bad json", http.StatusBadRequest)
		}
		return
	}
	if res := a.handlePublicPost(w, r, path, body); res {
		return
	}
	autoDisplay := (path == "/api/display/off" || path == "/api/display/on") && jsonutil.Truthy(body["automatic"])
	oneShot := jsonutil.BodyString(body, "oneShotToken")
	if !autoDisplay && !a.tokenOK(r.Header.Get("X-Dashboard-Token")) && !a.consumeOneShot(oneShot, path) {
		a.err(w, "locked", 401)
		return
	}
	if a.handleHouseholdPeoplePost(w, r, path, body) {
		return
	}
	if a.handleHouseholdPeopleInboxPINPost(w, r, path, body) {
		return
	}
	if a.handleHouseholdPeopleNotificationPost(w, r, path, body) {
		return
	}
	if a.handleCalendarPost(w, r, path, body) {
		return
	}
	if a.handleOperationsPost(w, r, path, body) {
		return
	}
	if a.handlePresentationPost(w, path, body) {
		return
	}
	switch path {
	case "/api/lock/config":
		cfg := a.lockConfig()
		if cfg["available"] != true {
			a.pinConfigurationUnavailable(w)
			return
		}
		if cfg["enabled"] == false {
			a.err(w, "PIN lock is not enabled", http.StatusBadRequest)
			return
		}
		timeout := normalizeTimeout(body["timeout"])
		if err := a.setPinTimeout(timeout); err != nil {
			a.err(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		ref := a.refreshSession(r.Header.Get("X-Dashboard-Token"), timeout)
		out := a.lockConfig()
		for k, v := range ref {
			out[k] = v
		}
		if ref["sessionRefreshed"] == true {
			out["token"] = r.Header.Get("X-Dashboard-Token")
		}
		a.json(w, out)
	case "/api/lock/heartbeat":
		cfg := a.lockConfig()
		if cfg["available"] != true {
			a.pinConfigurationUnavailable(w)
			return
		}
		ref := a.refreshSession(r.Header.Get("X-Dashboard-Token"), fmt.Sprint(cfg["timeout"]))
		if ref["sessionRefreshed"] != true {
			a.err(w, "locked", http.StatusUnauthorized)
			return
		}
		a.json(w, ref)
	case "/api/lock/set":
		// A pre-existing session may not rotate a credential through the setup
		// route. Only /api/lock/change accepts an enabled lock, and it proves the
		// current PIN before replacing it.
		a.err(w, "PIN lock is already enabled; use Change PIN with the current PIN.", http.StatusConflict)
	case "/api/lock/change":
		if wait := a.pinLockoutRemaining(); wait > 0 {
			a.pinLockoutResponse(w, wait)
			return
		}
		if !a.verifyPin(jsonutil.BodyString(body, "currentPin")) {
			a.pinFailureResponse(w)
			return
		}
		cfg, err := a.setPin(jsonutil.BodyString(body, "pin"), body["timeout"])
		if err != nil {
			a.err(w, err.Error(), http.StatusBadRequest)
			return
		}
		cfg["ok"] = true
		cfg["token"] = a.issueToken()
		a.json(w, cfg)
	case "/api/lock/remove":
		if wait := a.pinLockoutRemaining(); wait > 0 {
			a.pinLockoutResponse(w, wait)
			return
		}
		if !a.verifyPin(jsonutil.BodyString(body, "currentPin")) {
			a.pinFailureResponse(w)
			return
		}
		cfg, err := a.removePin()
		if err != nil {
			a.err(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		cfg["ok"] = true
		a.json(w, cfg)
	case "/api/settings":
		// Settings.json remains writable for durable household preferences.
		// Calendar/weather cadence values from older controls are retained only as
		// inert history; runtime cadence is automatic and profile/provider-owned.
		merged, err := a.updateSettings(func(settings map[string]any) {
			for k, v := range body {
				settings[k] = v
			}
		})
		if err != nil {
			a.err(w, err.Error(), 400)
			return
		}
		a.json(w, merged)
	case "/api/radar/settings":
		if err := validateRadarSettings(body); err != nil {
			a.err(w, err.Error(), 400)
			return
		}
		if _, err := a.updateSettings(func(settings map[string]any) {
			for k, v := range body {
				settings[k] = v
			}
			// Radar source selection is automatic. Keep older browser clients
			// compatible without allowing them to re-enable a fixed provider.
			if _, requested := body["radarProvider"]; requested {
				settings["radarProvider"] = "auto"
			}
		}); err != nil {
			a.err(w, err.Error(), 400)
			return
		}
		a.json(w, a.radarStatus())
	case "/api/profile":
		if set, ok := body["set"].(map[string]any); ok && len(set) > 0 {
			payload, err := a.updateProfileValues(set)
			if err != nil {
				a.err(w, err.Error(), 400)
				return
			}
			a.json(w, payload)
			return
		}
		prof := jsonutil.BodyString(body, "profile")
		if !jsonutil.Truthy(body["applyDefaults"]) {
			a.err(w, "profile defaults require explicit confirmation", 400)
			return
		}
		payload, err := a.applyProfilePreset(prof)
		if err != nil {
			a.err(w, err.Error(), 400)
			return
		}
		a.json(w, payload)
	case "/api/chalkboard":
		if err := validateChalkboardPayload(body); err != nil {
			a.err(w, "request fields exceed supported limits", http.StatusBadRequest)
			return
		}
		if err := fileio.WriteJSON(filepath.Join(a.configDir, "chalkboard.json"), body); err != nil {
			a.err(w, err.Error(), 500)
			return
		}
		a.json(w, map[string]any{"ok": true})
	case "/api/display/off":
		a.runXset(w, "off")
	case "/api/display/on":
		a.runXset(w, "on")
	case "/api/browser/restart":
		rc := runCmd("pkill", "-x", "surf")
		a.json(w, map[string]any{"restarted": rc == 0})
	case "/api/terminal/open":
		res, err := a.openTerminal()
		if err != nil {
			if errors.Is(err, errTerminalAccessDisabled) {
				a.err(w, err.Error(), http.StatusForbidden)
				return
			}
			a.err(w, err.Error(), 500)
			return
		}
		a.json(w, res)
	default:
		a.err(w, "unknown endpoint", 404)
	}
}
