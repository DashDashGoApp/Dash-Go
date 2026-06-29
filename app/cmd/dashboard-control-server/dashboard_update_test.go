package main

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func TestUpdateStateActive(t *testing.T) {
	for _, state := range []string{"preflight", "queued", "starting", "running", "validating-payload", "committing", "checking-runtime", "recycling-browser", "post-verify-pending"} {
		if !updateStateActive(state) {
			t.Fatalf("%q must be active", state)
		}
	}
	for _, state := range []string{"", "success", "rolledback", "failed", "cancelled"} {
		if updateStateActive(state) {
			t.Fatalf("%q must be terminal/inactive", state)
		}
	}
}

func TestCanonicalGitHubInstallerRejectsLegacyScriptAndAcceptsGitHubInstaller(t *testing.T) {
	dir := t.TempDir()
	installer := filepath.Join(dir, "install.sh")
	legacy := "#!/bin/bash\nBASE_URL=${BASE_URL:-}\nDASH_TRACK=beta\n"
	if err := os.WriteFile(installer, []byte(legacy), 0700); err != nil {
		t.Fatal(err)
	}
	if ready, detail := canonicalGitHubInstaller(installer); ready || !strings.Contains(detail, "legacy update script") {
		t.Fatalf("legacy installer ready=%v detail=%q", ready, detail)
	}
	github := "#!/bin/bash\nREPOSITORY=DashDashGoApp/Dash-Go\nDASH_TRACK=beta\ndownload_release_payload(){ :; }\n"
	if err := os.WriteFile(installer, []byte(github), 0700); err != nil {
		t.Fatal(err)
	}
	if ready, detail := canonicalGitHubInstaller(installer); !ready || detail != "" {
		t.Fatalf("GitHub installer ready=%v detail=%q", ready, detail)
	}
}

func TestUpdatePreflightDoesNotInventMetadataProblemsWhenCatalogFetchFails(t *testing.T) {
	a := testProfileApp(t)
	preflight := a.updatePreflightWithAvailability(map[string]any{
		"ok": false, "status": "forbidden", "label": "Catalog access denied",
		"detail": "Catalog fetch failed: HTTP Error 403. Release metadata was not retrieved.",
	})
	for _, raw := range preflight["problems"].([]string) {
		if strings.Contains(raw, "update metadata is missing") {
			t.Fatalf("catalog failure created an imaginary metadata error: %#v", preflight["problems"])
		}
	}
}

func TestUpdateProgressUsesOnlyTransactionRecords(t *testing.T) {
	a := testProfileApp(t)
	if err := writeJSONPrivateFile(a.updateJobPath(), map[string]any{
		"state": "committing", "label": "Committing update", "detail": "Replacing managed files.",
		"source": "control", "track": "beta", "target": "1.4.3-beta.93", "backup": "pre-update-test.zip",
	}); err != nil {
		t.Fatal(err)
	}
	if err := fileio.WriteJSON(filepath.Join(a.cacheDir, "update-status.json"), map[string]any{
		"state": "running", "label": "Runner status should not replace the job", "detail": "runner detail",
	}); err != nil {
		t.Fatal(err)
	}
	progress := a.updateProgress()
	if progress["active"] != true || progress["terminal"] != false {
		t.Fatalf("progress activity=%#v", progress)
	}
	if progress["state"] != "committing" || progress["label"] != "Committing update" {
		t.Fatalf("job record must be authoritative while present: %#v", progress)
	}
	if progress["backup"] != "pre-update-test.zip" || progress["target"] != "1.4.3-beta.93" {
		t.Fatalf("progress omitted safe transaction context: %#v", progress)
	}
	for _, forbidden := range []string{"preflight", "availability", "backups", "backupCount", "updaterUnit"} {
		if _, ok := progress[forbidden]; ok {
			t.Fatalf("lightweight progress payload must not perform full status work: found %q in %#v", forbidden, progress)
		}
	}
}

func TestUpdateProgressFallsBackToRunnerAndMarksTerminal(t *testing.T) {
	a := testProfileApp(t)
	if err := fileio.WriteJSON(filepath.Join(a.cacheDir, "update-status.json"), map[string]any{
		"state": "success", "label": "Update complete", "detail": "The dedicated updater completed successfully.", "source": "control",
	}); err != nil {
		t.Fatal(err)
	}
	progress := a.updateProgress()
	if progress["state"] != "success" || progress["terminal"] != true || progress["active"] != false {
		t.Fatalf("runner terminal status=%#v", progress)
	}
}

