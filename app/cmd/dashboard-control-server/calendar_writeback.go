package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/calendar/icalwrite"
	writebackpkg "github.com/DashDashGoApp/Dash-Go/app/internal/calendar/writeback"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

const (
	calendarWritebackRegistryFile = "calendar-writeback.json"
	calendarWritebackStatusFile   = "calendar-writeback-status.json"
)

func (a *app) calendarWritebackService() *writebackpkg.Service {
	a.writebackInitMu.Lock()
	defer a.writebackInitMu.Unlock()
	if a.writeback == nil {
		a.writeback = writebackpkg.New(writebackpkg.Config{
			RegistryPath: filepath.Join(a.configDir, calendarWritebackRegistryFile),
			StatusPath:   filepath.Join(a.cacheDir, calendarWritebackStatusFile),
			VDirHome:     filepath.Join(a.home, ".dashboard-vdirsyncer"),
			CalendarDir:  a.calDir,
			Now:          time.Now,
		})
	}
	return a.writeback
}

func (a *app) calendarWritebackSourceWritable(source string) bool {
	return a.calendarWritebackService().SourceWritable(source)
}

// calendarWritebackCacheFingerprint is deliberately derived from only the local
// registry bytes. It exposes no configuration contents to the event package but
// makes cache capabilities change whenever the master switch, a selected source,
// or PIN policy changes outside an immediate forced refresh.
func (a *app) calendarWritebackCacheFingerprint() string {
	b, err := os.ReadFile(filepath.Join(a.configDir, calendarWritebackRegistryFile))
	if err != nil {
		return "unconfigured"
	}
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum[:])
}
func (a *app) calendarWritebackRequirePIN() bool { return a.calendarWritebackService().RequirePIN() }

// Delete is intentionally stricter than create/edit/skip: it is available
// only through an enabled Dashboard Control PIN session, never merely because
// loopback access is present.
func (a *app) calendarWritebackDeleteAllowed(source string) bool {
	if !a.calendarWritebackService().SourceWritable(source) || !a.lockConfigAvailable() {
		return false
	}
	return a.lockConfig()["enabled"] == true
}

func (a *app) calendarWritebackSourceBlocked(source string) error {
	status, err := a.calendarWritebackService().Status()
	if err != nil {
		return nil
	}
	state, ok := status.States[strings.TrimSpace(source)]
	if !ok {
		return nil
	}
	switch state.Sync {
	case "conflict":
		return errors.New("this calendar has a sync conflict; resolve it in Calendar Manager before changing events")
	case "attention-undiscovered":
		return errors.New("this calendar needs connection repair in Calendar Manager before changing events")
	}
	if state.State == "conflict" {
		return errors.New("this calendar has a sync conflict; resolve it in Calendar Manager before changing events")
	}
	return nil
}
func (a *app) calendarWritebackStatus() map[string]any {
	status, err := a.calendarWritebackService().Status()
	if err != nil {
		return map[string]any{"enabled": false, "requirePin": false, "calendars": []any{}, "error": "Calendar writeback configuration is unavailable."}
	}
	rows := make([]any, 0, len(status.Calendars))
	for _, cal := range status.Calendars {
		deleteAllowed := a.calendarWritebackDeleteAllowed(cal.Source)
		row := map[string]any{"source": cal.Source, "name": cal.Name, "writable": cal.Writable, "enabled": cal.Enabled, "pair": cal.Pair, "provider": cal.Provider, "connection": cal.Connection, "remoteId": cal.RemoteID, "deleteAllowed": deleteAllowed}
		if state, ok := status.States[cal.Source]; ok {
			row["state"] = state.State
			row["detail"] = state.Detail
			row["updatedAt"] = state.UpdatedAt
			if state.Sync != "" {
				row["sync"] = state.Sync
			}
		}
		rows = append(rows, row)
	}
	out := map[string]any{"enabled": status.Enabled, "requirePin": status.RequirePIN, "calendars": rows}
	if status.Last != nil {
		out["last"] = status.Last
	}
	return out
}

func (a *app) configureCalendarWriteback(body map[string]any) (map[string]any, error) {
	requested := map[string]bool{}
	for _, raw := range jsonutil.List(body["calendars"]) {
		row := jsonutil.Map(raw)
		source := strings.TrimSpace(jsonutil.StringValue(row["source"]))
		if source != "" {
			requested[source] = jsonutil.Truthy(row["enabled"])
		}
	}
	current := a.calendarWritebackStatus()
	enabled := current["enabled"] == true
	requirePIN := current["requirePin"] == true
	if _, ok := body["enabled"]; ok {
		enabled = jsonutil.Truthy(body["enabled"])
	}
	if _, ok := body["requirePin"]; ok {
		requirePIN = jsonutil.Truthy(body["requirePin"])
	}
	status, err := a.calendarWritebackService().Configure(enabled, requirePIN, requested)
	if err != nil {
		return nil, err
	}
	if _, err := a.refreshEventCache(true, 90, 365); err != nil {
		return nil, fmt.Errorf("refresh Dashboard event capabilities: %w", err)
	}
	a.recordAction("calendars", "Configure calendar edits", "success", fmt.Sprintf("%d registered calendar(s) · %s", len(status.Calendars), map[bool]string{true: "enabled", false: "disabled"}[status.Enabled]), nil)
	return a.calendarWritebackStatus(), nil
}

