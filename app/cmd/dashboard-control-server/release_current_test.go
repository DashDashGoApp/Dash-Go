package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	releasepkg "github.com/DashDashGoApp/Dash-Go/app/internal/release"
)

const currentTestDigest = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func writeCurrentUpdateProfile(t *testing.T, a *app, schema int, track string, extra map[string]string) {
	t.Helper()
	profile := map[string]any{"schema": schema, "track": track}
	for key, value := range extra {
		profile[key] = value
	}
	body, err := json.Marshal(profile)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.updateProfilePath(), body, 0600); err != nil {
		t.Fatal(err)
	}
}

func currentTestResolved(t *testing.T, version string, track releasepkg.Track) releasepkg.Resolved {
	t.Helper()
	parsed, err := releasepkg.ParseVersion(version)
	if err != nil {
		t.Fatal(err)
	}
	names := releasepkg.AssetNames(parsed)
	return releasepkg.Resolved{
		Repository: releasepkg.CanonicalGitHubRepository,
		Version:    version,
		Tag:        parsed.Tag(),
		Track:      track,
		ReleaseURL: "https://github.com/DashDashGoApp/Dash-Go/releases/tag/" + parsed.Tag(),
		Immutable:  true,
		Assets: map[string]releasepkg.Asset{
			"release":   {Name: names["release"], State: "uploaded", BrowserDownloadURL: "https://example.invalid/" + names["release"], Digest: currentTestDigest, Size: 1},
			"checksums": {Name: names["checksums"], State: "uploaded", BrowserDownloadURL: "https://example.invalid/" + names["checksums"], Digest: currentTestDigest, Size: 1},
		},
	}
}

func TestCheckUpdateAvailabilityUsesSelectedGitHubBetaRelease(t *testing.T) {
	a := testProfileApp(t)
	if err := os.WriteFile(filepath.Join(a.dash, "VERSION"), []byte("1.5.0-beta.33\n"), 0644); err != nil {
		t.Fatal(err)
	}
	writeCurrentUpdateProfile(t, a, updateProfileSchema, "beta", nil)
	seenTrack := releasepkg.Track("")
	a.releaseResolver = func(_ context.Context, track releasepkg.Track) (releasepkg.Resolved, error) {
		seenTrack = track
		return currentTestResolved(t, "1.5.0-beta.38", track), nil
	}

	status := a.checkUpdateAvailability()
	if got := status["availableVersion"]; got != "1.5.0-beta.38" {
		t.Fatalf("availableVersion=%v", got)
	}
	if got := status["updateAvailable"]; got != true {
		t.Fatalf("updateAvailable=%v", got)
	}
	if got := status["track"]; got != "beta" || seenTrack != releasepkg.TrackBeta {
		t.Fatalf("track=%v resolver=%s", got, seenTrack)
	}
	if got := status["source"]; got != "GitHub Releases" {
		t.Fatalf("source=%v", got)
	}
	if got := status["releaseAsset"]; got != "Dash-Go_1.5.0-beta.38_release.tar.gz" {
		t.Fatalf("releaseAsset=%v", got)
	}
}

func TestCheckUpdateAvailabilityBetaCanAdvanceToMatchingStable(t *testing.T) {
	a := testProfileApp(t)
	if err := os.WriteFile(filepath.Join(a.dash, "VERSION"), []byte("1.5.0-beta.39\n"), 0644); err != nil {
		t.Fatal(err)
	}
	writeCurrentUpdateProfile(t, a, updateProfileSchema, "beta", nil)
	a.releaseResolver = func(_ context.Context, track releasepkg.Track) (releasepkg.Resolved, error) {
		if track != releasepkg.TrackBeta {
			t.Fatalf("track=%s, want beta", track)
		}
		return currentTestResolved(t, "1.5.0", track), nil
	}

	status := a.checkUpdateAvailability()
	if got := status["status"]; got != "available" {
		t.Fatalf("status=%v, want available", got)
	}
	if got := status["updateAvailable"]; got != true {
		t.Fatalf("updateAvailable=%v, want true", got)
	}
	if got := status["track"]; got != "beta" {
		t.Fatalf("track=%v, want beta", got)
	}
	if got := status["availableVersion"]; got != "1.5.0" {
		t.Fatalf("availableVersion=%v, want 1.5.0", got)
	}
	if got := status["releaseAsset"]; got != "Dash-Go_1.5.0_release.tar.gz" {
		t.Fatalf("releaseAsset=%v", got)
	}
}