func holdDashboardUpdateLock(t *testing.T, a *app) *os.File {
	t.Helper()
	file, err := os.OpenFile(a.updateLockPath(), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		_ = file.Close()
	})
	return file
}

func TestUpdateLockUsesFlockRatherThanPersistentFilePresence(t *testing.T) {
	a := testProfileApp(t)
	if err := os.WriteFile(a.updateLockPath(), []byte("stable lock anchor\n"), 0600); err != nil {
		t.Fatal(err)
	}
	held, err := a.updateLockHeld()
	if err != nil || held {
		t.Fatalf("released update lock held=%v err=%v, want false nil", held, err)
	}
	holdDashboardUpdateLock(t, a)
	held, err = a.updateLockHeld()
	if err != nil || !held {
		t.Fatalf("locked update lock held=%v err=%v, want true nil", held, err)
	}
}

func TestInterruptedUpdateRecoveryMarksOnlyUnclaimedAgedJobFailed(t *testing.T) {
	a := testProfileApp(t)
	now := time.Unix(1_800_000_000, 0)
	old := now.Add(-updateInterruptedGrace - time.Second).Unix()
	initialJob := map[string]any{
		"id": "interrupted-update", "state": "running", "label": "Running update", "updatedAt": old,
		"source": "control", "target": "1.5.0-beta.22", "track": "beta",
	}
	if err := writeJSONPrivateFile(a.updateJobPath(), initialJob); err != nil {
		t.Fatal(err)
	}
	if err := a.recordUpdateAction(initialJob); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONPrivateFile(filepath.Join(a.cacheDir, "update-status.json"), map[string]any{
		"state": "running", "jobId": "interrupted-update", "updatedAt": old,
	}); err != nil {
		t.Fatal(err)
	}
	a.updateMu.Lock()
	recovered := a.reconcileInterruptedUpdateStateLockedWith(now, false, false)
	a.updateMu.Unlock()
	if !recovered {
		t.Fatal("aged unlocked job was not recovered")
	}
	job := a.readUpdateJob()
	if job["state"] != "failed" || job["label"] != "Interrupted update recovered" || job["interrupted"] != true {
		t.Fatalf("recovered job=%#v", job)
	}
	status := jsonutil.Map(a.readJSONDefault(filepath.Join(a.cacheDir, "update-status.json"), map[string]any{}))
	if status["state"] != "failed" || status["jobId"] != "interrupted-update" || status["interrupted"] != true {
		t.Fatalf("recovered status=%#v", status)
	}
	history := a.actionHistory(10)
	entries, ok := history["entries"].([]map[string]any)
	if !ok || len(entries) != 1 || entries[0]["state"] != "failed" || entries[0]["detail"] != job["detail"] {
		t.Fatalf("recovered action history=%#v", history)
	}
}

func TestInterruptedUpdateRecoveryPreservesDifferentStatusJob(t *testing.T) {
	a := testProfileApp(t)
	now := time.Unix(1_800_000_000, 0)
	old := now.Add(-updateInterruptedGrace - time.Second).Unix()
	job := map[string]any{"id": "abandoned", "state": "running", "updatedAt": old}
	if err := writeJSONPrivateFile(a.updateJobPath(), job); err != nil {
		t.Fatal(err)
	}
	if err := a.recordUpdateAction(job); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONPrivateFile(filepath.Join(a.cacheDir, "update-status.json"), map[string]any{
		"jobId": "newer", "state": "running", "detail": "newer transaction evidence", "updatedAt": now.Unix(),
	}); err != nil {
		t.Fatal(err)
	}
	a.updateMu.Lock()
	recovered := a.reconcileInterruptedUpdateStateLockedWith(now, false, false)
	a.updateMu.Unlock()
	if !recovered {
		t.Fatal("aged abandoned job was not recovered")
	}
	status := jsonutil.Map(a.readJSONDefault(filepath.Join(a.cacheDir, "update-status.json"), map[string]any{}))
	if status["jobId"] != "newer" || status["detail"] != "newer transaction evidence" || status["state"] != "running" {
		t.Fatalf("recovery overwrote newer status=%#v", status)
	}
}

