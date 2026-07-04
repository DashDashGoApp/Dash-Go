package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	writebackpkg "github.com/DashDashGoApp/Dash-Go/app/internal/calendar/writeback"
	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
)

func TestWritebackPairResultUsesOnlyTheRequestedPair(t *testing.T) {
	output := "RESULT\tdash_other\tconflict\nRESULT\tdash_family\tsynced\n"
	if got := writebackPairResult(output, "dash_family"); got != "synced" {
		t.Fatalf("result = %q, want synced", got)
	}
}

func TestWritebackPairResultLegacyFallbackPrefersMostSeriousOutcome(t *testing.T) {
	output := "RESULT\tdash_first\tsynced\nRESULT\tdash_second\tattention-empty\nRESULT\tdash_third\tconflict\n"
	if got := writebackPairResult(output, "__legacy__"); got != "conflict" {
		t.Fatalf("legacy result = %q, want conflict", got)
	}
}

func TestWritebackPairResultRejectsMalformedRows(t *testing.T) {
	output := "RESULT dash_family conflict\nRESULT\tdash_family\nRESULT\tdash_family\tsynced\textra\n"
	if got := writebackPairResult(output, "dash_family"); got != "" {
		t.Fatalf("malformed result = %q, want empty", got)
	}
}

func TestWritebackSyncArgsArmsEmptyOverrideOnlyForVerifiedFinalDelete(t *testing.T) {
	regular := writebackSyncArgs("dash_family", false)
	if slices.Contains(regular, "--allow-empty-once") {
		t.Fatalf("non-final delete args unexpectedly armed the empty override: %#v", regular)
	}
	final := writebackSyncArgs("dash_family", true)
	if !slices.Contains(final, "--allow-empty-once") {
		t.Fatalf("final delete args omitted the one-run empty override: %#v", final)
	}
	if got := writebackSyncArgs("__legacy__", true); len(got) != 0 {
		t.Fatalf("legacy source must not guess a targeted destructive pair: %#v", got)
	}
}

func TestWritebackSyncOutcomeClassifiesZeroExitSkipped(t *testing.T) {
	state, detail, _ := writebackSyncOutcome("skipped", false, false, false, 0, false)
	if state != "attention" {
		t.Fatalf("zero-exit skipped state = %q, want attention", state)
	}
	if strings.Contains(strings.ToLower(detail), "synchronized") {
		t.Fatalf("zero-exit skipped result was reported as synchronized: %q", detail)
	}
	if !strings.Contains(strings.ToLower(detail), "authorization") {
		t.Fatalf("zero-exit skipped result did not explain authorization: %q", detail)
	}
}

func TestWritebackSyncOutcomeSchedulesOnlyTheBoundedFinalDeleteRetry(t *testing.T) {
	state, detail, retry := writebackSyncOutcome("", true, false, true, 1, true)
	if state != "waiting" || !retry {
		t.Fatalf("first failed final delete = (%q, %t), want waiting retry", state, retry)
	}
	if !strings.Contains(strings.ToLower(detail), "will retry") {
		t.Fatalf("first final-delete failure did not explain the bounded retry: %q", detail)
	}
	state, detail, retry = writebackSyncOutcome("", true, false, true, calendarWritebackFinalDeleteMaxAutomaticAttempts, true)
	if state != "attention" || retry {
		t.Fatalf("bounded final-delete retry exhaustion = (%q, %t), want attention without retry", state, retry)
	}
	if !strings.Contains(strings.ToLower(detail), "bounded") {
		t.Fatalf("bounded retry exhaustion was not explained: %q", detail)
	}
}

func TestWritebackSyncOutcomeDoesNotTreatUnknownZeroExitResultAsSynced(t *testing.T) {
	state, detail, _ := writebackSyncOutcome("unexpected-state", false, false, false, 0, false)
	if state != "attention" {
		t.Fatalf("unknown zero-exit result state = %q, want attention", state)
	}
	if strings.Contains(strings.ToLower(detail), "synchronized") {
		t.Fatalf("unknown zero-exit result was reported as synchronized: %q", detail)
	}
	if !strings.Contains(strings.ToLower(detail), "unrecognized") {
		t.Fatalf("unknown zero-exit result did not explain its safe classification: %q", detail)
	}
}

func newFinalDeleteSyncTestApp(t *testing.T) (*app, string, string, string) {
	t.Helper()
	a := testApp(t)
	source := "calendars/family.blue.ics"
	pair := "dash_family"
	collection := filepath.Join(a.home, ".dashboard-vdirsyncer", "collections", "family")
	if err := os.MkdirAll(collection, 0700); err != nil {
		t.Fatal(err)
	}
	registry := writebackpkg.Registry{
		Version: writebackpkg.RegistryVersion,
		Enabled: true,
		Calendars: []writebackpkg.Calendar{{
			Source: source, Collection: collection, Writable: true, Enabled: true,
			Name: "Family", Pair: pair, Provider: "caldav", Connection: "test", RemoteID: "family",
		}},
	}
	if err := fileio.WriteJSON(filepath.Join(a.configDir, calendarWritebackRegistryFile), registry); err != nil {
		t.Fatal(err)
	}
	return a, source, pair, collection
}