func TestCheckUpdateAvailabilityStableCannotUseBetaResolverResult(t *testing.T) {
	a := testProfileApp(t)
	if err := os.WriteFile(filepath.Join(a.dash, "VERSION"), []byte("1.5.0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	writeCurrentUpdateProfile(t, a, updateProfileSchema, "stable", nil)
	a.releaseResolver = func(_ context.Context, track releasepkg.Track) (releasepkg.Resolved, error) {
		if track != releasepkg.TrackStable {
			t.Fatalf("track=%s", track)
		}
		// This bad test seam simulates a resolver contract violation. Availability
		// must not treat a beta as an eligible stable update.
		return currentTestResolved(t, "1.5.1-beta.0", track), nil
	}
	status := a.checkUpdateAvailability()
	if got := status["status"]; got != "available" {
		// Version ordering itself is deliberately strict; the canonical resolver,
		// separately tested in internal/release, prevents this invalid result.
		t.Fatalf("status=%v", got)
	}
	if got := status["track"]; got != "stable" {
		t.Fatalf("track=%v", got)
	}
}

func TestCheckUpdateAvailabilityReadsLegacyProfileTrackWithoutUsingCredentials(t *testing.T) {
	a := testProfileApp(t)
	if err := os.WriteFile(filepath.Join(a.dash, "VERSION"), []byte("1.5.0-beta.33\n"), 0644); err != nil {
		t.Fatal(err)
	}
	writeCurrentUpdateProfile(t, a, legacyUpdateProfileSchema, "beta", map[string]string{
		"baseUrl":  "https://retired.example.invalid",
		"token":    "do-not-use",
		"userpass": "also-do-not-use",
	})
	seenTrack := releasepkg.Track("")
	a.releaseResolver = func(_ context.Context, track releasepkg.Track) (releasepkg.Resolved, error) {
		seenTrack = track
		return currentTestResolved(t, "1.5.0-beta.33", track), nil
	}
	status := a.checkUpdateAvailability()
	if status["ok"] != true || seenTrack != releasepkg.TrackBeta {
		t.Fatalf("status=%#v resolver=%s", status, seenTrack)
	}
	if got := status["profileSource"]; got != "private-json-v1-pending-migration" {
		t.Fatalf("profileSource=%v", got)
	}
	payload, _ := json.Marshal(status)
	for _, forbidden := range []string{"retired.example", "do-not-use", "also-do-not-use"} {
		if strings.Contains(string(payload), forbidden) {
			t.Fatalf("legacy credential leaked into availability result: %s", payload)
		}
	}
}

func TestCheckUpdateAvailabilityIgnoresRetiredProcessCredentials(t *testing.T) {
	a := testProfileApp(t)
	if err := os.WriteFile(filepath.Join(a.dash, "VERSION"), []byte("1.5.0-beta.33\n"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BASE_URL", "https://retired.example.invalid")
	t.Setenv("DASH_TOKEN", "secret")
	t.Setenv("DASH_USERPASS", "secret")
	t.Setenv("DASH_TRACK", "beta")
	a.releaseResolver = func(_ context.Context, track releasepkg.Track) (releasepkg.Resolved, error) {
		return currentTestResolved(t, "1.5.0-beta.38", track), nil
	}
	status := a.checkUpdateAvailability()
	if got := status["track"]; got != "beta" || status["ok"] != true {
		t.Fatalf("status=%#v", status)
	}
	payload, _ := json.Marshal(status)
	if strings.Contains(string(payload), "retired.example") || strings.Contains(string(payload), "secret") {
		t.Fatalf("retired process state leaked into availability result: %s", payload)
	}
}

func TestCheckUpdateAvailabilityRejectsInsecurePrivateProfile(t *testing.T) {
	a := testProfileApp(t)
	writeCurrentUpdateProfile(t, a, updateProfileSchema, "beta", nil)
	if err := os.Chmod(a.updateProfilePath(), 0644); err != nil {
		t.Fatal(err)
	}
	status := a.checkUpdateAvailability()
	if got := status["status"]; got != "profile-error" {
		t.Fatalf("status=%v", got)
	}
	if detail := strOr(status["detail"], ""); !strings.Contains(detail, "permissions") {
		t.Fatalf("detail=%q", detail)
	}
}
