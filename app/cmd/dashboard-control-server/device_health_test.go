package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func TestTextValueNilIsEmpty(t *testing.T) {
	if got := jsonutil.TextValue(nil); got != "" {
		t.Fatalf("textValue(nil) = %q, want empty string", got)
	}
}

func TestDeviceHealthSeparatesDeviceAndData(t *testing.T) {
	dash := t.TempDir()
	a := &app{dash: dash, cacheDir: filepath.Join(dash, "cache"), configDir: filepath.Join(dash, "config")}
	a.ensureDirs()
	if err := fileio.WriteJSON(filepath.Join(a.cacheDir, "storage-wear-state.json"), map[string]any{"level": "ok"}); err != nil {
		t.Fatal(err)
	}
	h := a.deviceHealth()
	if h["device"] == "failing" {
		t.Fatalf("unexpected device failure: %#v", h)
	}
	if h["data"] != "unknown" {
		t.Fatalf("expected unknown data without caches, got %#v", h["data"])
	}
}

func TestDeviceHealthRunningGuardIsHealthy(t *testing.T) {
	dash := t.TempDir()
	a := &app{dash: dash, cacheDir: filepath.Join(dash, "cache"), configDir: filepath.Join(dash, "config")}
	a.ensureDirs()
	if err := fileio.WriteJSON(filepath.Join(a.cacheDir, "health-guard-status.json"), map[string]any{"state": "running", "reason": nil}); err != nil {
		t.Fatal(err)
	}
	h := a.deviceHealth()
	if got := h["device"]; got != "ok" {
		t.Fatalf("running health guard made device %q, want ok; health=%#v", got, h)
	}
	if got := h["statusLine"]; got != "All systems normal" {
		t.Fatalf("running health guard line = %q, want healthy line", got)
	}
}

func TestDeviceHealthNullReasonUsesFactFallback(t *testing.T) {
	dash := t.TempDir()
	a := &app{dash: dash, cacheDir: filepath.Join(dash, "cache"), configDir: filepath.Join(dash, "config")}
	a.ensureDirs()
	if err := fileio.WriteJSON(filepath.Join(a.cacheDir, "storage-wear-state.json"), map[string]any{"level": "warn", "reason": nil}); err != nil {
		t.Fatal(err)
	}
	h := a.deviceHealth()
	if got := h["device"]; got != "degraded" {
		t.Fatalf("null-reason storage warning made device %q, want degraded; health=%#v", got, h)
	}
	line, _ := h["statusLine"].(string)
	if line != "storage is degraded" {
		t.Fatalf("null-reason status line = %q, want fact fallback", line)
	}
	if strings.Contains(line, "<nil>") {
		t.Fatalf("status line leaked nil sentinel: %q", line)
	}
}

func TestDeviceHealthPendingPostUpdateIsSilent(t *testing.T) {
	dash := t.TempDir()
	a := &app{dash: dash, cacheDir: filepath.Join(dash, "cache"), configDir: filepath.Join(dash, "config")}
	a.ensureDirs()
	if err := fileio.WriteJSON(filepath.Join(a.cacheDir, "post-update-verify.json"), map[string]any{"state": "pending", "reason": nil}); err != nil {
		t.Fatal(err)
	}
	h := a.deviceHealth()
	if got := h["device"]; got != "ok" {
		t.Fatalf("pending verifier made device %q, want ok; health=%#v", got, h)
	}
	if got := h["statusLine"]; got != "All systems normal" {
		t.Fatalf("pending verifier line=%q, want healthy line", got)
	}
}

