package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func parsePrivateCalendarSelection(output string) (kind string, selection privateCalendarSelection, state string, err error) {
	parts := strings.Split(strings.TrimSpace(output), "\t")
	if len(parts) == 0 || parts[0] == "" {
		return "", selection, "", errors.New("private calendar selection returned no result")
	}
	if parts[0] == "error" {
		if len(parts) > 1 {
			return "", selection, "", errors.New(strings.TrimSpace(parts[1]))
		}
		return "", selection, "", errors.New("private calendar selection failed")
	}
	if (parts[0] != "activated" && parts[0] != "existing") || len(parts) != 7 {
		return "", selection, "", errors.New("private calendar selection returned an invalid result")
	}
	selection = privateCalendarSelection{Source: strings.TrimSpace(parts[1]), Name: strings.TrimSpace(parts[2]), Provider: strings.TrimSpace(parts[3]), Pair: strings.TrimSpace(parts[4]), RemoteID: strings.TrimSpace(parts[5])}
	if parts[0] == "existing" {
		selection.Writable = strings.TrimSpace(parts[6]) == "1"
	}
	if !strings.HasPrefix(selection.Source, "calendars/") || !privateCalendarSafePair(selection.Pair) || !privateCalendarSafeField(selection.Name, 256) || !privateCalendarSafeField(selection.RemoteID, 512) || (selection.Provider != "google" && selection.Provider != "caldav") {
		return "", selection, "", errors.New("private calendar selection returned unsafe data")
	}
	if parts[0] == "activated" {
		state = strings.TrimSpace(parts[6])
	}
	return parts[0], selection, state, nil
}

func (a *app) activatePrivateCalendar(body map[string]any) (map[string]any, error) {
	pair, remoteID := strings.TrimSpace(jsonutil.BodyString(body, "pair")), strings.TrimSpace(jsonutil.BodyString(body, "remoteId"))
	candidate, err := a.findPrivateCalendarCandidate(pair, remoteID)
	if err != nil {
		return nil, err
	}
	editable := jsonutil.Truthy(body["editable"])
	script := filepath.Join(a.binDir, "private-calendar-selection.sh")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, script, "--activate", candidate.Pair, candidate.RemoteID, candidate.Name, candidate.Color, map[bool]string{true: "1", false: "0"}[editable])
	cmd.Env = append(os.Environ(), "DASH="+a.dash, "HOME="+a.home)
	output, runErr := cmd.CombinedOutput()
	kind, selection, initial, parseErr := parsePrivateCalendarSelection(string(output))
	if runErr != nil && parseErr == nil {
		return nil, errors.New("private calendar activation did not complete")
	}
	if parseErr != nil {
		return nil, parseErr
	}
	if editable {
		current := a.calendarWritebackStatus()
		if _, err := a.calendarWritebackService().Configure(true, current["requirePin"] == true, nil); err != nil {
			return nil, fmt.Errorf("enable Dashboard calendar edits: %w", err)
		}
	}
	if err := a.generateCalendarManifest(); err != nil {
		return nil, fmt.Errorf("refresh calendar manifest: %w", err)
	}
	if _, err := a.refreshEventCache(true, 90, 365); err != nil {
		return nil, fmt.Errorf("refresh dashboard events: %w", err)
	}
	selection.Writable = editable
	if initial == "waiting" {
		a.calendarWritebackService().Record(selection.Source, "waiting", "Selected calendar is saved; its initial remote sync needs attention.")
	} else if editable {
		a.calendarWritebackService().Record(selection.Source, "syncing", "Selected calendar is active; validating its initial remote sync.")
	}
	a.recordAction("calendars", "Select private calendar", "success", fmt.Sprintf("%s added as %s", candidate.Name, map[bool]string{true: "editable", false: "display-only"}[editable]), nil)
	out := a.privateCalendarStatus()
	out["result"] = kind
	return out, nil
}

func (a *app) setPrivateCalendarEditable(body map[string]any) (map[string]any, error) {
	source := strings.TrimSpace(jsonutil.BodyString(body, "source"))
	editable := jsonutil.Truthy(body["editable"])
	selectionFound := false
	for _, selection := range a.privateCalendarSelections() {
		if selection.Source == source {
			selectionFound = true
			break
		}
	}
	if !selectionFound {
		return nil, errors.New("selected private calendar is no longer available")
	}
	script := filepath.Join(a.binDir, "private-calendar-selection.sh")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, script, "--set-editable", source, map[bool]string{true: "1", false: "0"}[editable])
	cmd.Env = append(os.Environ(), "DASH="+a.dash, "HOME="+a.home)
	output, err := cmd.CombinedOutput()
	if err != nil || !strings.HasPrefix(string(output), "updated\t") {
		return nil, errors.New("could not update Dashboard edit permission for this private calendar")
	}
	if editable {
		current := a.calendarWritebackStatus()
		if _, err := a.calendarWritebackService().Configure(true, current["requirePin"] == true, nil); err != nil {
			return nil, fmt.Errorf("enable Dashboard calendar edits: %w", err)
		}
	}
	if _, err := a.refreshEventCache(true, 90, 365); err != nil {
		return nil, fmt.Errorf("refresh Dashboard event capabilities: %w", err)
	}
	a.recordAction("calendars", "Update private calendar edits", "success", fmt.Sprintf("%s is now %s in Dashboard Control", source, map[bool]string{true: "editable", false: "display-only"}[editable]), nil)
	return a.privateCalendarStatus(), nil
}

func (a *app) deactivatePrivateCalendar(body map[string]any) (map[string]any, error) {
	source := strings.TrimSpace(jsonutil.BodyString(body, "source"))
	selectionFound := false
	for _, selection := range a.privateCalendarSelections() {
		if selection.Source == source {
			selectionFound = true
			break
		}
	}
	if !selectionFound {
		return nil, errors.New("selected private calendar is no longer available")
	}
	script := filepath.Join(a.binDir, "private-calendar-selection.sh")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, script, "--deactivate", source)
	cmd.Env = append(os.Environ(), "DASH="+a.dash, "HOME="+a.home)
	output, err := cmd.CombinedOutput()
	if err != nil || !strings.HasPrefix(string(output), "deactivated\t") {
		return nil, errors.New("could not stop sync for this private calendar")
	}
	if err := a.generateCalendarManifest(); err != nil {
		return nil, fmt.Errorf("refresh calendar manifest: %w", err)
	}
	if _, err := a.refreshEventCache(true, 90, 365); err != nil {
		return nil, fmt.Errorf("refresh dashboard events: %w", err)
	}
	a.recordAction("calendars", "Stop private calendar sync", "success", "Remote calendar was left unchanged; the local mirror remains available.", nil)
	return a.privateCalendarStatus(), nil
}

func (a *app) syncPrivateCalendar(body map[string]any) (map[string]any, error) {
	source := strings.TrimSpace(jsonutil.BodyString(body, "source"))
	for _, selection := range a.privateCalendarSelections() {
		if selection.Source != source {
			continue
		}
		if err := a.calendarWritebackSourceBlocked(selection.Source); err != nil {
			return nil, err
		}
		a.queueCalendarWritebackSync(selection.Source, selection.Pair, false)
		a.recordAction("calendars", "Sync private calendar", "success", "Selected private calendar sync queued.", nil)
		return a.privateCalendarStatus(), nil
	}
	return nil, errors.New("selected private calendar is no longer available")
}
