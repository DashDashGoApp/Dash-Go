package main

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// handleOperationsPost owns expensive maintenance, backup, update, and device
// actions. Keeping these routes together makes their limiter placement and
// validation order auditable without growing the general POST dispatcher.
func (a *app) handleOperationsPost(w http.ResponseWriter, r *http.Request, path string, body map[string]any) bool {
	switch path {
	case "/api/cache/rebuild":
		finish, ok := a.beginLimitedOperation(w, path)
		if !ok {
			return true
		}
		defer finish()
		res, err := a.refreshCurrentEventCache(true)
		if err != nil {
			a.err(w, "event cache rebuild failed: "+err.Error(), 500)
			return true
		}
		a.recordAction("cache", "Rebuild event cache", "success", fmt.Sprintf("%v events", res["eventCount"]), nil)
		a.json(w, res)
	case "/api/weather/refresh":
		finish, ok := a.beginLimitedOperation(w, path)
		if !ok {
			return true
		}
		defer finish()
		payload, err := a.refreshGoWeatherLive(r.Context())
		if err != nil {
			a.err(w, "weather refresh failed: "+err.Error(), 500)
			return true
		}
		a.recordAction("weather", "Refresh weather", "success", fmt.Sprintf("%v source(s)", len(jsonutil.List(payload["sources"]))), nil)
		a.json(w, payload)
	case "/api/diagnostics":
		finish, ok := a.beginLimitedOperation(w, path)
		if !ok {
			return true
		}
		defer finish()
		res, err := a.buildDiagnostics()
		if err != nil {
			a.err(w, "diagnostics failed: "+err.Error(), 500)
			return true
		}
		a.recordAction("diagnostics", "Export diagnostics", "success", fmt.Sprintf("%v (%v bytes)", res["file"], res["size"]), nil)
		a.json(w, res)
	case "/api/backup":
		finish, ok := a.beginLimitedOperation(w, path)
		if !ok {
			return true
		}
		defer finish()
		res, err := a.createConfigBackup("manual", "Manual backup from Dashboard Control", "", true)
		if err != nil {
			a.err(w, "backup failed: "+err.Error(), 500)
			return true
		}
		a.recordAction("backup", "Create backup", "success", fmt.Sprint(res["name"]), map[string]any{"files": res["files"], "size": res["size"]})
		a.json(w, res)
	case "/api/backup/prune":
		res := a.pruneConfigBackups(jsonutil.Int(body["keep"], a.configBackupKeepLimit()))
		a.recordAction("backup", "Clean old backups", "success", fmt.Sprintf("kept %v newest · removed %v", res["keep"], res["removedCount"]), nil)
		a.json(w, res)
	case "/api/backup/restore":
		name := jsonutil.BodyString(body, "name")
		if _, err := a.chooseBackup(name); err != nil {
			a.err(w, "restore failed: "+err.Error(), http.StatusBadRequest)
			return true
		}
		finish, ok := a.beginLimitedOperation(w, path)
		if !ok {
			return true
		}
		defer finish()
		res, err := a.restoreConfigBackup(name)
		if err != nil {
			a.err(w, "restore failed: "+err.Error(), 500)
			return true
		}
		a.recordAction("restore", "Restore backup", "success", fmt.Sprintf("%v · %v files", res["name"], res["restored"]), nil)
		a.json(w, res)
	case "/api/backup/delete":
		res, err := a.deleteConfigBackup(jsonutil.BodyString(body, "name"))
		if err != nil {
			a.err(w, "backup delete failed: "+err.Error(), 500)
			return true
		}
		a.recordAction("backup", "Delete backup", "success", fmt.Sprint(res["deleted"]), nil)
		a.json(w, res)
	case "/api/system-update":
		finish, ok := a.beginLimitedOperation(w, path)
		if !ok {
			return true
		}
		defer finish()
		res, err := a.startSystemUpdate()
		if err != nil {
			a.recordAction("system-update", "System update", "failed", err.Error(), nil)
			a.err(w, "system update unavailable: "+err.Error(), 500)
			return true
		}
		a.recordAction("system-update", "System update", "running", "started apt-get update && apt-get -y upgrade", nil)
		a.json(w, res)
	case "/api/doctor":
		finish, ok := a.beginLimitedOperation(w, path)
		if !ok {
			return true
		}
		defer finish()
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
			return true
		}
		a.json(w, res)
	case "/api/update":
		finish, ok := a.beginLimitedOperation(w, path)
		if !ok {
			return true
		}
		defer finish()
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
			return true
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
	default:
		return false
	}
	return true
}
