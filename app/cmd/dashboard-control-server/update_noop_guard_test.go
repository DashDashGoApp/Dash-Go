package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	releasepkg "github.com/DashDashGoApp/Dash-Go/app/internal/release"
)

func TestCheckUpdateAvailabilityDistinguishesEqualAndNoDowngrade(t *testing.T) {
	a := testProfileApp(t)
	if err := os.WriteFile(filepath.Join(a.dash, "VERSION"), []byte("1.5.6-beta.9\n"), 0644); err != nil {
		t.Fatal(err)
	}
	writeCurrentUpdateProfile(t, a, updateProfileSchema, "beta", nil)
	a.releaseResolver = func(_ context.Context, track releasepkg.Track) (releasepkg.Resolved, error) {
		return currentTestResolved(t, "1.5.6-beta.9", track), nil
	}
	equal := a.checkUpdateAvailability()
	if equal["status"] != "current" || equal["comparison"] != "equal" || equal["updateAvailable"] != false {
		t.Fatalf("equal availability=%#v", equal)
	}

	if err := os.WriteFile(filepath.Join(a.dash, "VERSION"), []byte("1.5.7\n"), 0644); err != nil {
		t.Fatal(err)
	}
	older := a.checkUpdateAvailability()
	if older["status"] != "installed-newer" || older["comparison"] != "installed-newer" || older["noDowngrade"] != true || older["updateAvailable"] != false {
		t.Fatalf("downgrade availability=%#v", older)
	}
}

func TestDashboardNoUpdateResponseHasNoStartOrBackupSignal(t *testing.T) {
	response := dashboardNoUpdateResponse(map[string]any{
		"currentVersion": "1.5.6-beta.9", "availableVersion": "1.5.6-beta.9", "track": "beta", "detail": "current",
	})
	if response["started"] != false || response["noUpdate"] != true || response["label"] != "Already up to date" {
		t.Fatalf("equal no-op response=%#v", response)
	}
	if _, ok := response["backup"]; ok {
		t.Fatalf("no-op response leaked a backup claim: %#v", response)
	}
	downgrade := dashboardNoUpdateResponse(map[string]any{"noDowngrade": true, "detail": "do not downgrade"})
	if downgrade["label"] != "Installed version is newer" || downgrade["noDowngrade"] != true {
		t.Fatalf("downgrade no-op response=%#v", downgrade)
	}
}
