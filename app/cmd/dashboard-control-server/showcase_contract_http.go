package main

import (
	"net/http"
)

func (a *app) handleShowcaseStatus(w http.ResponseWriter, _ *http.Request) bool {
	if !a.showcaseMode() {
		return false
	}
	a.json(w, a.showcaseStatus())
	return true
}
