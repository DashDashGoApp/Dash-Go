package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
)

const actionHistoryLimit = 100

func (a *app) actionHistoryPath() string       { return filepath.Join(a.cacheDir, "action-history.json") }
func actionHistoryLockPath(path string) string { return path + ".lock" }

// withActionHistoryLock serializes read-modify-write cycles across the dashboard
// server and the dedicated updater runner. The history itself is still written
// through fileio.WriteJSON's same-directory atomic rename path.
func withActionHistoryLock(path string, fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	lock, err := os.OpenFile(actionHistoryLockPath(path), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := lock.Chmod(0600); err != nil {
		return err
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	return fn()
}

func actionHistoryMaps(raw any) []map[string]any {
	values := []any{}
	switch value := raw.(type) {
	case []any:
		values = value
	case map[string]any:
		if listed, ok := value["entries"].([]any); ok {
			values = listed
		} else if listed, ok := value["items"].([]any); ok {
			values = listed
		}
	}
	out := make([]map[string]any, 0, len(values))
	for _, value := range values {
		if item, ok := value.(map[string]any); ok && item != nil {
			out = append(out, item)
		}
	}
	return out
}

func readActionHistoryFile(path string) ([]map[string]any, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return []map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		// Preserve the dashboard's historical best-effort behavior: a damaged
		// maintenance log must never prevent Dashboard Control from opening.
		return []map[string]any{}, nil
	}
	return actionHistoryMaps(raw), nil
}

func readActionHistoryLocked(path string) ([]map[string]any, error) {
	var entries []map[string]any
	err := withActionHistoryLock(path, func() error {
		var err error
		entries, err = readActionHistoryFile(path)
		return err
	})
	return entries, err
}

func mutateActionHistoryFile(path string, mutate func(*[]map[string]any) (bool, error)) error {
	return withActionHistoryLock(path, func() error {
		entries, err := readActionHistoryFile(path)
		if err != nil {
			return err
		}
		changed, err := mutate(&entries)
		if err != nil || !changed {
			return err
		}
		if len(entries) > actionHistoryLimit {
			entries = entries[:actionHistoryLimit]
		}
		return fileio.WriteJSON(path, entries)
	})
}

func actionHistoryMeta(entry map[string]any) map[string]any {
	if meta, ok := entry["meta"].(map[string]any); ok && meta != nil {
		return meta
	}
	return map[string]any{}
}

func actionHistoryUpdateJobID(entry map[string]any) string {
	return strings.TrimSpace(strOr(actionHistoryMeta(entry)["updateJobId"], ""))
}

func updateActionEntryID(jobID string) string { return "update:" + jobID }

func updateActionMetadata(job map[string]any) map[string]any {
	meta := map[string]any{"updateJobId": strings.TrimSpace(strOr(job["id"], ""))}
	for _, key := range []string{"track", "target", "backup", "source"} {
		if value := strings.TrimSpace(strOr(job[key], "")); value != "" {
			meta[key] = value
		}
	}
	return meta
}

func updateActionStartDetail(job map[string]any) string {
	jobID := strings.TrimSpace(strOr(job["id"], ""))
	if jobID == "" {
		return "Dedicated updater service accepted the update job."
	}
	return "Dedicated updater service accepted update job " + jobID
}

func updateActionTerminalDetail(job map[string]any) string {
	state := strings.ToLower(strings.TrimSpace(strOr(job["state"], "")))
	detail := strings.TrimSpace(strOr(job["detail"], ""))
	if detail != "" {
		return detail
	}
	switch state {
	case "success":
		if version := firstNonEmpty(strOr(job["version"], ""), strOr(job["target"], "")); version != "" {
			return "Installed " + version + "; bounded runtime verification passed."
		}
		return "Installed release; bounded runtime verification passed."
	case "rolledback":
		return "The update did not pass verification; the prior release was restored."
	case "failed":
		return "The update failed. Review the update log for the retained transaction evidence."
	default:
		return "Update finished with an unrecognized terminal state. Review the update log."
	}
}

func updateActionTerminal(state string) bool {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "success", "rolledback", "failed":
		return true
	default:
		return false
	}
}

func actionHistoryTime(job map[string]any, key string, fallback int64) int64 {
	if value := anyInt64(job[key], 0); value > 0 {
		return value
	}
	return fallback
}

func (a *app) actionHistory(limit int) map[string]any {
	// The runner can be interrupted as the dashboard server is replaced. Reconcile
	// the durable terminal job before every history response as a second safety net.
	a.reconcileUpdateActionHistory()
	if limit <= 0 {
		limit = 25
	}
	limit = min(limit, actionHistoryLimit)
	entries, err := readActionHistoryLocked(a.actionHistoryPath())
	if err != nil {
		entries = []map[string]any{}
	}
	shown := entries
	if len(shown) > limit {
		shown = shown[:limit]
	}
	return map[string]any{"entries": shown, "count": len(entries), "file": a.actionHistoryPath()}
}

