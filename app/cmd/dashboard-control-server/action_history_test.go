package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
)

func actionHistoryEntriesForTest(t *testing.T, a *app) []map[string]any {
	t.Helper()
	entries, err := readActionHistoryLocked(a.actionHistoryPath())
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

func TestUpdateActionHistoryUsesOneJobLinkedEntryAndFinalizesIt(t *testing.T) {
	a := testProfileApp(t)
	job := map[string]any{
		"id": "update-history-1", "state": "queued", "requestedAt": int64(1700000000),
		"track": "beta", "target": "1.4.3-beta.95", "backup": "pre-update.zip", "source": "control",
	}
	if err := a.recordUpdateAction(job); err != nil {
		t.Fatal(err)
	}
	if err := a.recordUpdateAction(job); err != nil {
		t.Fatal(err)
	}
	entries := actionHistoryEntriesForTest(t, a)
	if len(entries) != 1 {
		t.Fatalf("linked update entries=%d want 1: %#v", len(entries), entries)
	}
	entry := entries[0]
	if entry["id"] != "update:update-history-1" || entry["state"] != "running" || actionHistoryUpdateJobID(entry) != "update-history-1" {
		t.Fatalf("running entry=%#v", entry)
	}

	job["state"] = "success"
	job["version"] = "1.4.3-beta.95"
	job["detail"] = "Installed 1.4.3-beta.95; runtime and bounded post-update health checks passed."
	job["updatedAt"] = int64(1700000010)
	if err := writeJSONPrivateFile(a.updateJobPath(), job); err != nil {
		t.Fatal(err)
	}
	a.reconcileUpdateActionHistory()
	entries = actionHistoryEntriesForTest(t, a)
	if len(entries) != 1 {
		t.Fatalf("completion appended a second entry: %#v", entries)
	}
	entry = entries[0]
	if entry["state"] != "success" || anyInt64(entry["completedAt"], 0) != 1700000010 {
		t.Fatalf("terminal entry=%#v", entry)
	}
	if !strings.Contains(strOr(entry["detail"], ""), "health checks passed") {
		t.Fatalf("success detail=%q", entry["detail"])
	}
}

func TestFinalizeUpdateActionCLIIsIdempotentForTerminalRollback(t *testing.T) {
	a := testProfileApp(t)
	job := map[string]any{
		"id": "update-history-rollback", "state": "running", "requestedAt": int64(1700000100), "source": "control",
	}
	if err := a.recordUpdateAction(job); err != nil {
		t.Fatal(err)
	}
	job["state"] = "rolledback"
	job["detail"] = "The updated release failed bounded runtime verification and the prior release was restored and verified."
	job["updatedAt"] = int64(1700000110)
	jobPath := filepath.Join(a.cacheDir, "terminal-job.json")
	if err := writeJSONPrivateFile(jobPath, job); err != nil {
		t.Fatal(err)
	}
	args := []string{"--history", a.actionHistoryPath(), "--job", jobPath}
	if rc := a.runFinalizeUpdateActionCLI(args); rc != 0 {
		t.Fatalf("first finalizer rc=%d", rc)
	}
	if rc := a.runFinalizeUpdateActionCLI(args); rc != 0 {
		t.Fatalf("idempotent finalizer rc=%d", rc)
	}
	entries := actionHistoryEntriesForTest(t, a)
	if len(entries) != 1 || entries[0]["state"] != "rolledback" {
		t.Fatalf("rollback history=%#v", entries)
	}
	if anyInt64(entries[0]["completedAt"], 0) != 1700000110 {
		t.Fatalf("rollback completion timestamp=%#v", entries[0])
	}
}

func TestUpdateActionHistoryLeavesUnlinkedRowsUntouched(t *testing.T) {
	a := testProfileApp(t)
	entries := []map[string]any{
		{"at": int64(1700000200), "kind": "update", "label": "Update dashboard", "state": "running", "detail": "unlinked active-looking row"},
		{"at": int64(1700000100), "kind": "update", "label": "Update dashboard", "state": "running", "detail": "unlinked historical row"},
	}
	if err := fileio.WriteJSON(a.actionHistoryPath(), entries); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONPrivateFile(a.updateJobPath(), map[string]any{"id": "current-job", "state": "success"}); err != nil {
		t.Fatal(err)
	}
	a.reconcileUpdateActionHistory()
	got := actionHistoryEntriesForTest(t, a)
	for _, entry := range got {
		if entry["state"] != "running" || entry["reconciledAt"] != nil || actionHistoryMeta(entry)["legacyOutcome"] != nil {
			t.Fatalf("unlinked history row was rewritten: %#v", entry)
		}
	}
}

func TestUpdateActionRecordedErrorKeepsDuplicateActionRouteFromAppending(t *testing.T) {
	if !updateActionAlreadyRecorded(dashboardUpdateActionRecordedError{err: errDashboardUpdateRunning}) {
		t.Fatal("recorded update error was not classified")
	}
	if updateActionAlreadyRecorded(errDashboardUpdateRunning) {
		t.Fatal("ordinary duplicate-update conflict must remain distinguishable")
	}
}
