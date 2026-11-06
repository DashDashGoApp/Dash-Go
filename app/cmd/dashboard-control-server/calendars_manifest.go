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
	manifestErr := a.generateCalendarManifest()
	_, refreshErr := a.refreshCurrentEventCache(true)
	body, _ := json.Marshal(result)
	fmt.Println(string(body))
	if manifestErr != nil {
		fmt.Fprintln(os.Stderr, "calendar manifest update failed:", manifestErr)
	}
	if refreshErr != nil {
		fmt.Fprintln(os.Stderr, "event cache refresh failed:", refreshErr)
	}
	if result["ok"] == false || refreshErr != nil {
		return 1
	}
	return 0
}
func (a *app) runISSPassesCLI(args []string) int {
	result := a.updateISSPasses()
	manifestErr := a.generateCalendarManifest()
	_, refreshErr := a.refreshCurrentEventCache(true)
	body, _ := json.Marshal(result)
	fmt.Println(string(body))
	if manifestErr != nil {
		fmt.Fprintln(os.Stderr, "calendar manifest update failed:", manifestErr)
	}
	if refreshErr != nil {
		fmt.Fprintln(os.Stderr, "event cache refresh failed:", refreshErr)
	}
	if result["ok"] == false || refreshErr != nil {
		return 1
	}
	return 0
}
