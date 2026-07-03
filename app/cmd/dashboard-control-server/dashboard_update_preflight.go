package main

import (
	"maps"
	"os"
	"path/filepath"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// Update preflight is intentionally separate from the job starter. The
// Control card may check its state repeatedly; starting a job is a distinct,
// side-effecting transaction that must run only after this bounded review says
// a strict upgrade is available.

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

// githubReleaseCatalogProblems keeps Dashboard Control's install preflight in
// lockstep with the canonical GitHub Release resolver. The retired
// catalog exposed a tarball, manifest, and installer checksum separately;
// GitHub releases instead expose a self-contained versioned bundle and the
// release-wide SHA256SUMS asset. Resolver validation already proves exact asset
// names, upload state, URLs, and digest syntax. This narrow preflight confirms
// that its safe summary still has every field the installer transaction needs.
func githubReleaseCatalogProblems(availability map[string]any) []string {
	if availability["ok"] != true {
		return []string{strOr(availability["detail"], "GitHub Release discovery is unavailable")}
	}
	problems := []string{}
	for _, item := range []struct {
		field string
		label string
	}{
		{field: "releaseAsset", label: "release bundle"},
		{field: "releaseDigest", label: "release-bundle SHA-256"},
		{field: "checksumsAsset", label: "SHA256SUMS asset"},
		{field: "checksumsDigest", label: "SHA256SUMS SHA-256"},
		{field: "releaseUrl", label: "release URL"},
	} {
		if jsonutil.StringValue(availability[item.field]) == "" {
			problems = append(problems, "GitHub Release metadata is missing the "+item.label)
		}
	}
	if availability["immutable"] != true {
		problems = append(problems, "the resolved GitHub Release is not immutable")
	}
	return problems
}

func (a *app) updatePreflightWithAvailability(availability map[string]any) map[string]any {
	problems := []string{}
	installer := filepath.Join(a.home, "install.sh")
	installerPresent := fileio.Exists(installer)
	installerReady, installerDetail := canonicalGitHubInstaller(installer)
	trackProfilePresent := fileio.Exists(a.updateProfilePath())
	runnerPresent := executableRegularFile(a.updateRunnerPath())
	unit := a.updateUnitSnapshot()
	job := a.readUpdateJob()
	lockHeld, lockErr := a.updateLockHeld()
	if !installerReady {
		problems = append(problems, installerDetail)
	}
	// The selected track has a safe installed-version fallback and the installer
	// recreates its owner-only record on update. Its absence is informative but
	// never a credential gate or an automatic-update blocker.
	if !runnerPresent {
		problems = append(problems, "dedicated updater runner is missing; complete one SSH update or repair")
	}
	catalogProblems := githubReleaseCatalogProblems(availability)
	problems = append(problems, catalogProblems...)
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
	catalogReady := len(catalogProblems) == 0
	ready := len(problems) == 0
	strictUpgrade := availability["ok"] == true && availability["updateAvailable"] == true
	canStart := ready && strictUpgrade
	label, detail := "Ready", "Updater, safety backup, and selected GitHub Release metadata are ready."
	// A current/ahead device does not need an updater runner, a writable backup
	// location, or a startable systemd unit merely to report that it will not
	// replace itself. An active job still wins, because its state must remain
	// visible until it reaches a terminal result.
	switch {
	case updateActive:
		label, detail = "Update in progress", "An existing update job must finish before another can start."
	case availability["ok"] == true && !strictUpgrade:
		if availability["noDowngrade"] == true {
			label, detail = "No downgrade", strOr(availability["detail"], "Installed Dash-Go is newer than the selected release.")
		} else {
			label, detail = "No update needed", strOr(availability["detail"], "No newer release is available on the selected track.")
		}
	case !ready:
		if !catalogReady {
			label = strOr(availability["label"], "GitHub Release needs attention")
			detail = strOr(availability["detail"], problems[0])
		} else {
			label, detail = "Update setup needed", problems[0]
		}
	}
	return map[string]any{
		"ok": ready, "ready": ready, "canStart": canStart, "strictUpgrade": strictUpgrade, "catalogReady": catalogReady, "label": label, "detail": detail, "problems": problems,
		"installerPresent": installerPresent, "installerReady": installerReady, "updateTrackProfilePresent": trackProfilePresent, "runnerPresent": runnerPresent,
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

func dashboardNoUpdateResponse(availability map[string]any) map[string]any {
	label := "Already up to date"
	detail := strOr(availability["detail"], "No newer release is available on the selected track.")
	if availability["noDowngrade"] == true {
		label = "Installed version is newer"
	}
	return map[string]any{
		"started": false, "noUpdate": true, "label": label, "detail": detail,
		"installedVersion": strOr(availability["currentVersion"], ""), "targetVersion": strOr(availability["availableVersion"], ""),
		"track": strOr(availability["track"], ""), "noDowngrade": availability["noDowngrade"] == true,
	}
}