func TestDeviceHealthConfirmedClockOverridesStaleMarker(t *testing.T) {
	dash := t.TempDir()
	a := &app{dash: dash, cacheDir: filepath.Join(dash, "cache"), configDir: filepath.Join(dash, "config"), settingsFile: filepath.Join(dash, "config", "settings.json")}
	a.ensureDirs()
	if err := os.WriteFile(filepath.Join(a.cacheDir, "clock-unverified"), []byte("\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := fileio.WriteJSON(filepath.Join(a.cacheDir, "clock-confirmed.json"), map[string]any{"confirmedAt": time.Now().Add(-time.Minute).Unix(), "source": "ntp"}); err != nil {
		t.Fatal(err)
	}
	h := a.deviceHealth()
	if got := h["device"]; got == "degraded" || got == "failing" {
		t.Fatalf("confirmed clock marker produced visible warning: %#v", h)
	}
}

func TestDeviceHealthClockPredatingInstallWarns(t *testing.T) {
	dash := t.TempDir()
	a := &app{dash: dash, cacheDir: filepath.Join(dash, "cache"), configDir: filepath.Join(dash, "config"), settingsFile: filepath.Join(dash, "config", "settings.json")}
	a.ensureDirs()
	if err := os.WriteFile(filepath.Join(a.cacheDir, "clock-unverified"), []byte("\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := fileio.WriteJSON(filepath.Join(dash, "manifest.json"), map[string]any{"buildEpoch": time.Now().Add(48 * time.Hour).Unix()}); err != nil {
		t.Fatal(err)
	}
	h := a.deviceHealth()
	if got := h["device"]; got != "degraded" {
		t.Fatalf("clock predating install made device %q, want degraded; health=%#v", got, h)
	}
}

func TestDataHealthUsesLastSuccessRatherThanCacheMtime(t *testing.T) {
	dash := t.TempDir()
	a := &app{dash: dash, cacheDir: filepath.Join(dash, "cache"), configDir: filepath.Join(dash, "config"), settingsFile: filepath.Join(dash, "config", "settings.json")}
	a.ensureDirs()
	if err := fileio.WriteJSON(a.settingsFile, map[string]any{"refreshWxMinutes": 30, "displaySleepEnabled": false}); err != nil {
		t.Fatal(err)
	}
	if err := fileio.WriteJSON(filepath.Join(a.cacheDir, "weather-cache.json"), map[string]any{"cache": map[string]any{"lastSuccessAt": time.Now().Add(-5 * time.Hour).UnixMilli()}}); err != nil {
		t.Fatal(err)
	}
	h := a.deviceHealth()
	facts := h["facts"].([]healthFact)
	for _, f := range facts {
		if f.Name == "weather" {
			if f.Level != "failing" {
				t.Fatalf("weather freshness=%q, want failing from old lastSuccessAt", f.Level)
			}
			return
		}
	}
	t.Fatal("weather fact missing")
}

func TestDisplaySleepSuppressesStalenessDuringSleepAndWakeGrace(t *testing.T) {
	a := &app{}
	settings := map[string]any{"displaySleepEnabled": true, "displaySleepOff": "22:30", "displaySleepOn": "06:00"}
	if !a.suppressDataStaleness(time.Date(2030, 1, 2, 2, 0, 0, 0, time.Local), settings, 45) {
		t.Fatal("expected overnight sleep suppression")
	}
	if !a.suppressDataStaleness(time.Date(2030, 1, 2, 6, 30, 0, 0, time.Local), settings, 45) {
		t.Fatal("expected post-wake grace suppression")
	}
	if a.suppressDataStaleness(time.Date(2030, 1, 2, 9, 0, 0, 0, time.Local), settings, 45) {
		t.Fatal("unexpected stale suppression long after wake")
	}
}

func TestDeviceHealthRealGuardWarningStillShows(t *testing.T) {
	dash := t.TempDir()
	a := &app{dash: dash, cacheDir: filepath.Join(dash, "cache"), configDir: filepath.Join(dash, "config")}
	a.ensureDirs()
	if err := fileio.WriteJSON(filepath.Join(a.cacheDir, "health-guard-status.json"), map[string]any{
		"state": "warning", "warnings": []string{"health-guard-lock-unavailable"}, "reason": "health guard cannot acquire its lock",
	}); err != nil {
		t.Fatal(err)
	}
	h := a.deviceHealth()
	if got := h["device"]; got != "degraded" {
		t.Fatalf("real guard warning was hidden: %#v", h)
	}
	if got := h["statusLine"]; got != "health guard cannot acquire its lock" {
		t.Fatalf("real guard warning lost its reason: %#v", h)
	}
}
