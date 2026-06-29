package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
)

func testProfileApp(t *testing.T) *app {
	t.Helper()
	dash := t.TempDir()
	a := &app{
		dash: dash, home: dash,
		configDir: filepath.Join(dash, "config"), calDir: filepath.Join(dash, "calendars"),
		cacheDir: filepath.Join(dash, "cache"), logDir: filepath.Join(dash, "logs"),
		binDir: filepath.Join(dash, "bin"), settingsFile: filepath.Join(dash, "config", "settings.json"), configLocal: filepath.Join(dash, "config", "config.local.js"),
	}
	a.ensureDirs()
	return a
}

func TestMissingSettingsAreNotSeeded(t *testing.T) {
	a := testProfileApp(t)
	if got := a.loadSettings(); len(got) != 0 {
		t.Fatalf("missing settings should not receive defaults: %#v", got)
	}
	if _, err := os.Stat(a.settingsFile); !os.IsNotExist(err) {
		t.Fatalf("loadSettings created user settings: %v", err)
	}
}

func TestApplyProfilePresetKeepsPersonalAndLegacySettings(t *testing.T) {
	a := testProfileApp(t)
	// Settings are persisted as JSON, so decoded numeric values are float64.
	// Keep this legacy fixture valid under the current safety boundary while
	// proving that applying a profile does not overwrite retired settings.
	personal := map[string]any{
		"fontPreset": "readable", "tempUnit": "fahrenheit", "firstDayOfWeek": float64(1),
		"maxEventsPerCell": float64(6), "agendaDays": float64(12), "refreshWxMinutes": float64(5),
		"showEventMaps": false, "pixelShiftEnabled": false, "profile": "balanced",
	}
	if err := fileio.WriteJSON(a.settingsFile, personal); err != nil {
		t.Fatal(err)
	}
	if _, err := a.applyProfilePreset("lite"); err != nil {
		t.Fatal(err)
	}
	settings := a.loadSettings()
	if got := settings["profile"]; got != "lite" {
		t.Fatalf("profile=%v", got)
	}
	for key, want := range personal {
		if key == "profile" {
			continue
		}
		if got := settings[key]; got != want {
			t.Fatalf("personal/legacy %s changed: got=%T %v want=%T %v", key, got, got, want, want)
		}
	}
	values := a.profilePayload()["values"].(map[string]any)
	if values["showSeconds"] != true {
		t.Fatalf("Lite clock seconds=%v, want true", values["showSeconds"])
	}
	if _, leaked := values["maxEventsPerCell"]; leaked {
		t.Fatal("retired density cap leaked into profile values")
	}
}

func TestProfilePresetsKeepSecondsOn(t *testing.T) {
	for _, name := range []string{"lite", "balanced", "enhanced"} {
		p, ok := profileByName(name)
		if !ok {
			t.Fatalf("missing profile %s", name)
		}
		if p.Values["showSeconds"] != true {
			t.Fatalf("%s showSeconds=%v, want true", name, p.Values["showSeconds"])
		}
	}
}

