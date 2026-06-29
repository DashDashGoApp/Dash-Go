package main

import (
	"net/http"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func (a *app) handleCalendarToggle(w http.ResponseWriter, body map[string]any) {
	name, url := jsonutil.BodyString(body, "name"), jsonutil.BodyString(body, "url")
	if name == "" && url == "" {
		a.err(w, "unknown calendar", 400)
		return
	}
	result, err := a.calendarService().Toggle(name, url)
	if err != nil {
		a.err(w, err.Error(), 400)
		return
	}
	a.json(w, result)
}
