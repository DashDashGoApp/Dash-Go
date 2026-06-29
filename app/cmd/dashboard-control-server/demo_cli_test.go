package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClearDemoModeRemovesFlagsWithoutReseeding(t *testing.T) {
	a := newTempCalendarApp(t)
	if err := a.writeSettings(map[string]any{"demoMode": true, "locationName": "Chicago"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.configLocal, []byte("window.DASHBOARD_LOCAL = { demoMode: true, lat: 41.8 };\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.cacheDir, "demo-mode.json"), []byte(`{"enabled":true}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.calDir, "demo-family.green.ics"), []byte("BEGIN:VCALENDAR\nEND:VCALENDAR\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := a.clearDemoMode(false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(a.cacheDir, "demo-mode.json")); !os.IsNotExist(err) {
		t.Fatalf("demo marker still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(a.calDir, "demo-family.green.ics")); !os.IsNotExist(err) {
		t.Fatalf("demo calendar still exists: %v", err)
	}
	settings := a.loadSettings()
	if _, ok := settings["demoMode"]; ok {
		t.Fatalf("demoMode remained in settings: %#v", settings)
	}
	body, err := os.ReadFile(a.configLocal)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "demoMode") || !strings.Contains(string(body), "lat: 41.8") {
		t.Fatalf("config.local.js was not cleanly preserved without demo flag: %s", body)
	}
}
