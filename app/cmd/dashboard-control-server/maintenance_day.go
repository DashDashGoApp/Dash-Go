package main

import (
	"net/http"
)

func (a *app) handleMaintenanceDayGet(w http.ResponseWriter, r *http.Request) bool {
	date := maintenanceDate(r.URL.Query().Get("date"))
	if date == "" {
		a.err(w, "date must be YYYY-MM-DD", http.StatusBadRequest)
		return true
	}
	a.json(w, maintenanceDayResponse(a.maintenancePayload(), date))
	return true
}
