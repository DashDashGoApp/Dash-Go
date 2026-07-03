package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

const (
	privateCalendarDiscoveryFile   = "private-calendar-discovery.json"
	privateCalendarDiscoverySchema = 1
)

type privateCalendarCandidate struct {
	Pair       string `json:"pair"`
	Provider   string `json:"provider"`
	Connection string `json:"connection"`
	RemoteID   string `json:"remoteId"`
	Name       string `json:"name"`
	Color      string `json:"color"`
}

type privateCalendarDiscovery struct {
	Schema     int                        `json:"schema"`
	UpdatedAt  string                     `json:"updatedAt"`
	Candidates []privateCalendarCandidate `json:"candidates"`
	Notices    []string                   `json:"notices,omitempty"`
}

type privateCalendarSelection struct {
	Source     string `json:"source"`
	Pair       string `json:"pair"`
	Provider   string `json:"provider"`
	Connection string `json:"connection"`
	RemoteID   string `json:"remoteId"`
	Name       string `json:"name"`
	Color      string `json:"color"`
	Writable   bool   `json:"writable"`
}

func (a *app) privateCalendarDiscoveryPath() string {
	return filepath.Join(a.configDir, privateCalendarDiscoveryFile)
}
func (a *app) privateCalendarVDirHome() string {
	return filepath.Join(a.home, ".dashboard-vdirsyncer")
}
func (a *app) privateCalendarMapPath() string {
	return filepath.Join(a.privateCalendarVDirHome(), "calendars.map")
}