type calendarWritebackInput struct {
	Source     string
	UID        string
	Title      string
	Desc       string
	Location   string
	AllDay     bool
	Start      time.Time
	End        time.Time
	Occurrence time.Time
}

func parseCalendarWritebackInput(body map[string]any, needUID, needOccurrence bool) (calendarWritebackInput, error) {
	input := calendarWritebackInput{Source: strings.TrimSpace(jsonutil.BodyString(body, "calUrl")), UID: strings.TrimSpace(jsonutil.BodyString(body, "uid")), Title: jsonutil.BodyString(body, "title"), Desc: jsonutil.BodyString(body, "desc"), Location: jsonutil.BodyString(body, "location"), AllDay: jsonutil.Truthy(body["allDay"])}
	if input.Source == "" {
		return input, errors.New("calendar required")
	}
	if needUID && input.UID == "" {
		return input, errors.New("event identifier required")
	}
	if needOccurrence {
		ms := anyInt64(body["occurrenceMs"], 0)
		if ms <= 0 {
			return input, errors.New("calendar occurrence required")
		}
		input.Occurrence = time.UnixMilli(ms)
	}
	if input.AllDay {
		start, err := time.ParseInLocation("2006-01-02", jsonutil.BodyString(body, "startDate"), time.Local)
		if err != nil {
			return input, errors.New("valid start date required")
		}
		end, err := time.ParseInLocation("2006-01-02", jsonutil.BodyString(body, "endDate"), time.Local)
		if err != nil {
			return input, errors.New("valid end date required")
		}
		input.Start, input.End = start, end
	} else {
		startMS := anyInt64(body["startMs"], 0)
		endMS := anyInt64(body["endMs"], 0)
		if startMS <= 0 || endMS <= 0 {
			return input, errors.New("start and end times required")
		}
		input.Start, input.End = time.UnixMilli(startMS), time.UnixMilli(endMS)
	}
	now := time.Now()
	if input.Start.Before(now.AddDate(-1, 0, 0)) || input.Start.After(now.AddDate(3, 0, 0)) || input.End.After(now.AddDate(3, 1, 0)) {
		return input, errors.New("event date is outside the supported calendar window")
	}
	if err := (icalwrite.Event{UID: firstNonEmpty(input.UID, "dashgo-validation"), Title: input.Title, Desc: input.Desc, Location: input.Location, Start: input.Start, End: input.End, AllDay: input.AllDay}).Validate(); err != nil {
		return input, err
	}
	return input, nil
}

// calendarWritebackCacheCapability validates that a browser-provided recurring
// occurrence still exists in the local dashboard window and retains the
// server-declared capability. This prevents a stale or crafted request from
// creating a detached override for an arbitrary date.
func (a *app) calendarWritebackCacheCapability(source, uid string, occurrence time.Time, field string) bool {
	cache := jsonutil.Map(a.readJSONDefault(filepath.Join(a.cacheDir, "events.cache.json"), map[string]any{}))
	for _, raw := range jsonutil.List(cache["events"]) {
		item := jsonutil.Map(raw)
		if strings.TrimSpace(jsonutil.StringValue(item["calUrl"])) != strings.TrimSpace(source) || strings.TrimSpace(jsonutil.StringValue(item["uid"])) != strings.TrimSpace(uid) {
			continue
		}
		capability := jsonutil.Map(item["writeback"])
		if capability["candidate"] != true || capability[field] != true {
			continue
		}
		if !occurrence.IsZero() && anyInt64(capability["occurrenceMs"], 0) != occurrence.UnixMilli() {
			continue
		}
		return true
	}
	return false
}

