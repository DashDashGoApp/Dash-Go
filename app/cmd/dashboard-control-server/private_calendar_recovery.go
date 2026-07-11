package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
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
	privateCalendarRepairTimeout  = 6 * time.Minute
	privateCalendarResolveTimeout = 2 * time.Minute
	privateCalendarSnapshotFiles  = 512
	privateCalendarSnapshotBytes  = 8 << 20
	privateCalendarSnapshotsKeep  = 4
)

// beginPrivateCalendarRecovery reserves the in-process writeback queue while a
// targeted repair or conflict resolver owns the vdirsyncer wrapper. The wrapper
// still acquires the cross-process vdir lock, so cron remains authoritative for
// serialization across processes. Any ordinary event write arriving meanwhile
// is retained in the existing queue and starts after recovery returns.
func (a *app) beginPrivateCalendarRecovery() (func(), error) {
	a.writebackSyncMu.Lock()
	if a.writebackSyncing {
		a.writebackSyncMu.Unlock()
		return nil, errors.New("calendar sync is already running; try again shortly")
	}
	a.writebackSyncing = true
	a.writebackSyncMu.Unlock()

	return func() {
		a.writebackSyncMu.Lock()
		startQueued := len(a.writebackPending) > 0
		if startQueued {
			// Hand the reservation straight to the queue so a concurrent local
			// edit cannot be stranded after the recovery command finishes.
			a.writebackSyncing = true
		} else {
			a.writebackSyncing = false
		}
		a.writebackSyncMu.Unlock()
		if startQueued {
			go a.runCalendarWritebackQueue(filepath.Join(a.binDir, "sync-vdir.sh"))
		}
	}, nil
}

func (a *app) selectedPrivateCalendar(source string) (privateCalendarSelection, error) {
	source = strings.TrimSpace(source)
	for _, selection := range a.privateCalendarSelections() {
		if selection.Source == source {
			return selection, nil
		}
	}
	return privateCalendarSelection{}, errors.New("selected private calendar is no longer available")
}

func privateCalendarPairOutcome(output, pair string) string {
	return writebackPairResult(output, pair)
}

func privateCalendarSafeFailure(output string) string {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Split(strings.TrimSpace(line), "\t")
		if len(fields) == 2 && fields[0] == "error" {
			message := strings.TrimSpace(fields[1])
			if message != "" && len(message) <= 240 && !strings.ContainsAny(message, "\r\n") {
				return message
			}
		}
	}
	return "The private calendar operation could not complete. Existing calendar data was not changed."
}

func (a *app) refreshPrivateCalendarRecovery() error {
	if err := a.generateCalendarManifest(); err != nil {
		return fmt.Errorf("refresh calendar manifest: %w", err)
	}
	if _, err := a.refreshCurrentEventCache(true); err != nil {
		return fmt.Errorf("refresh dashboard events: %w", err)
	}
	return nil
}

// repairPrivateCalendar performs the one deliberate discover+sync sequence
// required for an exact previously selected pair. It never broad-discovers an
// account, edits selections, or supplies force-delete permission.
func (a *app) repairPrivateCalendar(body map[string]any) (map[string]any, error) {
	source := strings.TrimSpace(jsonutil.BodyString(body, "source"))
	selection, err := a.selectedPrivateCalendar(source)
	if err != nil {
		return nil, err
	}
	if _, err := a.calendarWritebackService().RegisteredCalendar(selection.Source); err != nil {
		return nil, err
	}
	finish, err := a.beginPrivateCalendarRecovery()
	if err != nil {
		return nil, err
	}
	defer finish()

	script := filepath.Join(a.binDir, "private-calendar-selection.sh")
	info, statErr := os.Stat(script)
	if statErr != nil || info.Mode()&0111 == 0 {
		return nil, errors.New("private calendar repair is unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), privateCalendarRepairTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, script, "--repair", selection.Source)
	cmd.Env = append(os.Environ(), "DASH="+a.dash, "HOME="+a.home)
	output, runErr := cmd.CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		a.calendarWritebackService().Record(selection.Source, "attention", "This selected calendar repair timed out before changing its sync selection.")
		return nil, errors.New("private calendar repair timed out")
	}
	if runErr != nil {
		message := privateCalendarSafeFailure(string(output))
		a.calendarWritebackService().Record(selection.Source, "attention", message)
		return nil, errors.New(message)
	}
	if !strings.HasPrefix(strings.TrimSpace(string(output)), "repaired\t") {
		return nil, errors.New("private calendar repair returned an invalid result")
	}
	if err := a.refreshPrivateCalendarRecovery(); err != nil {
		a.calendarWritebackService().Record(selection.Source, "waiting", "The private calendar was repaired, but Dashboard refresh will retry automatically.")
		return nil, err
	}
	a.calendarWritebackService().Record(selection.Source, "synced", "The selected calendar connection was repaired and synchronized.")
	a.recordAction("calendars", "Repair private calendar connection", "success", "Targeted collection discovery and sync completed.", map[string]any{"source": selection.Source, "pair": selection.Pair})
	out := a.privateCalendarStatus()
	out["result"] = "repaired"
	return out, nil
}

func (a *app) privateCalendarConflict(source string) bool {
	status, err := a.calendarWritebackService().Status()
	if err != nil {
		return false
	}
	state, ok := status.States[source]
	if !ok {
		return false
	}
	return state.Sync == "conflict" || state.State == "conflict"
}

func (a *app) privateCalendarSnapshotRoot() string {
	return filepath.Join(a.privateCalendarVDirHome(), "conflict-snapshots")
}

// snapshotPrivateCalendarCollection creates a bounded, owner-only local copy
// before a user deliberately chooses the remote winner. It is not advertised
// as a remote backup and is never served through Dashboard Control.
func (a *app) snapshotPrivateCalendarCollection(pair, collection string) error {
	if !privateCalendarSafePair(pair) {
		return errors.New("invalid private calendar pair")
	}
	root := filepath.Clean(collection)
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return errors.New("private calendar collection is unavailable")
	}
	stamp := time.Now().UTC().Format("20060102T150405Z")
	if err := os.MkdirAll(a.privateCalendarSnapshotRoot(), 0700); err != nil {
		return err
	}
	dest, err := os.MkdirTemp(a.privateCalendarSnapshotRoot(), pair+"-"+stamp+"-")
	if err != nil {
		return err
	}
	if err := os.Chmod(dest, 0700); err != nil {
		_ = os.RemoveAll(dest)
		return err
	}
	files, bytes := 0, int64(0)
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return errors.New("private calendar collection contains an unsafe link")
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".ics") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
			return errors.New("private calendar snapshot path is unsafe")
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files++
		bytes += int64(len(body))
		if files > privateCalendarSnapshotFiles || bytes > privateCalendarSnapshotBytes {
			return errors.New("private calendar snapshot exceeds its safety limit")
		}
		return fileio.WriteAtomic(filepath.Join(dest, rel), body, 0600)
	})
	if err != nil {
		_ = os.RemoveAll(dest)
		return err
	}
	return a.trimPrivateCalendarSnapshots()
}

