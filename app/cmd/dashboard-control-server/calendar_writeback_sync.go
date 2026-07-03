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
	// The final queued local state is authoritative. In particular, a create
	// after a final delete must clear the destructive one-run permission rather
	// than inherit it from the earlier mutation.
	a.writebackPending[key] = writebackPendingSync{Source: source, FinalDelete: finalDelete}
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
func writebackSyncOutcome(result string, commandFailed, timedOut, finalDelete bool) (state, detail string) {
	switch result {
	case "skipped":
		return "attention", "Saved locally; provider authorization is required before this calendar can synchronize."
	case "conflict":
		return "conflict", "This calendar changed locally and remotely before sync. Nothing was overwritten; the dashboard keeps showing its local version. Editing the event on your phone or deleting the dashboard copy resolves it."
	case "attention-empty":
		return "attention", "The last local event was removed while this calendar had unsynced remote changes. Sync is paused for safety; use Sync now after reviewing the calendar."
	case "attention-undiscovered":
		return "attention", "This selected calendar has not completed its one-time collection discovery. Re-add it from Calendar Manager or re-run private calendar setup."
	case "attention-auth":
		return "attention", "The provider rejected this calendar's credentials. Renew its app password or Google authorization, then use Sync now."
	case "failed":
		if finalDelete {
			return "attention", "Saved locally, but the final-event remote deletion did not complete. Dash-Go will not repeat the one-time empty-collection override automatically; review the remote calendar before deliberately retrying that deletion."
		}
		return "waiting", "Saved locally; remote sync failed safely. Existing dashboard events were kept."
	case "", "synced":
		// Continue below. An empty result preserves compatibility with an older
		// wrapper that exits 0 without structured RESULT rows.
	default:
		return "attention", "Saved locally; the selected remote sync returned an unrecognized safe result. Existing dashboard events were kept."
	}
	if commandFailed {
		if finalDelete {
			return "attention", "Saved locally, but the final-event remote deletion did not complete. Dash-Go will not repeat the one-time empty-collection override automatically; review the remote calendar before deliberately retrying that deletion."
		}
		if timedOut {
			return "waiting", "Saved locally; the selected remote sync timed out and will retry later."
		}
		return "waiting", "Saved locally; remote sync will retry automatically."
	}
	// An older wrapper may not emit RESULT rows. Preserve its exit-0 contract,
	// but never let a known safe skip or unknown classification be reported as
	// synchronized.
	return "synced", "Saved locally and synchronized."
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
		output, err := exec.CommandContext(ctx, script, writebackSyncArgs(key, pending.FinalDelete)...).CombinedOutput()
		timedOut := errors.Is(ctx.Err(), context.DeadlineExceeded)
		cancel()
		state, detail := writebackSyncOutcome(writebackPairResult(string(output), key), err != nil, timedOut, pending.FinalDelete)
		a.calendarWritebackService().Record(source, state, detail)
	}
}