func (a *app) handleCalendarWritebackMutation(path string, body map[string]any) (map[string]any, error) {
	service := a.calendarWritebackService()
	var result writebackpkg.Result
	var input calendarWritebackInput
	var err error
	var refreshErr error
	err = service.WithSyncLock(func() error {
		switch path {
		case "/api/calendar/event/create":
			input, err = parseCalendarWritebackInput(body, false, false)
			if err != nil {
				return err
			}
			if err = a.calendarWritebackSourceBlocked(input.Source); err != nil {
				return err
			}
			result, err = service.Create(input.Source, icalwrite.Event{Title: input.Title, Desc: input.Desc, Location: input.Location, Start: input.Start, End: input.End, AllDay: input.AllDay})
		case "/api/calendar/event/update":
			input, err = parseCalendarWritebackInput(body, true, false)
			if err != nil {
				return err
			}
			if err = a.calendarWritebackSourceBlocked(input.Source); err != nil {
				return err
			}
			result, err = service.Update(input.Source, input.UID, icalwrite.Event{UID: input.UID, Title: input.Title, Desc: input.Desc, Location: input.Location, Start: input.Start, End: input.End, AllDay: input.AllDay})
		case "/api/calendar/event/occurrence/update":
			input, err = parseCalendarWritebackInput(body, true, true)
			if err != nil {
				return err
			}
			if err = a.calendarWritebackSourceBlocked(input.Source); err != nil {
				return err
			}
			if !a.calendarWritebackCacheCapability(input.Source, input.UID, input.Occurrence, "canOccurrenceEdit") {
				return errors.New("this calendar occurrence is no longer available for Dash-Go editing")
			}
			result, err = service.UpdateOccurrence(input.Source, input.UID, input.Occurrence, icalwrite.Event{UID: input.UID, Title: input.Title, Desc: input.Desc, Location: input.Location, Start: input.Start, End: input.End, AllDay: input.AllDay})
		case "/api/calendar/event/series/update":
			input, err = parseCalendarWritebackInput(body, true, false)
			if err != nil {
				return err
			}
			if err = a.calendarWritebackSourceBlocked(input.Source); err != nil {
				return err
			}
			if !a.calendarWritebackCacheCapability(input.Source, input.UID, time.Time{}, "canSeriesEdit") {
				return errors.New("this repeating series uses an advanced pattern; manage the series in its calendar app")
			}
			result, err = service.UpdateSeries(input.Source, input.UID, icalwrite.Event{UID: input.UID, Title: input.Title, Desc: input.Desc, Location: input.Location, Start: input.Start, End: input.End, AllDay: input.AllDay})
		case "/api/calendar/event/delete":
			source, uid := strings.TrimSpace(jsonutil.BodyString(body, "calUrl")), strings.TrimSpace(jsonutil.BodyString(body, "uid"))
			if source == "" || uid == "" {
				return errors.New("calendar event required")
			}
			if err = a.calendarWritebackSourceBlocked(source); err != nil {
				return err
			}
			if !a.calendarWritebackDeleteAllowed(source) {
				return errors.New("configure and unlock a Dashboard Control PIN before deleting calendar events")
			}
			result, err = service.Delete(source, uid)
		case "/api/calendar/event/skip-occurrence":
			source, uid := strings.TrimSpace(jsonutil.BodyString(body, "calUrl")), strings.TrimSpace(jsonutil.BodyString(body, "uid"))
			occurrence := time.UnixMilli(anyInt64(body["occurrenceMs"], 0))
			if source == "" || uid == "" || occurrence.UnixMilli() <= 0 {
				return errors.New("calendar occurrence required")
			}
			if err = a.calendarWritebackSourceBlocked(source); err != nil {
				return err
			}
			if !a.calendarWritebackCacheCapability(source, uid, occurrence, "canSkip") {
				return errors.New("this calendar occurrence is no longer available for Dash-Go skipping")
			}
			result, err = service.SkipOccurrence(source, uid, occurrence)
		default:
			return errors.New("unknown calendar edit")
		}
		if err != nil {
			return err
		}
		if err = service.MergeCollection(result.Source, result.Collection); err != nil {
			refreshErr = fmt.Errorf("refreshing the local calendar mirror: %w", err)
			return nil
		}
		if _, err = a.refreshEventCache(true, 90, 365); err != nil {
			refreshErr = fmt.Errorf("refreshing Dashboard events: %w", err)
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, writebackpkg.ErrBusy) {
			return nil, fmt.Errorf("calendar sync is already running; try again shortly")
		}
		return nil, err
	}
	message := "Saved locally; remote sync queued."
	if refreshErr != nil {
		message = "Saved locally; remote sync queued. Dashboard refresh will retry automatically."
	}
	service.Record(result.Source, "saved", message)
	a.queueCalendarWritebackSync(result.Source, result.Pair, result.Action == "deleted")
	action := map[string]string{"created": "Add calendar event", "updated": "Manage calendar event", "occurrence-updated": "Edit calendar occurrence", "series-updated": "Edit recurring series", "deleted": "Delete calendar event", "skipped": "Skip calendar occurrence"}[result.Action]
	severity := "success"
	if refreshErr != nil {
		severity = "warning"
	}
	a.recordAction("calendars", action, severity, message, map[string]any{"source": result.Source, "uid": result.UID})
	response := map[string]any{"ok": true, "source": result.Source, "uid": result.UID, "action": result.Action, "sync": "queued"}
	if refreshErr != nil {
		response["warning"] = message
	}
	return response, nil
}
