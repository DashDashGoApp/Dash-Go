package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

var errDashboardUpdateRunning = errors.New("dashboard update is already running")

type dashboardUpdatePreflightError struct{ detail string }

func (e dashboardUpdatePreflightError) Error() string { return e.detail }

// dashboardUpdateActionRecordedError marks failures that occur after the
// job-linked Recent Actions row exists. The HTTP route must update that same
// row instead of appending a duplicate terminal entry.
type dashboardUpdateActionRecordedError struct{ err error }

func (e dashboardUpdateActionRecordedError) Error() string { return e.err.Error() }
func (e dashboardUpdateActionRecordedError) Unwrap() error { return e.err }

func updateActionAlreadyRecorded(err error) bool {
	var recorded dashboardUpdateActionRecordedError
	return errors.As(err, &recorded)
}

func (a *app) updateJobPath() string    { return filepath.Join(a.cacheDir, "update-job.json") }
func (a *app) updateLockPath() string   { return filepath.Join(a.cacheDir, "update.lock") }
func (a *app) updateRunnerPath() string { return filepath.Join(a.binDir, "dashboard-update-runner.sh") }

func executableRegularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0
}

// canonicalGitHubInstaller prevents Dashboard Control from reporting a false
// ready state when ~/install.sh is a retained legacy updater. The dedicated
// runner executes that home-side file, so presence alone is insufficient.
func canonicalGitHubInstaller(path string) (bool, string) {
	if !executableRegularFile(path) {
		return false, "the canonical installer is missing or is not executable"
	}
	f, err := os.Open(path)
	if err != nil {
		return false, "the canonical installer could not be read"
	}
	defer f.Close()
	body, err := io.ReadAll(io.LimitReader(f, 1024*1024))
	if err != nil {
		return false, "the canonical installer could not be read"
	}
	source := string(body)
	for _, marker := range []string{"DashDashGoApp/Dash-Go", "download_release_payload(){", "DASH_TRACK"} {
		if !strings.Contains(source, marker) {
			return false, "the canonical installer is still a legacy update script; install a verified Dash-Go GitHub Release bundle once"
		}
	}
	return true, ""
}

func updateStateActive(state string) bool {
	switch state {
	case "preflight", "queued", "starting", "running", "validating-payload", "committing", "checking-runtime", "recycling-browser", "post-verify-pending", "rollback-requested":
		return true
	default:
		return false
	}
}

func (a *app) readUpdateJob() map[string]any {
	return jsonutil.Map(a.readJSONDefault(a.updateJobPath(), map[string]any{}))
}

func updateID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err == nil {
		return fmt.Sprintf("update-%d-%s", time.Now().Unix(), hex.EncodeToString(buf))
	}
	return fmt.Sprintf("update-%d", time.Now().UnixNano())
}

func (a *app) writeUpdateJob(job map[string]any) error {
	if job == nil {
		job = map[string]any{}
	}
	if _, ok := job["schema"]; !ok {
		job["schema"] = 1
	}
	if _, ok := job["requestedAt"]; !ok {
		job["requestedAt"] = time.Now().Unix()
	}
	job["updatedAt"] = time.Now().Unix()
	return writeJSONPrivateFile(a.updateJobPath(), job)
}

func (a *app) updateUnitSnapshot() map[string]any {
	out := map[string]any{"present": false, "sudoReady": false, "active": false, "state": "unknown", "result": "unknown", "mainPID": 0}
	path, err := exec.LookPath("systemctl")
	if err != nil {
		out["detail"] = "systemctl is missing"
		return out
	}
	out["systemctl"] = path
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sudo", "-n", path, "show", "dash-go-update.service")
	data, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			out["detail"] = "dedicated updater service query timed out"
		} else {
			out["detail"] = "Dashboard Control is not permitted to query the dedicated updater service"
		}
		return out
	}
	out["sudoReady"] = true
	for line := range strings.SplitSeq(string(data), "\n") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		switch parts[0] {
		case "LoadState":
			out["loadState"] = parts[1]
			out["present"] = parts[1] == "loaded"
		case "ActiveState":
			out["state"] = parts[1]
			out["active"] = parts[1] == "active" || parts[1] == "activating"
		case "SubState":
			out["subState"] = parts[1]
		case "Result":
			out["result"] = parts[1]
		case "MainPID":
			out["mainPID"] = jsonutil.Int(parts[1], 0)
		case "ExecMainStatus":
			out["exitCode"] = jsonutil.Int(parts[1], 0)
		}
	}
	if out["present"] != true && out["detail"] == nil {
		out["detail"] = "dedicated updater service is not installed"
	}
	return out
}