func privateCalendarSafeField(value string, max int) bool {
	value = strings.TrimSpace(value)
	return value != "" && len(value) <= max && !strings.ContainsAny(value, "\r\n\t|")
}
func privateCalendarSafePair(value string) bool {
	if !privateCalendarSafeField(value, 96) {
		return false
	}
	for _, r := range value {
		if !(r == '-' || r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}
func privateCalendarSource(name, color, tag string) string {
	if tag != "" {
		return "calendars/" + name + "." + color + "." + tag + ".ics"
	}
	return "calendars/" + name + "." + color + ".ics"
}

func (a *app) loadPrivateCalendarDiscovery() privateCalendarDiscovery {
	payload := privateCalendarDiscovery{Schema: privateCalendarDiscoverySchema, Candidates: []privateCalendarCandidate{}, Notices: []string{}}
	b, err := os.ReadFile(a.privateCalendarDiscoveryPath())
	if err != nil || json.Unmarshal(b, &payload) != nil || payload.Schema != privateCalendarDiscoverySchema {
		return privateCalendarDiscovery{Schema: privateCalendarDiscoverySchema, Candidates: []privateCalendarCandidate{}, Notices: []string{}}
	}
	if payload.Candidates == nil {
		payload.Candidates = []privateCalendarCandidate{}
	}
	if payload.Notices == nil {
		payload.Notices = []string{}
	}
	return payload
}

func (a *app) savePrivateCalendarDiscovery(payload privateCalendarDiscovery) error {
	payload.Schema = privateCalendarDiscoverySchema
	if payload.Candidates == nil {
		payload.Candidates = []privateCalendarCandidate{}
	}
	if payload.Notices == nil {
		payload.Notices = []string{}
	}
	return fileio.WriteJSON(a.privateCalendarDiscoveryPath(), payload)
}

// privateCalendarSelections reads only the non-secret source map. Credentials
// remain in the paired private rows and are never returned to the browser.
func (a *app) privateCalendarSelections() []privateCalendarSelection {
	file, err := os.Open(a.privateCalendarMapPath())
	if err != nil {
		return []privateCalendarSelection{}
	}
	defer file.Close()
	rows := []privateCalendarSelection{}
	scan := bufio.NewScanner(file)
	scan.Buffer(make([]byte, 1024), 64*1024)
	seen := map[string]bool{}
	for scan.Scan() {
		parts := strings.Split(scan.Text(), "|")
		if len(parts) < 7 {
			continue
		}
		name, color, tag, pair := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), strings.TrimSpace(parts[2]), strings.TrimSpace(parts[3])
		writable, remoteID := strings.TrimSpace(parts[5]) == "1", strings.TrimSpace(parts[6])
		displayName, provider, connection := name, "caldav", name
		if len(parts) > 7 && strings.TrimSpace(parts[7]) != "" {
			displayName = strings.TrimSpace(parts[7])
		}
		if len(parts) > 8 && strings.TrimSpace(parts[8]) != "" {
			provider = strings.TrimSpace(parts[8])
		}
		if len(parts) > 9 && strings.TrimSpace(parts[9]) != "" {
			connection = strings.TrimSpace(parts[9])
		}
		source := privateCalendarSource(name, color, tag)
		if !privateCalendarSafeField(name, 96) || !privateCalendarSafeField(color, 32) || !privateCalendarSafePair(pair) || !privateCalendarSafeField(remoteID, 512) || !privateCalendarSafeField(displayName, 256) || !privateCalendarSafeField(connection, 96) || (provider != "google" && provider != "caldav") || seen[source] {
			continue
		}
		seen[source] = true
		rows = append(rows, privateCalendarSelection{Source: source, Pair: pair, Provider: provider, Connection: connection, RemoteID: remoteID, Name: displayName, Color: color, Writable: writable})
	}
	slices.SortFunc(rows, func(left, right privateCalendarSelection) int {
		return strings.Compare(strings.ToLower(left.Name), strings.ToLower(right.Name))
	})
	return rows
}

// privateCalendarLegacyBroadMirrorPresent detects only the old all-collections
// mapping shape. It never changes it: a user must choose whether to hide it
// after confirming that newly selected exact sources are present.
func (a *app) privateCalendarLegacyBroadMirrorPresent() bool {
	file, err := os.Open(a.privateCalendarMapPath())
	if err != nil {
		return false
	}
	defer file.Close()
	scan := bufio.NewScanner(file)
	for scan.Scan() {
		parts := strings.Split(scan.Text(), "|")
		if len(parts) >= 7 && strings.TrimSpace(parts[0]) != "" && strings.TrimSpace(parts[6]) == "" {
			return true
		}
	}
	return false
}

func providerLabel(provider string) string {
	if provider == "google" {
		return "Google"
	}
	return "iCloud / CalDAV"
}

func (a *app) privateCalendarStatus() map[string]any {
	discovery := a.loadPrivateCalendarDiscovery()
	selections := a.privateCalendarSelections()
	selectionByRemote := map[string]privateCalendarSelection{}
	for _, selection := range selections {
		selectionByRemote[selection.Connection+"\x00"+selection.RemoteID] = selection
	}
	registry := a.calendarWritebackStatus()
	states := map[string]map[string]any{}
	for _, raw := range jsonutil.List(registry["calendars"]) {
		row := jsonutil.Map(raw)
		if source := strings.TrimSpace(jsonutil.StringValue(row["source"])); source != "" {
			states[source] = row
		}
	}
	candidates := make([]any, 0, len(discovery.Candidates))
	for _, candidate := range discovery.Candidates {
		row := map[string]any{"pair": candidate.Pair, "provider": candidate.Provider, "providerLabel": providerLabel(candidate.Provider), "connection": candidate.Connection, "remoteId": candidate.RemoteID, "name": candidate.Name, "color": candidate.Color, "selected": false}
		if selection, ok := selectionByRemote[candidate.Connection+"\x00"+candidate.RemoteID]; ok {
			row["selected"] = true
			row["source"] = selection.Source
			row["writable"] = selection.Writable
			row["selectionPair"] = selection.Pair
			if state, found := states[selection.Source]; found {
				row["state"] = state["state"]
				row["detail"] = state["detail"]
				row["updatedAt"] = state["updatedAt"]
				if state["deleteAllowed"] != nil {
					row["deleteAllowed"] = state["deleteAllowed"]
				}
				if state["sync"] != nil {
					row["sync"] = state["sync"]
				}
			}
		}
		candidates = append(candidates, row)
	}
	selectedRows := make([]any, 0, len(selections))
	for _, selection := range selections {
		row := map[string]any{"source": selection.Source, "pair": selection.Pair, "provider": selection.Provider, "providerLabel": providerLabel(selection.Provider), "connection": selection.Connection, "remoteId": selection.RemoteID, "name": selection.Name, "color": selection.Color, "writable": selection.Writable}
		if state, found := states[selection.Source]; found {
			row["state"] = state["state"]
			row["detail"] = state["detail"]
			row["updatedAt"] = state["updatedAt"]
			if state["deleteAllowed"] != nil {
				row["deleteAllowed"] = state["deleteAllowed"]
			}
			if state["sync"] != nil {
				row["sync"] = state["sync"]
			}
		}
		selectedRows = append(selectedRows, row)
	}
	a.privateCalendarMu.Lock()
	discovering := a.privateCalendarDiscovering
	a.privateCalendarMu.Unlock()
	_, helperErr := os.Stat(filepath.Join(a.binDir, "private-calendar-discovery.sh"))
	return map[string]any{"available": helperErr == nil, "discovering": discovering, "updatedAt": discovery.UpdatedAt, "notices": discovery.Notices, "candidates": candidates, "selected": selectedRows, "legacyBroadMirror": a.privateCalendarLegacyBroadMirrorPresent()}
}

func parsePrivateCalendarDiscovery(output string) (privateCalendarDiscovery, error) {
	payload := privateCalendarDiscovery{Schema: privateCalendarDiscoverySchema, UpdatedAt: time.Now().UTC().Format(time.RFC3339), Candidates: []privateCalendarCandidate{}, Notices: []string{}}
	seen := map[string]bool{}
	for _, line := range strings.Split(output, "\n") {
		parts := strings.Split(line, "\t")
		if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
			continue
		}
		switch parts[0] {
		case "notice":
			if len(parts) == 2 && privateCalendarSafeField(parts[1], 512) {
				payload.Notices = append(payload.Notices, strings.TrimSpace(parts[1]))
			}
		case "calendar":
			if len(parts) != 7 {
				continue
			}
			candidate := privateCalendarCandidate{Pair: strings.TrimSpace(parts[1]), Provider: strings.TrimSpace(parts[2]), Connection: strings.TrimSpace(parts[3]), RemoteID: strings.TrimSpace(parts[4]), Name: strings.TrimSpace(parts[5]), Color: strings.TrimSpace(parts[6])}
			if !privateCalendarSafePair(candidate.Pair) || !privateCalendarSafeField(candidate.Connection, 96) || !privateCalendarSafeField(candidate.RemoteID, 512) || !privateCalendarSafeField(candidate.Name, 256) || !privateCalendarSafeField(candidate.Color, 32) || (candidate.Provider != "google" && candidate.Provider != "caldav") {
				continue
			}
			key := candidate.Connection + "\x00" + candidate.RemoteID
			if !seen[key] {
				seen[key] = true
				payload.Candidates = append(payload.Candidates, candidate)
			}
		}
	}
	slices.SortFunc(payload.Candidates, func(left, right privateCalendarCandidate) int {
		return strings.Compare(strings.ToLower(left.Name), strings.ToLower(right.Name))
	})
	return payload, nil
}