func TestInterruptedUpdateRecoveryKeepsFreshLockedAndServiceOwnedJobs(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	cases := []struct {
		name       string
		updatedAt  int64
		lockHeld   bool
		unitActive bool
	}{
		{name: "fresh handoff", updatedAt: now.Unix()},
		{name: "live flock", updatedAt: now.Add(-updateInterruptedGrace - time.Second).Unix(), lockHeld: true},
		{name: "active unit", updatedAt: now.Add(-updateInterruptedGrace - time.Second).Unix(), unitActive: true},
		{name: "untimestamped legacy job", updatedAt: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := testProfileApp(t)
			if err := writeJSONPrivateFile(a.updateJobPath(), map[string]any{"id": "live", "state": "running", "updatedAt": tc.updatedAt}); err != nil {
				t.Fatal(err)
			}
			a.updateMu.Lock()
			recovered := a.reconcileInterruptedUpdateStateLockedWith(now, tc.lockHeld, tc.unitActive)
			a.updateMu.Unlock()
			if recovered {
				t.Fatalf("%s job was incorrectly recovered", tc.name)
			}
			if got := a.readUpdateJob()["state"]; got != "running" {
				t.Fatalf("%s state=%v, want running", tc.name, got)
			}
		})
	}
}

func TestGitHubReleaseCatalogProblemsAcceptCurrentReleaseMetadata(t *testing.T) {
	availability := map[string]any{
		"ok": true, "label": "Up to date", "detail": "1.5.0-beta.39 is current.",
		"releaseAsset": "Dash-Go_1.5.0-beta.39_release.tar.gz", "releaseDigest": currentTestDigest,
		"checksumsAsset": "SHA256SUMS", "checksumsDigest": currentTestDigest,
		"releaseUrl": "https://github.com/DashDashGoApp/Dash-Go/releases/tag/v1.5.0-beta.39", "immutable": true,
	}
	if problems := githubReleaseCatalogProblems(availability); len(problems) != 0 {
		t.Fatalf("current GitHub Release metadata must be ready, got %#v", problems)
	}
}

func TestGitHubReleaseCatalogProblemsRejectsRetiredCatalogShape(t *testing.T) {
	availability := map[string]any{
		"ok": true, "label": "Up to date", "detail": "current",
		// This is the old nginx metadata shape. It must never make a GitHub
		// Release updater ready merely because unrelated legacy fields exist.
		"tarball": "Dash-Go.tar.gz", "manifest": "MANIFEST.json", "shaPresent": true, "installerShaPresent": true,
	}
	problems := githubReleaseCatalogProblems(availability)
	if len(problems) == 0 {
		t.Fatal("retired catalog metadata unexpectedly passed GitHub Release preflight")
	}
	joined := strings.Join(problems, " | ")
	for _, want := range []string{"release bundle", "SHA256SUMS asset", "not immutable"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing GitHub Release preflight problem %q in %q", want, joined)
		}
	}
}

func TestUpdatePreflightTreatsTrackProfileAsInformationalNotCredentialGate(t *testing.T) {
	a := testProfileApp(t)
	availability := map[string]any{
		"ok": true, "label": "Up to date", "detail": "1.5.0-beta.39 is current.",
		"releaseAsset": "Dash-Go_1.5.0-beta.39_release.tar.gz", "releaseDigest": currentTestDigest,
		"checksumsAsset": "SHA256SUMS", "checksumsDigest": currentTestDigest,
		"releaseUrl": "https://github.com/DashDashGoApp/Dash-Go/releases/tag/v1.5.0-beta.39", "immutable": true,
	}
	preflight := a.updatePreflightWithAvailability(availability)
	if preflight["catalogReady"] != true {
		t.Fatalf("current GitHub Release metadata must be catalog-ready: %#v", preflight)
	}
	if preflight["updateTrackProfilePresent"] != false {
		t.Fatalf("missing optional track profile state=%v, want false", preflight["updateTrackProfilePresent"])
	}
	for _, problem := range preflight["problems"].([]string) {
		if strings.Contains(strings.ToLower(problem), "credential") {
			t.Fatalf("preflight retained a credential gate: %#v", preflight["problems"])
		}
	}
}