func (a *app) recordAction(kind, label, state, detail string, meta map[string]any) {
	entry := map[string]any{"at": time.Now().Unix(), "kind": kind, "label": label, "state": state, "detail": detail}
	if len(meta) > 0 {
		entry["meta"] = meta
	}
	_ = mutateActionHistoryFile(a.actionHistoryPath(), func(entries *[]map[string]any) (bool, error) {
		*entries = append([]map[string]any{entry}, (*entries)...)
		return true, nil
	})
}

// recordUpdateAction creates the single job-linked history row before the
// dedicated service starts. Repeating it is idempotent and never appends a
// second row for the same durable update job ID.
func (a *app) recordUpdateAction(job map[string]any) error {
	jobID := strings.TrimSpace(strOr(job["id"], ""))
	if jobID == "" {
		return errors.New("update job is missing an identity")
	}
	return mutateActionHistoryFile(a.actionHistoryPath(), func(entries *[]map[string]any) (bool, error) {
		for _, entry := range *entries {
			if entry["id"] != updateActionEntryID(jobID) && actionHistoryUpdateJobID(entry) != jobID {
				continue
			}
			if updateActionTerminal(strOr(entry["state"], "")) {
				return false, nil
			}
			entry["state"] = "running"
			entry["detail"] = updateActionStartDetail(job)
			entry["updatedAt"] = time.Now().Unix()
			entry["meta"] = updateActionMetadata(job)
			return true, nil
		}
		entry := map[string]any{
			"id":     updateActionEntryID(jobID),
			"at":     actionHistoryTime(job, "requestedAt", time.Now().Unix()),
			"kind":   "update",
			"label":  "Update dashboard",
			"state":  "running",
			"detail": updateActionStartDetail(job),
			"meta":   updateActionMetadata(job),
		}
		*entries = append([]map[string]any{entry}, (*entries)...)
		return true, nil
	})
}

func applyTerminalUpdateAction(entries []map[string]any, job map[string]any) (bool, bool) {
	jobID := strings.TrimSpace(strOr(job["id"], ""))
	state := strings.ToLower(strings.TrimSpace(strOr(job["state"], "")))
	if jobID == "" || !updateActionTerminal(state) {
		return false, false
	}
	for _, entry := range entries {
		if entry["id"] != updateActionEntryID(jobID) && actionHistoryUpdateJobID(entry) != jobID {
			continue
		}
		if updateActionTerminal(strOr(entry["state"], "")) {
			return false, true
		}
		entry["state"] = state
		entry["detail"] = updateActionTerminalDetail(job)
		entry["completedAt"] = actionHistoryTime(job, "updatedAt", time.Now().Unix())
		entry["updatedAt"] = time.Now().Unix()
		entry["meta"] = updateActionMetadata(job)
		return true, true
	}
	return false, false
}

func finalizeUpdateActionHistoryFile(historyPath string, job map[string]any) (bool, error) {
	changed := false
	err := mutateActionHistoryFile(historyPath, func(entries *[]map[string]any) (bool, error) {
		var found bool
		changed, found = applyTerminalUpdateAction(*entries, job)
		if !found {
			return false, fmt.Errorf("matching update action history entry was not found")
		}
		return changed, nil
	})
	return changed, err
}

func reconcileUpdateActionHistoryFile(historyPath string, job map[string]any) (bool, error) {
	changed := false
	err := mutateActionHistoryFile(historyPath, func(entries *[]map[string]any) (bool, error) {
		if terminalChanged, _ := applyTerminalUpdateAction(*entries, job); terminalChanged {
			changed = true
		}
		return changed, nil
	})
	return changed, err
}

func (a *app) finalizeUpdateActionHistory(job map[string]any) error {
	_, err := finalizeUpdateActionHistoryFile(a.actionHistoryPath(), job)
	return err
}

func (a *app) reconcileUpdateActionHistory() {
	_, _ = reconcileUpdateActionHistoryFile(a.actionHistoryPath(), a.readUpdateJob())
}

func (a *app) runFinalizeUpdateActionCLI(args []string) int {
	fs := flag.NewFlagSet("finalize-update-action", flag.ContinueOnError)
	historyPath := fs.String("history", "", "action history JSON file")
	jobPath := fs.String("job", "", "update job JSON file")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 {
		fmt.Fprintln(fs.Output(), "usage: --finalize-update-action --history FILE --job FILE")
		return 64
	}
	if strings.TrimSpace(*historyPath) == "" || strings.TrimSpace(*jobPath) == "" {
		fmt.Fprintln(fs.Output(), "--history and --job required")
		return 64
	}
	data, err := os.ReadFile(*jobPath)
	if err != nil {
		fmt.Fprintln(fs.Output(), err)
		return 1
	}
	job := map[string]any{}
	if err := json.Unmarshal(data, &job); err != nil || len(job) == 0 {
		if err == nil {
			err = errors.New("update job record is empty")
		}
		fmt.Fprintln(fs.Output(), err)
		return 1
	}
	if _, err := finalizeUpdateActionHistoryFile(*historyPath, job); err != nil {
		fmt.Fprintln(fs.Output(), err)
		return 1
	}
	return 0
}