func writeFinalDeleteSyncTestScript(t *testing.T, a *app) (logPath string) {
	t.Helper()
	logPath = filepath.Join(a.dash, "sync-args.log")
	countPath := filepath.Join(a.dash, "sync-count")
	script := fmt.Sprintf(`#!/bin/sh
set -eu
log=%q
count=%q
printf '%%s\n' "$*" >> "$log"
n=0
if [ -r "$count" ]; then n=$(cat "$count"); fi
n=$((n + 1))
printf '%%s\n' "$n" > "$count"
if [ "$n" -eq 1 ]; then
  printf 'RESULT\tdash_family\tfailed\n'
  exit 1
fi
printf 'RESULT\tdash_family\tsynced\n'
`, logPath, countPath)
	path := filepath.Join(a.binDir, "sync-vdir.sh")
	if err := os.WriteFile(path, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	return logPath
}

func waitForWritebackTest(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func logLineCount(path string) int {
	body, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	return len(strings.FieldsFunc(string(body), func(r rune) bool { return r == '\n' || r == '\r' }))
}

func finalDeleteStoreForTest(t *testing.T, a *app) calendarWritebackFinalDeleteStore {
	t.Helper()
	a.writebackSyncMu.Lock()
	defer a.writebackSyncMu.Unlock()
	store, err := a.loadCalendarWritebackFinalDeleteStoreLocked()
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func TestFinalDeleteAuthorizationPersistsFailedTargetedRunAcrossRestart(t *testing.T) {
	oldDelay := calendarWritebackFinalDeleteRetryDelay
	calendarWritebackFinalDeleteRetryDelay = time.Hour
	t.Cleanup(func() { calendarWritebackFinalDeleteRetryDelay = oldDelay })

	a, source, pair, collection := newFinalDeleteSyncTestApp(t)
	logPath := writeFinalDeleteSyncTestScript(t, a)
	if err := a.recordCalendarWritebackMutation(source, pair, collection, true); err != nil {
		t.Fatal(err)
	}
	a.queueCalendarWritebackSync(source, pair, true)
	waitForWritebackTest(t, "first targeted final-delete sync", func() bool { return logLineCount(logPath) == 1 })
	waitForWritebackTest(t, "failed final-delete state", func() bool {
		status, err := a.calendarWritebackService().Status()
		return err == nil && status.States[source].State == "waiting"
	})
	store := finalDeleteStoreForTest(t, a)
	permit, ok := store.Pairs[pair]
	if !ok || permit.Attempts != 1 {
		t.Fatalf("failed targeted sync lost durable final-delete permission: %#v", store)
	}

	restarted := &app{
		dash: a.dash, home: a.home, configDir: a.configDir, calDir: a.calDir,
		cacheDir: a.cacheDir, logDir: a.logDir, binDir: a.binDir,
		settingsFile: a.settingsFile, configLocal: a.configLocal,
	}
	restarted.resumeCalendarWritebackFinalDeletes()
	waitForWritebackTest(t, "recovered targeted final-delete sync", func() bool { return logLineCount(logPath) == 2 })
	waitForWritebackTest(t, "cleared final-delete permission", func() bool {
		_, err := os.Stat(restarted.calendarWritebackFinalDeletePath())
		return os.IsNotExist(err)
	})
	lines, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.FieldsFunc(string(lines), func(r rune) bool { return r == '\n' || r == '\r' }) {
		if !strings.Contains(line, "--pair dash_family") || !strings.Contains(line, "--allow-empty-once") {
			t.Fatalf("recovered run lost the final-delete override: %q", line)
		}
	}
}

func TestNonFinalMutationClearsDurableFinalDeleteAuthorization(t *testing.T) {
	a, source, pair, collection := newFinalDeleteSyncTestApp(t)
	if err := a.recordCalendarWritebackMutation(source, pair, collection, true); err != nil {
		t.Fatal(err)
	}
	if _, ok := finalDeleteStoreForTest(t, a).Pairs[pair]; !ok {
		t.Fatal("verified final delete was not persisted")
	}
	if err := a.recordCalendarWritebackMutation(source, pair, collection, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(a.calendarWritebackFinalDeletePath()); !os.IsNotExist(err) {
		t.Fatalf("non-final mutation retained final-delete authority, err=%v", err)
	}
}
