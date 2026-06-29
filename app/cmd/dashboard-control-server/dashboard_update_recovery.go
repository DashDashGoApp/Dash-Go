package main

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// updateInterruptedGrace leaves the control-launched handoff enough time to
// create the durable job and for systemd to start the dedicated runner. After
// that window, an active job without either the cross-process flock or an
// active updater service is abandoned state, not a live transaction.
const updateInterruptedGrace = 2 * time.Minute

// updateLockHeld probes the stable flock anchor without treating the lock file
// itself as state. update.lock intentionally remains after a process exits;
// deleting or renaming it would split lock ownership across inodes and could
// permit overlapping update transactions.
func (a *app) updateLockHeld() (bool, error) {
	path := a.updateLockPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return false, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return false, err
	}
	defer file.Close()
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return true, nil
		}
		return false, err
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	return false, nil
}

func updateJobLastActivity(job map[string]any) int64 {
	if at := anyInt64(job["updatedAt"], 0); at > 0 {
		return at
	}
	return anyInt64(job["requestedAt"], 0)
}

func updateJobRecoveryDue(job map[string]any, now time.Time, lockHeld, unitActive bool) bool {
	if !updateStateActive(strOr(job["state"], "")) || lockHeld || unitActive {
		return false
	}
	at := updateJobLastActivity(job)
	if at <= 0 {
		// Older/malformed records lack enough evidence to distinguish a slow
		// handoff from an abandoned transaction. Preserve them for explicit
		// repair rather than guessing and starting a second update.
		return false
	}
	return !now.Before(time.Unix(at, 0).Add(updateInterruptedGrace))
}

func (a *app) reconcileInterruptedUpdateState() {
	a.updateMu.Lock()
	defer a.updateMu.Unlock()
	_ = a.reconcileInterruptedUpdateStateLocked()
}

func (a *app) reconcileInterruptedUpdateStateLocked() bool {
	job := a.readUpdateJob()
	if !updateStateActive(strOr(job["state"], "")) {
		return false
	}
	if !updateJobRecoveryDue(job, time.Now(), false, false) {
		return false
	}
	lockHeld, err := a.updateLockHeld()
	if err != nil {
		log.Printf("could not inspect Dash-Go update lock before interrupted-job recovery: %v", err)
		return false
	}
	unit := a.updateUnitSnapshot()
	return a.reconcileInterruptedUpdateStateLockedWith(time.Now(), lockHeld, unit["active"] == true)
}

// reconcileInterruptedUpdateStateLockedWith contains the durable transition so
// focused tests can exercise crash recovery without needing systemd. Caller
// must hold updateMu whenever this may race another local Control mutation.
func (a *app) reconcileInterruptedUpdateStateLockedWith(now time.Time, lockHeld, unitActive bool) bool {
	job := a.readUpdateJob()
	if !updateJobRecoveryDue(job, now, lockHeld, unitActive) {
		return false
	}
	jobID := strings.TrimSpace(strOr(job["id"], ""))
	detail := "Recovered an interrupted update: its durable job remained active after the handoff grace period, but no updater lock or active dedicated updater service remained."
	job["state"] = "failed"
	job["label"] = "Interrupted update recovered"
	job["detail"] = detail
	job["exitCode"] = 1
	job["interrupted"] = true
	job["recoveredAt"] = now.Unix()
	if err := a.writeUpdateJob(job); err != nil {
		log.Printf("could not persist interrupted Dash-Go update recovery: %v", err)
		return false
	}
	if err := a.writeInterruptedUpdateStatus(job, detail, now); err != nil {
		log.Printf("could not persist interrupted Dash-Go update status recovery: %v", err)
	}
	if err := a.finalizeUpdateActionHistory(job); err != nil && jobID != "" {
		log.Printf("could not finalize interrupted Dash-Go update action %s: %v", jobID, err)
	}
	log.Printf("recovered interrupted Dash-Go update job %s", firstNonEmpty(jobID, "(unnamed)"))
	return true
}

func (a *app) writeInterruptedUpdateStatus(job map[string]any, detail string, now time.Time) error {
	path := filepath.Join(a.cacheDir, "update-status.json")
	status := jsonutil.Map(a.readJSONDefault(path, map[string]any{}))
	jobID := strings.TrimSpace(strOr(job["id"], ""))
	statusJobID := strings.TrimSpace(strOr(status["jobId"], ""))
	if statusJobID != "" && jobID != "" && statusJobID != jobID {
		// A later transaction has already published a different durable status.
		// Preserve it rather than letting old crash recovery overwrite it.
		return nil
	}
	if statusJobID == "" && status["state"] != nil && !updateStateActive(strOr(status["state"], "")) {
		// A terminal status without an identity may be a legacy/later outcome.
		// The recovered job is still authoritative for action history, but avoid
		// replacing unrelated terminal evidence.
		return nil
	}
	status["schema"] = 1
	status["state"] = "failed"
	status["label"] = "Interrupted update recovered"
	status["detail"] = detail
	status["exitCode"] = 1
	status["interrupted"] = true
	status["recoveredAt"] = now.Unix()
	status["updatedAt"] = now.Unix()
	if jobID != "" {
		status["jobId"] = jobID
	}
	for _, key := range []string{"source", "target", "track", "version", "previousVersion"} {
		if value := job[key]; value != nil && strOr(value, "") != "" {
			status[key] = value
		}
	}
	return writeJSONPrivateFile(path, status)
}