func (a *app) trimPrivateCalendarSnapshots() error {
	root := a.privateCalendarSnapshotRoot()
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	dirs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}
	slices.Sort(dirs)
	for len(dirs) > privateCalendarSnapshotsKeep {
		if err := os.RemoveAll(filepath.Join(root, dirs[0])); err != nil {
			return err
		}
		dirs = dirs[1:]
	}
	return nil
}

// resolvePrivateCalendarConflict runs vdirsyncer once with an ephemeral
// conflict policy for exactly one registered pair. Normal generated configs
// remain conflict-safe and never retain a winner policy.
func (a *app) resolvePrivateCalendarConflict(body map[string]any) (map[string]any, error) {
	source, winner := strings.TrimSpace(jsonutil.BodyString(body, "source")), strings.TrimSpace(jsonutil.BodyString(body, "winner"))
	if winner != "remote" && winner != "dashboard" {
		return nil, errors.New("choose the remote calendar or this dashboard version")
	}
	if !a.lockConfigAvailable() || a.lockConfig()["enabled"] != true {
		return nil, errors.New("configure and unlock a Dashboard Control PIN before resolving calendar conflicts")
	}
	selection, err := a.selectedPrivateCalendar(source)
	if err != nil {
		return nil, err
	}
	calendar, err := a.calendarWritebackService().RegisteredCalendar(selection.Source)
	if err != nil {
		return nil, err
	}
	if !a.privateCalendarConflict(selection.Source) {
		return nil, errors.New("this calendar no longer has a conflict to resolve")
	}
	finish, err := a.beginPrivateCalendarRecovery()
	if err != nil {
		return nil, err
	}
	defer finish()
	if err := a.snapshotPrivateCalendarCollection(calendar.Pair, calendar.Collection); err != nil {
		return nil, fmt.Errorf("could not prepare the local conflict snapshot: %w", err)
	}

	script := filepath.Join(a.binDir, "sync-vdir.sh")
	info, statErr := os.Stat(script)
	if statErr != nil || info.Mode()&0111 == 0 {
		return nil, errors.New("private calendar sync is unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), privateCalendarResolveTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, script, "--pair", calendar.Pair, "--resolve-conflict", winner)
	cmd.Env = append(os.Environ(), "DASH="+a.dash, "HOME="+a.home)
	output, runErr := cmd.CombinedOutput()
	outcome := privateCalendarPairOutcome(string(output), calendar.Pair)
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		a.calendarWritebackService().Record(selection.Source, "conflict", "Conflict resolution timed out. Both versions were left unchanged.")
		return nil, errors.New("calendar conflict resolution timed out")
	}
	if runErr != nil || outcome != "synced" {
		if outcome == "conflict" {
			a.calendarWritebackService().Record(selection.Source, "conflict", "Conflict remains unresolved. Both versions were left unchanged.")
		} else {
			a.calendarWritebackService().Record(selection.Source, "attention", "Conflict resolution did not complete. Both versions were left unchanged.")
		}
		return nil, errors.New("calendar conflict resolution did not complete; both versions were left unchanged")
	}
	if err := a.calendarWritebackService().MergeCollection(calendar.Source, calendar.Collection); err != nil {
		a.calendarWritebackService().Record(selection.Source, "waiting", "Conflict was resolved, but Dashboard refresh will retry automatically.")
		return nil, fmt.Errorf("refreshing the local calendar mirror: %w", err)
	}
	if _, err := a.refreshCurrentEventCache(true); err != nil {
		a.calendarWritebackService().Record(selection.Source, "waiting", "Conflict was resolved, but Dashboard refresh will retry automatically.")
		return nil, fmt.Errorf("refreshing Dashboard events: %w", err)
	}
	winnerLabel := map[string]string{"remote": "remote calendar", "dashboard": "Dashboard"}[winner]
	a.calendarWritebackService().Record(selection.Source, "synced", "Conflict resolved using the selected "+winnerLabel+" version.")
	a.recordAction("calendars", "Resolve private calendar conflict", "success", "Applied one targeted conflict choice after an owner-only local snapshot.", map[string]any{"source": selection.Source, "pair": calendar.Pair, "winner": winner, "localSnapshot": true})
	out := a.privateCalendarStatus()
	out["result"] = "resolved"
	return out, nil
}