func (a *app) discoverPrivateCalendars() (map[string]any, error) {
	a.privateCalendarMu.Lock()
	if a.privateCalendarDiscovering {
		a.privateCalendarMu.Unlock()
		return nil, errors.New("private calendar discovery is already running")
	}
	a.privateCalendarDiscovering = true
	a.privateCalendarMu.Unlock()
	defer func() {
		a.privateCalendarMu.Lock()
		a.privateCalendarDiscovering = false
		a.privateCalendarMu.Unlock()
	}()
	script := filepath.Join(a.binDir, "private-calendar-discovery.sh")
	if info, err := os.Stat(script); err != nil || info.Mode()&0111 == 0 {
		return nil, errors.New("private calendar discovery is unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, script)
	cmd.Env = append(os.Environ(), "DASH="+a.dash, "HOME="+a.home)
	output, err := cmd.CombinedOutput()
	payload, parseErr := parsePrivateCalendarDiscovery(string(output))
	if parseErr != nil {
		return nil, parseErr
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		payload.Notices = append(payload.Notices, "Discovery timed out. Existing selected calendars were not changed.")
	}
	if err != nil && len(payload.Notices) == 0 {
		payload.Notices = append(payload.Notices, "Discovery could not complete. Existing selected calendars were not changed.")
	}
	if err := a.savePrivateCalendarDiscovery(payload); err != nil {
		return nil, fmt.Errorf("save calendar discovery: %w", err)
	}
	out := a.privateCalendarStatus()
	a.recordAction("calendars", "Discover private calendars", "success", fmt.Sprintf("%d calendar%s found; nothing was activated", len(payload.Candidates), map[bool]string{true: "", false: "s"}[len(payload.Candidates) == 1]), nil)
	return out, nil
}

func (a *app) findPrivateCalendarCandidate(pair, remoteID string) (privateCalendarCandidate, error) {
	for _, candidate := range a.loadPrivateCalendarDiscovery().Candidates {
		if candidate.Pair == pair && candidate.RemoteID == remoteID {
			return candidate, nil
		}
	}
	return privateCalendarCandidate{}, errors.New("discover this calendar again before selecting it")
}