func (a *app) startDashboardUpdate() (map[string]any, error) {
	a.updateMu.Lock()
	defer a.updateMu.Unlock()
	a.reconcileInterruptedUpdateStateLocked()
	preflight := a.updatePreflightFresh()
	availability := jsonutil.Map(preflight["availability"])
	if updateStateActive(strOr(jsonutil.Map(preflight["job"])["state"], "")) || jsonutil.Map(preflight["unit"])["active"] == true || preflight["lockHeld"] == true {
		return nil, errDashboardUpdateRunning
	}
	// The API is authoritative even when a stale browser still shows an armed
	// button. Do not create a backup, update job, action-history row, systemd
	// request, payload stage, or kiosk restart unless a strict/recovery upgrade
	// is still available at the moment this POST is handled.
	if availability["ok"] == true && availability["updateAvailable"] != true {
		return dashboardNoUpdateResponse(availability), nil
	}
	if preflight["ready"] != true || preflight["canStart"] != true {
		return nil, dashboardUpdatePreflightError{detail: strOr(preflight["detail"], "update preflight failed")}
	}
	backup, err := a.createConfigBackup("pre-update", "Automatic backup before dashboard update", "update", true)
	if err != nil {
		return nil, fmt.Errorf("safety backup failed: %w", err)
	}
	job := map[string]any{
		"id": updateID(), "state": "queued", "label": "Queued", "detail": "Safety backup verified; waiting for the dedicated updater service.",
		"source": "control", "track": strOr(availability["track"], ""), "installedVersion": fileio.ReadString(filepath.Join(a.dash, "VERSION"), ""),
		"target": firstNonEmpty(strOr(availability["availableVersion"], ""), "latest"), "backup": backup["name"], "backupFiles": backup["validatedFiles"],
		"unit": "dash-go-update.service", "logFile": filepath.Join(a.logDir, "update.log"),
	}
	if err := a.writeUpdateJob(job); err != nil {
		return nil, fmt.Errorf("could not record update job: %w", err)
	}
	// Persist the linked Recent Actions row before systemd can run the updater.
	// The runner later finalizes this same row from the durable job record.
	if err := a.recordUpdateAction(job); err != nil {
		job["state"] = "failed"
		job["label"] = "Could not record update history"
		job["detail"] = "The update did not start because its durable Recent Actions record could not be written: " + err.Error()
		job["exitCode"] = 1
		_ = a.writeUpdateJob(job)
		return nil, dashboardUpdatePreflightError{detail: "could not record update history: " + err.Error()}
	}
	unit := jsonutil.Map(preflight["unit"])
	systemctlPath := strOr(unit["systemctl"], "")
	if systemctlPath == "" {
		return nil, dashboardUpdatePreflightError{detail: "systemctl is unavailable"}
	}
	cmd := exec.Command("sudo", "-n", systemctlPath, "start", "--no-block", "dash-go-update.service")
	if output, err := cmd.CombinedOutput(); err != nil {
		job["state"] = "failed"
		job["label"] = "Could not start updater"
		job["detail"] = strings.TrimSpace(string(output))
		job["exitCode"] = 1
		_ = a.writeUpdateJob(job)
		_ = a.finalizeUpdateActionHistory(job)
		return nil, dashboardUpdateActionRecordedError{err: fmt.Errorf("could not start the dedicated updater service: %w", err)}
	}
	job["state"] = "starting"
	job["label"] = "Starting updater"
	job["detail"] = "Dedicated updater service accepted the job."
	_ = a.writeUpdateJob(job)
	return map[string]any{"started": true, "job": job, "preBackup": backup["name"], "targetVersion": job["target"]}, nil
}
