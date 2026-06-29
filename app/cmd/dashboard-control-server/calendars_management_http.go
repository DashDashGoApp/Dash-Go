package main

import (
	"fmt"
	"net/http"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func (a *app) handleCalendarManagementPost(w http.ResponseWriter, path string, body map[string]any) {
	switch path {
	case "/api/calendars/manage/delete":
		record, err := a.archiveLocalCalendar(strOr(body["url"], ""), strOr(body["name"], ""))
		if err != nil {
			a.err(w, err.Error(), 400)
			return
		}
		a.recordAction("calendars", "Archive local calendar", "success", record.Name+" retained for 30 days", nil)
		a.json(w, map[string]any{"ok": true, "calendar": record, "manager": a.calendarManagementStatus()})
	case "/api/calendars/manage/restore":
		record, err := a.restoreLocalCalendar(strOr(body["id"], ""))
		if err != nil {
			a.err(w, err.Error(), 400)
			return
		}
		a.recordAction("calendars", "Restore local calendar", "success", record.Name, nil)
		a.json(w, map[string]any{"ok": true, "calendar": record, "manager": a.calendarManagementStatus()})
	case "/api/calendars/manage/app-output":
		result, err := a.setOwnedCalendarOutput(strOr(body["owner"], ""), jsonutil.Truthy(body["enabled"]))
		if err != nil {
			a.err(w, err.Error(), 400)
			return
		}
		a.recordAction("calendars", "Set app calendar output", "success", fmt.Sprintf("%v %s", result["owner"], tern(result["enabled"] == true, "enabled", "disabled")), nil)
		result["ok"] = true
		result["manager"] = a.calendarManagementStatus()
		a.json(w, result)
	case "/api/calendars/manage/repair":
		result, err := a.repairCalendarIndex()
		if err != nil {
			a.err(w, err.Error(), 500)
			return
		}
		a.recordAction("calendars", "Repair calendar index", "success", fmt.Sprintf("%v sources", result["after"]), nil)
		result["manager"] = a.calendarManagementStatus()
		a.json(w, result)
	default:
		a.err(w, "unknown calendar management action", 404)
	}
}
