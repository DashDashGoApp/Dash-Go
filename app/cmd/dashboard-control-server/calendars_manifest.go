package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// Calendar CLI commands remain core adapters. Calendar owns the operation and
// durable state; core owns process exit codes and terminal wording.
func (a *app) runCalendarManifestCLI(args []string) int {
	if err := a.generateCalendarManifest(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
func (a *app) runDefaultCalendarsCLI(args []string) int {
	result, err := a.generateDefaultCalendars(true)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	body, _ := json.Marshal(result)
	fmt.Println(string(body))
	return 0
}
func (a *app) runHolidayUpdateCLI(args []string) int {
	result := a.updateHolidayCalendars()
	_ = a.generateCalendarManifest()
	_, _ = a.refreshEventCache(true, 90, 365)
	body, _ := json.Marshal(result)
	fmt.Println(string(body))
	if result["ok"] == false {
		return 1
	}
	return 0
}
func (a *app) runISSPassesCLI(args []string) int {
	result := a.updateISSPasses()
	_ = a.generateCalendarManifest()
	_, _ = a.refreshEventCache(true, 90, 365)
	body, _ := json.Marshal(result)
	fmt.Println(string(body))
	if result["ok"] == false {
		return 1
	}
	return 0
}
