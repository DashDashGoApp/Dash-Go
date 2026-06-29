package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
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

func (a *app) updateBackupWritable() (bool, string) {
	if err := a.ensureBackupDir(); err != nil {
		return false, err.Error()
	}
	probe, err := os.CreateTemp(a.backupDir(), ".update-preflight-")
	if err != nil {
		return false, err.Error()
	}
	name := probe.Name()
	if err := probe.Chmod(0600); err != nil {
		_ = probe.Close()
		_ = os.Remove(name)
		return false, err.Error()
	}
	_, err = probe.WriteString("ok\n")
	closeErr := probe.Close()
	removeErr := os.Remove(name)
	if err != nil {
		return false, err.Error()
	}
	if closeErr != nil {
		return false, closeErr.Error()
	}
	if removeErr != nil {
		return false, removeErr.Error()
	}
	return true, ""
}

func copyUpdateAvailability(value map[string]any) map[string]any {
	copy := make(map[string]any, len(value))
	maps.Copy(copy, value)
	return copy
}

// Dashboard Control polls update status while a job is active. Cache catalog
// checks briefly so a single progress view never turns into repeated network
// requests on a Pi; POST /api/update always forces a fresh preflight.
func (a *app) cachedUpdateAvailability(maxAge time.Duration) map[string]any {
	a.updateAvailabilityMu.Lock()
	defer a.updateAvailabilityMu.Unlock()
	if a.updateAvailabilityCache != nil && time.Since(a.updateAvailabilityAt) < maxAge {
		return copyUpdateAvailability(a.updateAvailabilityCache)
	}
	availability := a.checkUpdateAvailability()
	a.updateAvailabilityCache = copyUpdateAvailability(availability)
	a.updateAvailabilityAt = time.Now()
	return availability
}

func (a *app) updatePreflightWithAvailability(availability map[string]any) map[string]any {
	problems := []string{}
	installer := filepath.Join(a.home, "install.sh")
	installerPresent := fileio.Exists(installer)
	credentialsPresent := fileio.Exists(a.updateProfilePath())
	runnerPresent := executableRegularFile(a.updateRunnerPath())
	unit := a.updateUnitSnapshot()
	job := a.readUpdateJob()
	lockHeld, lockErr := a.updateLockHeld()
	if !installerPresent {
		problems = append(problems, "installer is missing from the dashboard account home directory")
	}
	if !credentialsPresent {
		problems = append(problems, "saved update credentials are missing")
	}
	if !runnerPresent {
		problems = append(problems, "dedicated updater runner is missing; complete one SSH update or repair")
	}
	if availability["ok"] != true {
		// A failed catalog request has no metadata to inspect. Reporting absent
		// tarball/hash fields here turns one actionable network/auth failure into
		// a misleading cascade of imaginary packaging defects.
		problems = append(problems, strOr(availability["detail"], "update catalog is unavailable"))
	} else {
		for _, field := range []string{"tarball", "manifest"} {
			if jsonutil.StringValue(availability[field]) == "" {
				problems = append(problems, "update metadata is missing "+field)
			}
		}
		if availability["shaPresent"] != true {
			problems = append(problems, "update metadata is missing the release SHA256")
		}
		if availability["installerShaPresent"] != true {
			problems = append(problems, "update metadata is missing the shared installer SHA256")
		}
	}
	if unit["present"] != true {
		problems = append(problems, strOr(unit["detail"], "dedicated updater service is missing"))
	}
	if unit["sudoReady"] != true {
		problems = append(problems, "Dashboard Control cannot start the dedicated updater service without a password")
	}
	lockProbeError := ""
	if lockErr != nil {
		lockProbeError = lockErr.Error()
		problems = append(problems, "could not inspect the update lock: "+lockProbeError)
	}
	updateActive := updateStateActive(strOr(job["state"], "")) || unit["active"] == true || lockHeld
	if updateActive {
		problems = append(problems, "an update is already running")
	}
	backupWritable, backupDetail := a.updateBackupWritable()
	if !backupWritable {
		problems = append(problems, "backup storage is not writable: "+backupDetail)
	}
	catalogReady := availability["ok"] == true
	if catalogReady {
		for _, field := range []string{"tarball", "manifest"} {
			if jsonutil.StringValue(availability[field]) == "" {
				catalogReady = false
			}
		}
		if availability["shaPresent"] != true || availability["installerShaPresent"] != true {
			catalogReady = false
		}
	}
	ready := len(problems) == 0
	label, detail := "Ready", "Updater, safety backup, and selected release metadata are ready."
	if !ready {
		switch {
		case updateActive:
			label, detail = "Update in progress", "An existing update job must finish before another can start."
		case !catalogReady:
			label = strOr(availability["label"], "Update source needs attention")
			detail = strOr(availability["detail"], problems[0])
		default:
			label, detail = "Update setup needed", problems[0]
		}
	}
	return map[string]any{
		"ok": ready, "ready": ready, "catalogReady": catalogReady, "label": label, "detail": detail, "problems": problems,
		"installerPresent": installerPresent, "credentialsPresent": credentialsPresent, "runnerPresent": runnerPresent,
		"backupWritable": backupWritable, "availability": availability, "unit": unit, "job": job, "lockHeld": lockHeld,
		"lockProbeError": lockProbeError,
	}
}

func (a *app) updatePreflight() map[string]any {
	return a.updatePreflightWithAvailability(a.cachedUpdateAvailability(30 * time.Second))
}

func (a *app) updatePreflightFresh() map[string]any {
	availability := a.checkUpdateAvailability()
	a.updateAvailabilityMu.Lock()
	a.updateAvailabilityCache = copyUpdateAvailability(availability)
	a.updateAvailabilityAt = time.Now()
	a.updateAvailabilityMu.Unlock()
	return a.updatePreflightWithAvailability(availability)
}

func (a *app) startDashboardUpdate() (map[string]any, error) {
	a.updateMu.Lock()
	defer a.updateMu.Unlock()
	a.reconcileInterruptedUpdateStateLocked()
	preflight := a.updatePreflightFresh()
	if preflight["ok"] != true {
		if updateStateActive(strOr(jsonutil.Map(preflight["job"])["state"], "")) || jsonutil.Map(preflight["unit"])["active"] == true || preflight["lockHeld"] == true {
			return nil, errDashboardUpdateRunning
		}
		return nil, dashboardUpdatePreflightError{detail: strOr(preflight["detail"], "update preflight failed")}
	}
	backup, err := a.createConfigBackup("pre-update", "Automatic backup before dashboard update", "update", true)
	if err != nil {
		return nil, fmt.Errorf("safety backup failed: %w", err)
	}
	job := map[string]any{
		"id": updateID(), "state": "queued", "label": "Queued", "detail": "Safety backup verified; waiting for the dedicated updater service.",
		"source": "control", "track": strOr(jsonutil.Map(preflight["availability"])["track"], ""), "installedVersion": fileio.ReadString(filepath.Join(a.dash, "VERSION"), ""),
		"target": firstNonEmpty(strOr(jsonutil.Map(preflight["availability"])["availableVersion"], ""), "latest"), "backup": backup["name"], "backupFiles": backup["validatedFiles"],
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