func TestProfileCustomStateTracksRetainedControls(t *testing.T) {
	a := testProfileApp(t)
	if _, err := a.applyProfilePreset("lite"); err != nil {
		t.Fatal(err)
	}
	payload, err := a.updateProfileValues(map[string]any{
		"weeksBelow": 9, "showInteractiveMaps": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := payload["current"]; got != "custom" {
		t.Fatalf("current=%v, want custom", got)
	}
	if got := payload["base"]; got != "lite" {
		t.Fatalf("base=%v, want lite", got)
	}
	diverged, ok := payload["diverged"].([]string)
	if !ok || len(diverged) != 2 || diverged[0] != "weeksBelow" || diverged[1] != "showInteractiveMaps" {
		t.Fatalf("diverged=%#v", payload["diverged"])
	}
	if _, err := a.applyProfilePreset("lite"); err != nil {
		t.Fatal(err)
	}
	if got := a.profilePayload()["current"]; got != "lite" {
		t.Fatalf("current after reset=%v, want lite", got)
	}
}

func TestProfileRejectsRetiredTuningControls(t *testing.T) {
	a := testProfileApp(t)
	if _, err := a.applyProfilePreset("balanced"); err != nil {
		t.Fatal(err)
	}
	for _, set := range []map[string]any{
		{"agendaDays": 31}, {"maxEventsPerCell": 6}, {"refreshCalMinutes": 5},
		{"refreshWxMinutes": 30}, {"weatherDays": 16}, {"complimentSeconds": 12},
		{"complimentFadeMs": 300}, {"showEventMaps": false}, {"pixelShiftEnabled": false},
		{"radarProvider": "nws"}, {"showInteractiveMaps": "maybe"}, {"unknown": 1},
	} {
		if _, err := a.updateProfileValues(set); err == nil {
			t.Fatalf("retired or invalid Profile set %#v unexpectedly succeeded", set)
		}
	}
}

func TestProfileRetainedEditsAreBoundedAndAlertCadenceAutomatic(t *testing.T) {
	a := testProfileApp(t)
	if _, err := a.applyProfilePreset("balanced"); err != nil {
		t.Fatal(err)
	}
	payload, err := a.updateProfileValues(map[string]any{
		"weeksAbove": 3, "weeksBelow": 12, "rowHeight": 220, "sidebarWidth": 420,
		"showSeconds": false, "showInteractiveMaps": true,
		"weatherAlerts": map[string]any{"enabled": false, "refreshMinutes": 12, "minSeverity": "severe"},
	})
	if err != nil {
		t.Fatal(err)
	}
	values := payload["values"].(map[string]any)
	if values["weeksAbove"] != 3 || values["weeksBelow"] != 12 || values["rowHeight"] != 220 || values["sidebarWidth"] != 420 {
		t.Fatalf("retained numeric values=%#v", values)
	}
	if values["showSeconds"] != false || values["showInteractiveMaps"] != true {
		t.Fatalf("retained bool values=%#v", values)
	}
	alerts := values["weatherAlerts"].(map[string]any)
	if alerts["enabled"] != false || alerts["minSeverity"] != "severe" || alerts["refreshMinutes"] != 5 {
		t.Fatalf("weather alerts=%#v", alerts)
	}
	for _, set := range []map[string]any{{"weeksBelow": 1}, {"rowHeight": 149}, {"sidebarWidth": 521}} {
		if _, err := a.updateProfileValues(set); err == nil {
			t.Fatalf("out-of-range set %#v unexpectedly succeeded", set)
		}
	}
}

func TestProfileChangedSettingsExposeProfileDefaultAndCurrent(t *testing.T) {
	a := testProfileApp(t)
	if _, err := a.applyProfilePreset("lite"); err != nil {
		t.Fatal(err)
	}
	payload, err := a.updateProfileValues(map[string]any{
		"weeksBelow":    9,
		"weatherAlerts": map[string]any{"enabled": false, "refreshMinutes": 99, "minSeverity": "severe"},
	})
	if err != nil {
		t.Fatal(err)
	}
	rows, ok := payload["changedSettings"].([]map[string]any)
	if !ok || len(rows) != 2 {
		t.Fatalf("changedSettings=%#v", payload["changedSettings"])
	}
	byKey := map[string]map[string]any{}
	for _, row := range rows {
		key, _ := row["key"].(string)
		byKey[key] = row
	}
	weeks := byKey["weeksBelow"]
	if weeks["default"] != 8 || weeks["current"] != 9 {
		t.Fatalf("weeksBelow comparison=%#v", weeks)
	}
	alerts := byKey["weatherAlerts"]
	defaultAlerts, ok := alerts["default"].(map[string]any)
	if !ok || defaultAlerts["enabled"] != true || defaultAlerts["refreshMinutes"] != 5 {
		t.Fatalf("weatherAlerts default=%#v", alerts["default"])
	}
	currentAlerts, ok := alerts["current"].(map[string]any)
	if !ok || currentAlerts["enabled"] != false || currentAlerts["refreshMinutes"] != 5 || currentAlerts["minSeverity"] != "severe" {
		t.Fatalf("weatherAlerts current=%#v", alerts["current"])
	}
}

func TestLayoutProfileDoesNotCreateVisibleProfileDivergence(t *testing.T) {
	a := testProfileApp(t)
	if _, err := a.applyProfilePreset("balanced"); err != nil {
		t.Fatal(err)
	}
	settings := a.loadSettings()
	settings["layoutProfile"] = "legacy-manual"
	payload := a.profilePayloadForSettings(settings)
	if got := payload["current"]; got != "balanced" {
		t.Fatalf("current=%v, want balanced", got)
	}
	if got := payload["custom"]; got != false {
		t.Fatalf("custom=%v, want false", got)
	}
	if rows, ok := payload["changedSettings"].([]map[string]any); !ok || len(rows) != 0 {
		t.Fatalf("changedSettings=%#v, want empty", payload["changedSettings"])
	}
	if keys, ok := payload["diverged"].([]string); !ok || len(keys) != 0 {
		t.Fatalf("diverged=%#v, want empty", payload["diverged"])
	}
}
