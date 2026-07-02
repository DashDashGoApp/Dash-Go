package main

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func testSystemUpdateApp(t *testing.T) *app {
	t.Helper()
	dash := t.TempDir()
	a := &app{dash: dash, cacheDir: filepath.Join(dash, "cache"), logDir: filepath.Join(dash, "logs"), binDir: filepath.Join(dash, "bin")}
	a.ensureDirs()
	if err := fileio.WriteAtomic(filepath.Join(a.binDir, "dashboard-system-update.sh"), []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}
	return a
}

func writeSystemStatusForTest(t *testing.T, a *app, payload map[string]any) {
	t.Helper()
	if err := fileio.WriteJSON(filepath.Join(a.cacheDir, "system-update-status.json"), payload); err != nil {
		t.Fatal(err)
	}
}

func TestSystemUpdateStatusKeepsLiveLockRunning(t *testing.T) {
	a := testSystemUpdateApp(t)
	lock := filepath.Join(a.cacheDir, "system-update.lock")
	if err := os.MkdirAll(lock, 0700); err != nil {
		t.Fatal(err)
	}
	if err := fileio.WriteAtomic(filepath.Join(lock, "pid"), []byte(strconv.Itoa(os.Getpid())+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	writeSystemStatusForTest(t, a, map[string]any{"state": "running", "updatedAt": time.Now().Add(-10 * time.Minute).Unix(), "bootId": systemBootID()})
	got := a.systemUpdateStatus()
	if got["running"] != true || got["lockRunning"] != true || got["state"] != "running" {
		t.Fatalf("live lock status=%#v", got)
	}
}

func TestSystemUpdateStatusRecoversAgedDeadOwner(t *testing.T) {
	a := testSystemUpdateApp(t)
	lock := filepath.Join(a.cacheDir, "system-update.lock")
	if err := os.MkdirAll(lock, 0700); err != nil {
		t.Fatal(err)
	}
	if err := fileio.WriteAtomic(filepath.Join(lock, "pid"), []byte("999999\n"), 0600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-systemUpdateInterruptedGrace - time.Second).Unix()
	writeSystemStatusForTest(t, a, map[string]any{"state": "running", "updatedAt": old, "startedAt": old, "bootId": systemBootID(), "commandPid": 999999})
	got := a.systemUpdateStatus()
	if got["running"] != false || got["state"] != "failed" || got["interrupted"] != true {
		t.Fatalf("stale status=%#v", got)
	}
	persisted := jsonutil.Map(a.readJSONDefault(filepath.Join(a.cacheDir, "system-update-status.json"), map[string]any{}))
	if persisted["state"] != "failed" || persisted["interrupted"] != true {
		t.Fatalf("recovery not persisted: %#v", persisted)
	}
	if recoveredAt, updatedAt := jsonutil.Int(persisted["recoveredAt"], 0), jsonutil.Int(persisted["updatedAt"], 0); int64(recoveredAt) <= old || updatedAt != recoveredAt {
		t.Fatalf("recovery timestamp was not preserved: recoveredAt=%d updatedAt=%d payload=%#v", recoveredAt, updatedAt, persisted)
	}
}

func TestSystemUpdateStatusPreservesFreshLaunchHandoff(t *testing.T) {
	a := testSystemUpdateApp(t)
	writeSystemStatusForTest(t, a, map[string]any{"state": "starting", "updatedAt": time.Now().Unix(), "bootId": systemBootID()})
	got := a.systemUpdateStatus()
	if got["running"] != true || got["state"] != "starting" {
		t.Fatalf("fresh launch handoff=%#v", got)
	}
}

func TestSystemUpdateStatusPreservesAmbiguousLock(t *testing.T) {
	a := testSystemUpdateApp(t)
	lock := filepath.Join(a.cacheDir, "system-update.lock")
	if err := os.MkdirAll(lock, 0700); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-systemUpdateInterruptedGrace - time.Second).Unix()
	writeSystemStatusForTest(t, a, map[string]any{"state": "running", "updatedAt": old, "startedAt": old, "bootId": systemBootID()})
	got := a.systemUpdateStatus()
	if got["running"] != true || got["state"] != "running" || got["lockPresent"] != true {
		t.Fatalf("ambiguous lock status=%#v", got)
	}
}

func TestSystemUpdateStatusRecoversAcrossBootChange(t *testing.T) {
	a := testSystemUpdateApp(t)
	writeSystemStatusForTest(t, a, map[string]any{"state": "running", "updatedAt": time.Now().Unix(), "bootId": "previous-boot", "commandPid": os.Getpid()})
	got := a.systemUpdateStatus()
	if got["running"] != false || got["state"] != "failed" || got["interrupted"] != true {
		t.Fatalf("boot changed status=%#v", got)
	}
}

func TestWriteStatusCLIRecordsCommandPID(t *testing.T) {
	a := testSystemUpdateApp(t)
	path := filepath.Join(a.cacheDir, "status.json")
	if code := a.runWriteStatusCLI([]string{"--file", path, "--state", "running", "--command-pid", "1234"}); code != 0 {
		t.Fatalf("write-status exit=%d", code)
	}
	got := jsonutil.Map(a.readJSONDefault(path, map[string]any{}))
	if jsonutil.Int(got["commandPid"], 0) != 1234 {
		t.Fatalf("command PID not persisted: %#v", got)
	}
	if code := a.runWriteStatusCLI([]string{"--file", path, "--command-pid", "nope"}); code != 64 {
		t.Fatalf("invalid command PID exit=%d, want 64", code)
	}
}
