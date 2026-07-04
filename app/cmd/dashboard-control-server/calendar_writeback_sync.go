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
	Automatic   bool
}

func (a *app) queueCalendarWritebackSync(source, pair string, automatic bool) {
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
	finalDelete, authorizationExists, err := a.finalDeletePermissionLocked(source, key, automatic)
	if err != nil {
		a.writebackSyncMu.Unlock()
		a.calendarWritebackService().Record(source, "attention", "Saved locally; Dash-Go could not verify the durable final-delete sync authorization. Existing dashboard events were kept.")
		return
	}
	if automatic && authorizationExists && !finalDelete {
		a.writebackSyncMu.Unlock()
		a.calendarWritebackService().Record(source, "attention", "Saved locally, but the verified final-event deletion did not complete after its bounded automatic retry. Review the remote calendar before deliberately retrying.")
		return
	}
	if a.writebackPending == nil {
		a.writebackPending = map[string]writebackPendingSync{}
	}
	// The final queued local state is authoritative. A later create, update, or
	// non-final delete clears the durable permission before reaching this queue.
	a.writebackPending[key] = writebackPendingSync{Source: source, FinalDelete: finalDelete, Automatic: automatic}
	if a.writebackSyncing {
		a.writebackSyncMu.Unlock()
		a.calendarWritebackService().Record(source, "syncing", "Saved locally; this calendar is queued behind the active private sync.")
		return
	}
	a.writebackSyncing = true
	a.writebackSyncMu.Unlock()
	go a.runCalendarWritebackQueue(script)
}

// writebackSyncArgs is the sole bridge from a verified post-mutation result to
// the wrapper's destructive --allow-empty-once flag. Callers must pass true
// only when the exact local vdir collection has no remaining .ics files.
func writebackSyncArgs(pair string, finalDelete bool) []string {
	pair = strings.TrimSpace(pair)
	if pair == "" || pair == "__legacy__" {
		return nil
	}
	args := []string{"--pair", pair}
	if finalDelete {
		args = append(args, "--allow-empty-once")
	}
	return args
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

// writebackSyncOutcome turns the wrapper's classified result into the exact
// per-calendar state shown in Dashboard Control. Classification is considered
// even when the process exits successfully: Google authorization can safely
// skip a pair with exit status 0, which is not a successful synchronization.
func writebackSyncOutcome(result string, commandFailed, timedOut, finalDelete bool, attempts int, automatic bool) (state, detail string, retry bool) {
	finalFailure := func() (string, string, bool) {
		if automatic && attempts < calendarWritebackFinalDeleteMaxAutomaticAttempts {
			return "waiting", "Saved locally; the verified final-event remote deletion did not complete. Dash-Go will retry this targeted deletion once while the local collection remains empty.", true
		}
		if automatic {
			return "attention", "Saved locally, but the verified final-event remote deletion did not complete after its bounded automatic retry. Review the remote calendar before deliberately retrying.", false
		}
		return "attention", "Saved locally, but the verified final-event remote deletion did not complete. Its durable retry authorization remains available only for a deliberate Sync now request.", false
	}
	switch result {
	case "skipped":
		return "attention", "Saved locally; provider authorization is required before this calendar can synchronize.", false
	case "conflict":
		return "conflict", "This calendar changed locally and remotely before sync. Nothing was overwritten; the dashboard keeps showing its local version. Editing the event on your phone or deleting the dashboard copy resolves it.", false
	case "attention-empty":
		return "attention", "The last local event was removed while this calendar had unsynced remote changes. Sync is paused for safety; use Sync now after reviewing the calendar.", false
	case "attention-undiscovered":
		return "attention", "This selected calendar has not completed its one-time collection discovery. Re-add it from Calendar Manager or re-run private calendar setup.", false
	case "attention-auth":
		return "attention", "The provider rejected this calendar's credentials. Renew its app password or Google authorization, then use Sync now.", false
	case "failed":
		if finalDelete {
			return finalFailure()
		}
		return "waiting", "Saved locally; remote sync failed safely. Existing dashboard events were kept.", false
	case "", "synced":
		// Continue below. An empty result preserves compatibility with an older
		// wrapper that exits 0 without structured RESULT rows.
	default:
		return "attention", "Saved locally; the selected remote sync returned an unrecognized safe result. Existing dashboard events were kept.", false
	}
	if commandFailed {
		if finalDelete {
			return finalFailure()
		}
		if timedOut {
			return "waiting", "Saved locally; the selected remote sync timed out and will retry later.", false
		}
		return "waiting", "Saved locally; remote sync will retry automatically.", false
	}
	// An older wrapper may not emit RESULT rows. Preserve its exit-0 contract,
	// but never let a known safe skip or unknown classification be reported as
	// synchronized.
	return "synced", "Saved locally and synchronized.", false
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

		armedFinalDelete := pending.FinalDelete
		attempts := 0
		if armedFinalDelete {
			var beginErr error
			armedFinalDelete, attempts, beginErr = a.beginFinalDeleteAttempt(source, key, pending.Automatic)
			if beginErr != nil {
				a.calendarWritebackService().Record(source, "attention", "Saved locally; Dash-Go could not durably prepare the verified final-delete sync. Existing dashboard events were kept.")
				continue
			}
		}
		a.calendarWritebackService().Record(source, "syncing", "Saved locally; synchronizing the selected private calendar.")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		output, err := exec.CommandContext(ctx, script, writebackSyncArgs(key, armedFinalDelete)...).CombinedOutput()
		timedOut := errors.Is(ctx.Err(), context.DeadlineExceeded)
		cancel()
		state, detail, retry := writebackSyncOutcome(writebackPairResult(string(output), key), err != nil, timedOut, armedFinalDelete, attempts, pending.Automatic)
		if armedFinalDelete && state == "synced" {
			if clearErr := a.clearFinalDeletePermission(source, key); clearErr != nil {
				state = "attention"
				detail = "The final-event remote deletion completed, but Dash-Go could not clear its local retry authorization. Review Calendar Manager before another sync."
				retry = false
			}
		}
		a.calendarWritebackService().Record(source, state, detail)
		if armedFinalDelete && retry && pending.Automatic {
			a.scheduleCalendarWritebackFinalDeleteRetry(source, key)
		}
	}
}
