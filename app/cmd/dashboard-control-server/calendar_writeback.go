package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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
func (a *app) calendarWritebackStatus() map[string]any {
	status, err := a.calendarWritebackService().Status()
	if err != nil {
		return map[string]any{"enabled": false, "requirePin": false, "calendars": []any{}, "error": "Calendar writeback configuration is unavailable."}
	}
	rows := make([]any, 0, len(status.Calendars))
	for _, cal := range status.Calendars {
		row := map[string]any{"source": cal.Source, "name": cal.Name, "writable": cal.Writable, "enabled": cal.Enabled, "pair": cal.Pair, "provider": cal.Provider, "connection": cal.Connection, "remoteId": cal.RemoteID}
		if state, ok := status.States[cal.Source]; ok {
			row["state"] = state.State
			row["detail"] = state.Detail
			row["updatedAt"] = state.UpdatedAt
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
	a.recordAction("calendars", "Configure calendar edits", "success", fmt.Sprintf("%d registered calendar(s) · %s", len(status.Calendars), map[bool]string{true: "enabled", false: "disabled"}[status.Enabled]), nil)
	return a.calendarWritebackStatus(), nil
}

type calendarWritebackInput struct {
	Source   string
	UID      string
	Title    string
	Desc     string
	Location string
	AllDay   bool
	Start    time.Time
	End      time.Time
}

func parseCalendarWritebackInput(body map[string]any, needUID bool) (calendarWritebackInput, error) {
	input := calendarWritebackInput{Source: strings.TrimSpace(jsonutil.BodyString(body, "calUrl")), UID: strings.TrimSpace(jsonutil.BodyString(body, "uid")), Title: jsonutil.BodyString(body, "title"), Desc: jsonutil.BodyString(body, "desc"), Location: jsonutil.BodyString(body, "location"), AllDay: jsonutil.Truthy(body["allDay"])}
	if input.Source == "" {
		return input, errors.New("calendar required")
	}
	if needUID && input.UID == "" {
		return input, errors.New("event identifier required")
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

func (a *app) handleCalendarWritebackMutation(path string, body map[string]any) (map[string]any, error) {
	service := a.calendarWritebackService()
	var result writebackpkg.Result
	var input calendarWritebackInput
	var err error
	var refreshErr error
	err = service.WithSyncLock(func() error {
		switch path {
		case "/api/calendar/event/create":
			input, err = parseCalendarWritebackInput(body, false)
			if err != nil {
				return err
			}
			result, err = service.Create(input.Source, icalwrite.Event{Title: input.Title, Desc: input.Desc, Location: input.Location, Start: input.Start, End: input.End, AllDay: input.AllDay})
		case "/api/calendar/event/update":
			input, err = parseCalendarWritebackInput(body, true)
			if err != nil {
				return err
			}
			result, err = service.Update(input.Source, input.UID, icalwrite.Event{UID: input.UID, Title: input.Title, Desc: input.Desc, Location: input.Location, Start: input.Start, End: input.End, AllDay: input.AllDay})
		case "/api/calendar/event/delete":
			source, uid := strings.TrimSpace(jsonutil.BodyString(body, "calUrl")), strings.TrimSpace(jsonutil.BodyString(body, "uid"))
			if source == "" || uid == "" {
				return errors.New("calendar event required")
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
	a.queueCalendarWritebackSync(result.Source, result.Pair)
	action := map[string]string{"created": "Add calendar event", "updated": "Edit calendar event", "deleted": "Delete calendar event", "skipped": "Skip calendar occurrence"}[result.Action]
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

func (a *app) queueCalendarWritebackSync(source, pair string) {
	script := filepath.Join(a.binDir, "sync-vdir.sh")
	if info, err := os.Stat(script); err != nil || info.Mode()&0111 == 0 {
		a.calendarWritebackService().Record(source, "saved", "Saved locally; run private calendar sync to push remote changes.")
		return
	}
	key := strings.TrimSpace(pair)
	if key == "" {
		// A migrated beta.6 row has no pair until setup refresh. Keep its
		// behavior safe while avoiding a guessed remote collection target.
		key = "__legacy__"
	}
	a.writebackSyncMu.Lock()
	if a.writebackPending == nil {
		a.writebackPending = map[string]string{}
	}
	a.writebackPending[key] = source
	if a.writebackSyncing {
		a.writebackSyncMu.Unlock()
		a.calendarWritebackService().Record(source, "syncing", "Saved locally; this calendar is queued behind the active private sync.")
		return
	}
	a.writebackSyncing = true
	a.writebackSyncMu.Unlock()
	go a.runCalendarWritebackQueue(script)
}

func (a *app) runCalendarWritebackQueue(script string) {
	defer func() {
		a.writebackSyncMu.Lock()
		a.writebackSyncing = false
		a.writebackSyncMu.Unlock()
	}()
	for {
		a.writebackSyncMu.Lock()
		if len(a.writebackPending) == 0 {
			a.writebackSyncMu.Unlock()
			return
		}
		keys := make([]string, 0, len(a.writebackPending))
		for key := range a.writebackPending {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		key := keys[0]
		source := a.writebackPending[key]
		delete(a.writebackPending, key)
		a.writebackSyncMu.Unlock()

		a.calendarWritebackService().Record(source, "syncing", "Saved locally; synchronizing the selected private calendar.")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		args := []string{}
		if key != "__legacy__" {
			args = []string{"--pair", key}
		}
		output, err := exec.CommandContext(ctx, script, args...).CombinedOutput()
		timedOut := errors.Is(ctx.Err(), context.DeadlineExceeded)
		cancel()
		if err != nil {
			text := strings.ToLower(string(output))
			if strings.Contains(text, "conflict") {
				a.calendarWritebackService().Record(source, "conflict", "This calendar changed locally and remotely before sync. Nothing was overwritten automatically.")
			} else if timedOut {
				a.calendarWritebackService().Record(source, "waiting", "Saved locally; the selected remote sync timed out and will retry later.")
			} else {
				a.calendarWritebackService().Record(source, "waiting", "Saved locally; remote sync will retry automatically.")
			}
			continue
		}
		a.calendarWritebackService().Record(source, "synced", "Saved locally and synchronized.")
	}
}
