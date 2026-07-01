package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
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
	case "/api/cache/rebuild":
		res, err := a.refreshEventCache(true, 90, 365)
		if err != nil {
			a.err(w, "event cache rebuild failed: "+err.Error(), 500)
			return
		}
		a.recordAction("cache", "Rebuild event cache", "success", fmt.Sprintf("%v events", res["eventCount"]), nil)
		a.json(w, res)
	case "/api/weather/refresh":
		payload, err := a.fetchGoWeather(r.Context())
		if err != nil {
			a.err(w, "weather refresh failed: "+err.Error(), 500)
			return
		}
		_ = fileio.WriteJSON(filepath.Join(a.cacheDir, "weather-cache.json"), payload)
		a.recordAction("weather", "Refresh weather", "success", fmt.Sprintf("%v source(s)", len(jsonutil.List(payload["sources"]))), nil)
		a.json(w, payload)
	case "/api/diagnostics":
		res, err := a.buildDiagnostics()
		if err != nil {
			a.err(w, "diagnostics failed: "+err.Error(), 500)
			return
		}
		a.recordAction("diagnostics", "Export diagnostics", "success", fmt.Sprintf("%v (%v bytes)", res["file"], res["size"]), nil)
		a.json(w, res)
	case "/api/backup":
		res, err := a.createConfigBackup("manual", "Manual backup from Dashboard Control", "", true)
		if err != nil {
			a.err(w, "backup failed: "+err.Error(), 500)
			return
		}
		a.recordAction("backup", "Create backup", "success", fmt.Sprint(res["name"]), map[string]any{"files": res["files"], "size": res["size"]})
		a.json(w, res)
	case "/api/backup/prune":
		res := a.pruneConfigBackups(jsonutil.Int(body["keep"], a.configBackupKeepLimit()))
		a.recordAction("backup", "Clean old backups", "success", fmt.Sprintf("kept %v newest · removed %v", res["keep"], res["removedCount"]), nil)
		a.json(w, res)
	case "/api/backup/restore":
		res, err := a.restoreConfigBackup(jsonutil.BodyString(body, "name"))
		if err != nil {
			a.err(w, "restore failed: "+err.Error(), 500)
			return
		}
		a.recordAction("restore", "Restore backup", "success", fmt.Sprintf("%v · %v files", res["name"], res["restored"]), nil)
		a.json(w, res)
	case "/api/backup/delete":
		res, err := a.deleteConfigBackup(jsonutil.BodyString(body, "name"))
		if err != nil {
			a.err(w, "backup delete failed: "+err.Error(), 500)
			return
		}
		a.recordAction("backup", "Delete backup", "success", fmt.Sprint(res["deleted"]), nil)
		a.json(w, res)
	case "/api/system-update":
		res, err := a.startSystemUpdate()
		if err != nil {
			a.recordAction("system-update", "System update", "failed", err.Error(), nil)
			a.err(w, "system update unavailable: "+err.Error(), 500)
			return
		}
		a.recordAction("system-update", "System update", "running", "started apt-get update && apt-get -y upgrade", nil)
		a.json(w, res)
	case "/api/doctor":
		repair := jsonutil.Truthy(body["fix"])
		plan := jsonutil.Truthy(body["plan"])
		if plan {
			repair = false
		}
		health := a.runDoctorSummaryMode(repair, plan)
		state := "check"
		if health["ok"] == true {
			state = "success"
		}
		action := "Run health check"
		if plan {
			action = "Review Doctor repair plan"
		} else if repair {
			action = "Run safe doctor repairs"
		}
		a.recordAction("health", action, state, fmt.Sprintf("%v · %v fixed · %v fail · %v warn", health["label"], health["fixCount"], health["failCount"], health["warnCount"]), nil)
		a.json(w, map[string]any{"ok": health["ok"], "summary": health, "output": health["outputTail"]})
	case "/api/update/track/toggle":
		res, err := a.toggleUpdateTrack()
		if err != nil {
			code := http.StatusInternalServerError
			if errors.Is(err, errDashboardUpdateTrackBusy) {
				code = http.StatusConflict
			}
			a.err(w, "could not switch update track: "+err.Error(), code)
			return
		}
		a.json(w, res)
	case "/api/update":
		res, err := a.startDashboardUpdate()
		if err != nil {
			code := http.StatusInternalServerError
			if errors.Is(err, errDashboardUpdateRunning) {
				// The existing active job already owns the only live update row.
				// A rejected repeat tap must not manufacture another In progress item.
				code = http.StatusConflict
			} else if !updateActionAlreadyRecorded(err) {
				a.recordAction("update", "Update dashboard", "failed", err.Error(), nil)
			}
			a.err(w, "could not safely start the updater: "+err.Error(), code)
			return
		}
		a.json(w, res)
	case "/api/reboot":
		rc := runCmd("sudo", "-n", "/sbin/reboot")
		if rc != 0 {
			a.err(w, "reboot not permitted", 500)
		} else {
			a.json(w, map[string]any{"rebooting": true})
		}
	case "/api/poweroff":
		rc := runCmd("sudo", "-n", "/sbin/poweroff")
		if rc != 0 {
			a.err(w, "shutdown not permitted", 500)
		} else {
			a.json(w, map[string]any{"poweroff": true})
		}
	case "/api/theme":
		theme := a.themeNameFromBody(body)
		if theme == "" {
			a.err(w, "theme name required", 400)
			return
		}
		if ok, reason := a.themeIsAvailable(theme); !ok {
			a.err(w, reason, 400)
			return
		}
		if err := a.writeTheme(theme); err != nil {
			a.err(w, "could not write theme: "+err.Error(), 500)
			return
		}
		a.json(w, map[string]any{"ok": true, "theme": theme})
	case "/api/seasonal":
		enabled := jsonutil.Truthy(body["enabled"])
		if err := a.setSeasonalThemesEnabled(enabled); err != nil {
			a.err(w, "could not update seasonal rotation: "+err.Error(), 500)
			return
		}
		a.json(w, map[string]any{"ok": true, "seasonal": enabled})
	case "/api/theme/base":
		name := jsonutil.BodyString(body, "name")
		if name == "" {
			a.err(w, "theme name required", 400)
			return
		}
		if ok, reason := a.themeIsAvailable(name); !ok {
			a.err(w, reason, 400)
			return
		}
		if err := os.WriteFile(filepath.Join(a.home, ".dashboard-base-theme"), []byte(name+"\n"), 0644); err != nil {
			// Preserve the existing successful response contract while making a
			// user-visible preference persistence failure diagnosable.
			log.Printf("could not persist base theme: %v", err)
		}
		a.json(w, map[string]any{"ok": true, "base": name})
	case "/api/compliments/add", "/api/compliments/delete", "/api/compliments/import", "/api/compliments/update", "/api/compliments/defaults/toggle", "/api/compliments/defaults/remove-all", "/api/compliments/defaults/add-all", "/api/compliments/clear-defaults", "/api/compliments/restore-defaults", "/api/compliments/reconcile-defaults":
		a.handleCompliments(w, path, body)
	case "/api/message-sources", "/api/message-sources/refresh", "/api/message-sources/item/delete", "/api/message-sources/item/update", "/api/temporary-messages/add", "/api/temporary-messages/delete", "/api/scheduled-messages/add", "/api/scheduled-messages/update", "/api/scheduled-messages/delete":
		a.handleMessages(w, path, body)
	case "/api/maps/prewarm":
		a.json(w, a.startMapPrewarm(body))
	case "/api/maps/cleanup":
		a.json(w, map[string]any{"ok": true, "cache": a.cleanMapImageCache(), "tileCache": a.cleanMapTileCache()})
	case "/api/maps/clear":
		a.json(w, a.clearMapCache(jsonutil.Truthy(body["clearGeocodes"]), jsonutil.Truthy(body["clearProvider"]), jsonutil.Truthy(body["clearTiles"])))
	default:
		a.err(w, "unknown endpoint", 404)
	}
}
