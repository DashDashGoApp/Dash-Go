package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// writebackPendingSync is one coalesced queued targeted sync request.
type writebackPendingSync struct {
	Source      string
	FinalDelete bool
}

func (a *app) queueCalendarWritebackSync(source, pair string, finalDelete bool) {
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
		a.writebackPending = map[string]writebackPendingSync{}
	}
	entry := writebackPendingSync{Source: source}
	// A queued delete keeps its empty-collection permission even when a later
	// non-delete change coalesces into the same pending slot; the deletion
	// still has to propagate through this run.
	if previous, ok := a.writebackPending[key]; ok && previous.FinalDelete {
		entry.FinalDelete = true
	}
	entry.FinalDelete = entry.FinalDelete || finalDelete
	a.writebackPending[key] = entry
	if a.writebackSyncing {
		a.writebackSyncMu.Unlock()
		a.calendarWritebackService().Record(source, "syncing", "Saved locally; this calendar is queued behind the active private sync.")
		return
	}
	a.writebackSyncing = true
	a.writebackSyncMu.Unlock()
	go a.runCalendarWritebackQueue(script)
}

// writebackPairResult extracts this pair's classified outcome from the
// wrapper's structured stdout RESULT lines. An empty string means the wrapper
// predates classification or did not reach this pair.
func writebackPairResult(output, pair string) string {
	// Exact selected pairs produce at most one RESULT row. A migrated legacy
	// source has no pair identity, so its full-run fallback must represent the
	// most serious outcome rather than whichever pair happened to be printed
	// first.
	severity := map[string]int{
		"":                       0,
		"synced":                 1,
		"skipped":                2,
		"failed":                 3,
		"attention-auth":         4,
		"attention-undiscovered": 4,
		"attention-empty":        4,
		"conflict":               5,
	}
	best := ""
	for line := range strings.Lines(output) {
		fields := strings.Split(strings.TrimRight(line, "\r\n"), "\t")
		if len(fields) != 3 || fields[0] != "RESULT" || (pair != "__legacy__" && fields[1] != pair) {
			continue
		}
		state := strings.TrimSpace(fields[2])
		if pair != "__legacy__" {
			return state
		}
		if severity[state] > severity[best] {
			best = state
		}
	}
	return best
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
		pending := a.writebackPending[key]
		source := pending.Source
		delete(a.writebackPending, key)
		a.writebackSyncMu.Unlock()

		a.calendarWritebackService().Record(source, "syncing", "Saved locally; synchronizing the selected private calendar.")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		args := []string{}
		if key != "__legacy__" {
			args = []string{"--pair", key}
			if pending.FinalDelete {
				args = append(args, "--allow-empty-once")
			}
		}
		output, err := exec.CommandContext(ctx, script, args...).CombinedOutput()
		timedOut := errors.Is(ctx.Err(), context.DeadlineExceeded)
		cancel()
		if err != nil {
			switch writebackPairResult(string(output), key) {
			case "conflict":
				a.calendarWritebackService().Record(source, "conflict", "This calendar changed locally and remotely before sync. Nothing was overwritten; the dashboard keeps showing its local version. Editing the event on your phone or deleting the dashboard copy resolves it.")
			case "attention-empty":
				a.calendarWritebackService().Record(source, "attention", "The last local event was removed while this calendar had unsynced remote changes. Sync is paused for safety; use Sync now after reviewing the calendar.")
			case "attention-undiscovered":
				a.calendarWritebackService().Record(source, "attention", "This selected calendar has not completed its one-time collection discovery. Re-add it from Calendar Manager or re-run private calendar setup.")
			case "attention-auth":
				a.calendarWritebackService().Record(source, "attention", "The provider rejected this calendar's credentials. Renew its app password or Google authorization, then use Sync now.")
			default:
				if timedOut {
					a.calendarWritebackService().Record(source, "waiting", "Saved locally; the selected remote sync timed out and will retry later.")
				} else {
					a.calendarWritebackService().Record(source, "waiting", "Saved locally; remote sync will retry automatically.")
				}
			}
			continue
		}
		a.calendarWritebackService().Record(source, "synced", "Saved locally and synchronized.")
	}
}
