package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// captureStderr redirects the os.Stderr variable to a pipe and copies it into
// the returned buffer. finish() restores os.Stderr and drains the pipe, so
// assertions must call finish() before inspecting the buffer.
func captureStderr(t *testing.T) (*bytes.Buffer, func()) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	prev := os.Stderr
	os.Stderr = w
	buf := &bytes.Buffer{}
	done := make(chan struct{})
	go func() {
		_, _ = buf.ReadFrom(r)
		close(done)
	}()
	t.Cleanup(func() { r.Close() })
	finish := func() {
		os.Stderr = prev
		_ = w.Close()
		<-done
	}
	return buf, finish
}

// captureStdout works the same way for the os.Stdout variable.
func captureStdout(t *testing.T) (*bytes.Buffer, func()) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	prev := os.Stdout
	os.Stdout = w
	buf := &bytes.Buffer{}
	done := make(chan struct{})
	go func() {
		_, _ = buf.ReadFrom(r)
		close(done)
	}()
	t.Cleanup(func() { r.Close() })
	finish := func() {
		os.Stdout = prev
		_ = w.Close()
		<-done
	}
	return buf, finish
}

// blockEventCacheDir makes the event-cache lock unacquirable (a regular file
// stands in for the cache directory), which forces refreshCurrentEventCache to
// fail without any network access.
func blockEventCacheDir(t *testing.T, a *app) {
	t.Helper()
	blocker := filepath.Join(a.dash, "g4-blocker")
	if err := os.WriteFile(blocker, []byte("block"), 0644); err != nil {
		t.Fatal(err)
	}
	a.cacheDir = filepath.Join(blocker, "cache")
}

// setNoDefaultFeedFetches keeps the CLI from making network calls for default
// feeds (0 sources) in offline tests.
func setNoDefaultFeedFetches(t *testing.T, a *app) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(a.home, ".dashboard-default-calendars"), []byte("DEFAULT_US_HOLIDAYS=0\n"), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestHolidayUpdateCLIExitZeroWhenRefreshSucceeds(t *testing.T) {
	a := testProfileApp(t)
	setNoDefaultFeedFetches(t, a)
	stdout, finishOut := captureStdout(t)
	_, finishErr := captureStderr(t)
	code := a.runHolidayUpdateCLI(nil)
	finishOut()
	finishErr()
	if code != 0 {
		t.Fatalf("holiday CLI exit=%d; success path must exit 0", code)
	}
	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout is not JSON: %v (%q)", err, stdout.String())
	}
	if result["ok"] != true {
		t.Fatalf("holiday update result not ok: %#v", result)
	}
}

func TestHolidayUpdateCLIExitNonZeroWhenRefreshFails(t *testing.T) {
	a := testProfileApp(t)
	setNoDefaultFeedFetches(t, a)
	blockEventCacheDir(t, a)
	stdout, finishOut := captureStdout(t)
	stderr, finishErr := captureStderr(t)
	code := a.runHolidayUpdateCLI(nil)
	finishOut()
	finishErr()
	if code == 0 {
		t.Fatal("holiday CLI must exit non-zero when the forced event-cache refresh fails")
	}
	if !strings.Contains(stderr.String(), "event cache refresh failed") {
		t.Fatalf("stderr missing refresh diagnostic: %q", stderr.String())
	}
	if stdout.Len() == 0 {
		t.Fatal("result payload must still be printed on stdout")
	}
}

func TestISSPassesCLIExitNonZeroWhenRefreshFails(t *testing.T) {
	a := testProfileApp(t)
	blockEventCacheDir(t, a)
	stdout, finishOut := captureStdout(t)
	stderr, finishErr := captureStderr(t)
	code := a.runISSPassesCLI(nil)
	finishOut()
	finishErr()
	if code == 0 {
		t.Fatal("ISS passes CLI must exit non-zero when the forced event-cache refresh fails")
	}
	if !strings.Contains(stderr.String(), "event cache refresh failed") {
		t.Fatalf("stderr missing refresh diagnostic: %q", stderr.String())
	}
	if stdout.Len() == 0 {
		t.Fatal("result payload must still be printed on stdout")
	}
}

func TestHolidayUpdateCLIReportsManifestAndRefreshFailures(t *testing.T) {
	a := testProfileApp(t)
	setNoDefaultFeedFetches(t, a)
	blocker := filepath.Join(a.dash, "g4-blocker")
	if err := os.WriteFile(blocker, []byte("block"), 0644); err != nil {
		t.Fatal(err)
	}
	a.calDir = filepath.Join(blocker, "calendars")
	a.cacheDir = filepath.Join(blocker, "cache")
	a.calendar = nil // rebuild lazily against the blocked calendar dir
	stdout, finishOut := captureStdout(t)
	stderr, finishErr := captureStderr(t)
	code := a.runHolidayUpdateCLI(nil)
	finishOut()
	finishErr()
	if code == 0 {
		t.Fatal("holiday CLI must exit non-zero when refresh fails")
	}
	if !strings.Contains(stderr.String(), "calendar manifest update failed") {
		t.Fatalf("stderr missing manifest diagnostic: %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "event cache refresh failed") {
		t.Fatalf("stderr missing refresh diagnostic: %q", stderr.String())
	}
	if stdout.Len() == 0 {
		t.Fatal("result payload must still be printed on stdout")
	}
}
