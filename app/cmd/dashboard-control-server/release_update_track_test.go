package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"
)

func writeTrackToggleEnv(t *testing.T, a *app, track string) string {
	t.Helper()
	body := "# legacy installer update connection\n" +
		"BASE_URL=https://updates.example.invalid/dashboard\n" +
		"DASH_TOKEN=token-with-$-characters\n" +
		"DASH_USERPASS='dash:private value'\n" +
		"DASH_TRACK=" + track + "\n"
	if err := os.WriteFile(a.updateEnvPath(), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return body
}

func writeTrackToggleProfile(t *testing.T, a *app, schema int, track string) {
	t.Helper()
	profile := map[string]any{"schema": schema, "track": track}
	if schema == legacyUpdateProfileSchema {
		profile["baseUrl"] = "https://updates.example.invalid/dashboard"
		profile["token"] = "token-with-$-characters"
		profile["userpass"] = "dash:private value"
	}
	body, err := json.Marshal(profile)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.updateProfilePath(), body, 0600); err != nil {
		t.Fatal(err)
	}
}

func assertTrackTogglePrivateMode(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("%s mode=%#o, want 0600", path, got)
	}
}

func TestToggleUpdateTrackMigratesLegacyStateAndRemovesCredentials(t *testing.T) {
	a := testProfileApp(t)
	writeTrackToggleEnv(t, a, "beta")
	writeTrackToggleProfile(t, a, legacyUpdateProfileSchema, "beta")
	a.updateAvailabilityMu.Lock()
	a.updateAvailabilityCache = map[string]any{"track": "beta"}
	a.updateAvailabilityAt = time.Now()
	a.updateAvailabilityMu.Unlock()

	res, err := a.toggleUpdateTrack()
	if err != nil {
		t.Fatal(err)
	}
	if got := res["track"]; got != "stable" {
		t.Fatalf("track=%v, want stable", got)
	}
	if got := res["profileSource"]; got != "private-json-v2" {
		t.Fatalf("profileSource=%v", got)
	}
	env, err := os.ReadFile(a.updateEnvPath())
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(env), "DASH_TRACK="); got != 1 || !strings.Contains(string(env), "DASH_TRACK=stable\n") {
		t.Fatalf("sanitized env=%q", env)
	}
	for _, forbidden := range []string{"BASE_URL", "DASH_TOKEN", "DASH_USERPASS", "updates.example", "token-with"} {
		if strings.Contains(string(env), forbidden) {
			t.Fatalf("legacy connection content remained in env: %q", env)
		}
	}
	assertTrackTogglePrivateMode(t, a.updateEnvPath())
	profileRaw, err := os.ReadFile(a.updateProfilePath())
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"baseUrl", "token", "userpass", "updates.example", "private value"} {
		if strings.Contains(string(profileRaw), forbidden) {
			t.Fatalf("legacy connection content remained in profile: %s", profileRaw)
		}
	}
	profile, err := readPrivateUpdateProfile(a.updateProfilePath())
	if err != nil {
		t.Fatal(err)
	}
	if profile["DASH_TRACK"] != "stable" || profile["schema"] != "2" {
		t.Fatalf("profile=%#v", profile)
	}
	assertTrackTogglePrivateMode(t, a.updateProfilePath())
	a.updateAvailabilityMu.Lock()
	cache, at := a.updateAvailabilityCache, a.updateAvailabilityAt
	a.updateAvailabilityMu.Unlock()
	if cache != nil || !at.IsZero() {
		t.Fatalf("availability cache was not invalidated: cache=%#v at=%v", cache, at)
	}
}

func TestToggleUpdateTrackRecreatesMissingStateFromInstalledVersion(t *testing.T) {
	a := testProfileApp(t)
	if err := os.WriteFile(a.dash+"/VERSION", []byte("1.5.0-beta.33\n"), 0644); err != nil {
		t.Fatal(err)
	}
	res, err := a.toggleUpdateTrack()
	if err != nil {
		t.Fatal(err)
	}
	if got := res["track"]; got != "stable" {
		t.Fatalf("track=%v, want stable", got)
	}
	if env, err := os.ReadFile(a.updateEnvPath()); err != nil || !strings.Contains(string(env), "DASH_TRACK=stable\n") {
		t.Fatalf("recreated env=%q err=%v", env, err)
	}
}

func TestToggleUpdateTrackPostRouteUsesNormalProtectedAPIPath(t *testing.T) {
	a := testProfileApp(t)
	writeTrackToggleEnv(t, a, "beta")
	writeTrackToggleProfile(t, a, updateProfileSchema, "beta")
	req := httptest.NewRequest(http.MethodPost, "/api/update/track/toggle", strings.NewReader("{}"))
	req.RemoteAddr = "127.0.0.1:41111"
	rec := httptest.NewRecorder()
	a.handle(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("route status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload["track"] != "stable" {
		t.Fatalf("route payload=%#v, want stable track", payload)
	}
}

func TestToggleUpdateTrackRefusesActiveUpdateWithoutChangingLegacyState(t *testing.T) {
	a := testProfileApp(t)
	original := writeTrackToggleEnv(t, a, "beta")
	writeTrackToggleProfile(t, a, legacyUpdateProfileSchema, "beta")
	if err := writeJSONPrivateFile(a.updateJobPath(), map[string]any{"state": "running"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.toggleUpdateTrack(); !errors.Is(err, errDashboardUpdateTrackBusy) {
		t.Fatalf("toggle error=%v, want active-update refusal", err)
	}
	after, err := os.ReadFile(a.updateEnvPath())
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != original {
		t.Fatalf("active update changed saved state:\nwant %q\n got %q", original, after)
	}
}

func TestUpdateEnvWithTrackAlwaysSanitizesLegacyContent(t *testing.T) {
	got := updateEnvWithTrack("beta")
	if strings.Contains(got, "BASE_URL") || strings.Contains(got, "DASH_TOKEN") || strings.Contains(got, "DASH_USERPASS") {
		t.Fatalf("credential fields in sanitized env: %q", got)
	}
	if strings.Count(got, "DASH_TRACK=") != 1 || !strings.HasSuffix(got, "DASH_TRACK=beta\n") {
		t.Fatalf("sanitized env=%q", got)
	}
}

func TestToggleUpdateTrackIgnoresReleasedLockAnchor(t *testing.T) {
	a := testProfileApp(t)
	writeTrackToggleEnv(t, a, "beta")
	writeTrackToggleProfile(t, a, updateProfileSchema, "beta")
	if err := os.WriteFile(a.updateLockPath(), []byte("stable flock anchor\n"), 0600); err != nil {
		t.Fatal(err)
	}
	res, err := a.toggleUpdateTrack()
	if err != nil {
		t.Fatal(err)
	}
	if res["track"] != "stable" {
		t.Fatalf("track=%v, want stable", res["track"])
	}
}

func TestToggleUpdateTrackRefusesHeldLock(t *testing.T) {
	a := testProfileApp(t)
	writeTrackToggleEnv(t, a, "beta")
	writeTrackToggleProfile(t, a, updateProfileSchema, "beta")
	file, err := os.OpenFile(a.updateLockPath(), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	if _, err := a.toggleUpdateTrack(); !errors.Is(err, errDashboardUpdateTrackBusy) {
		t.Fatalf("toggle error=%v, want held-lock busy refusal", err)
	}
}
