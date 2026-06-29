package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestSettingsWriteKeepsLastGoodAndBootRestoresIt(t *testing.T) {
	dash := t.TempDir()
	a := &app{dash: dash, configDir: filepath.Join(dash, "config"), cacheDir: filepath.Join(dash, "cache"), settingsFile: filepath.Join(dash, "config", "settings.json")}
	a.ensureDirs()
	good := map[string]any{"profile": "lite", "agendaDays": float64(10), "showEventMaps": false}
	if err := a.writeSettings(good); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(a.lastGoodSettingsFile()); err != nil {
		t.Fatalf("last-good missing: %v", err)
	}
	if err := os.WriteFile(a.settingsFile, []byte(`{"agendaDays":0}`), 0644); err != nil {
		t.Fatal(err)
	}
	a.ensureSettingsSafeAtBoot()
	restored, err := readSettingsObject(a.settingsFile)
	if err != nil {
		t.Fatalf("restored settings invalid: %v", err)
	}
	if restored["profile"] != "lite" {
		t.Fatalf("expected restored profile, got %#v", restored)
	}
	marker := readHealthFile(a.configRevertFile())
	if marker["state"] != "reverted" {
		t.Fatalf("expected revert marker, got %#v", marker)
	}
}

func TestSettingsRejectsUnsafeKnownRangesButAllowsUnknown(t *testing.T) {
	if err := validateSettingsShape(map[string]any{"futureReleaseKey": map[string]any{"ok": true}, "radarRenderMode": map[string]any{"retired": true}, "radarHistoryMode": []any{"retired"}, "agendaDays": float64(12)}); err != nil {
		t.Fatal(err)
	}
	if err := validateSettingsShape(map[string]any{"agendaDays": float64(0)}); err == nil {
		t.Fatal("expected invalid agendaDays")
	}
	if err := validateSettingsShape(map[string]any{"showEventMaps": "yes"}); err == nil {
		t.Fatal("expected invalid boolean")
	}
}

func TestSettingsValidateRadarValues(t *testing.T) {
	if err := validateSettingsShape(map[string]any{"radarProvider": "rainviewer"}); err != nil {
		t.Fatal(err)
	}
	if err := validateSettingsShape(map[string]any{"radarProvider": "not-a-provider"}); err == nil {
		t.Fatal("expected unsupported radar provider")
	}
}

func TestUpdateSettingsSerializesConcurrentMutations(t *testing.T) {
	dash := t.TempDir()
	a := &app{dash: dash, configDir: filepath.Join(dash, "config"), cacheDir: filepath.Join(dash, "cache"), settingsFile: filepath.Join(dash, "config", "settings.json")}
	a.ensureDirs()
	if err := a.writeSettings(map[string]any{}); err != nil {
		t.Fatal(err)
	}
	const writers = 12
	start := make(chan struct{})
	errs := make(chan error, writers)
	var wg sync.WaitGroup
	for i := range writers {
		wg.Go(func() {
			<-start
			_, err := a.updateSettings(func(s map[string]any) { s[fmt.Sprintf("writer-%d", i)] = i })
			errs <- err
		})
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	got := a.loadSettings()
	for i := range writers {
		if got[fmt.Sprintf("writer-%d", i)] != float64(i) {
			t.Fatalf("concurrent write %d was lost: %#v", i, got)
		}
	}
}

func TestValidateDashboardTypographySettings(t *testing.T) {
	settings := map[string]any{
		"calendarTextSize":   -0.5,
		"calendarTextWeight": 900,
		"calendarTextFont":   "mono",
		"clockTextSize":      2,
		"clockTextWeight":    400,
		"clockTextFont":      "readable",
		"weatherTextSize":    -2,
		"weatherTextWeight":  800,
		"weatherTextFont":    "rounded",
		"messageTextSize":    1,
		"messageTextWeight":  850,
		"messageTextFont":    "system",
	}
	if err := validateSettingsShape(settings); err != nil {
		t.Fatalf("valid Dashboard Display typography rejected: %v", err)
	}
	for key, value := range map[string]any{
		"calendarTextSize": 0.25,
		"clockTextWeight":  650,
		"weatherTextFont":  "remote",
		"messageTextSize":  "large",
	} {
		bad := map[string]any{key: value}
		if err := validateSettingsShape(bad); err == nil {
			t.Fatalf("invalid Dashboard Display typography %s=%v was accepted", key, value)
		}
	}
}
