package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
)

func doctorDataTestApp(t *testing.T) *app {
	t.Helper()
	root := t.TempDir()
	a := &app{
		dash:      root,
		home:      root,
		configDir: filepath.Join(root, "config"),
		calDir:    filepath.Join(root, "calendars"),
		cacheDir:  filepath.Join(root, "cache"),
		logDir:    filepath.Join(root, "logs"),
		binDir:    filepath.Join(root, "bin"),
		fontsDir:  filepath.Join(root, "fonts"),
	}
	a.settingsFile = filepath.Join(a.configDir, "settings.json")
	a.configLocal = filepath.Join(a.configDir, "config.local.js")
	a.ensureDirs()
	return a
}

func doctorDataHas(findings []string, prefix string) bool {
	for _, finding := range findings {
		if strings.HasPrefix(finding, prefix) {
			return true
		}
	}
	return false
}

func TestDoctorDataFindingsDetectSemanticCacheAndSourceProblems(t *testing.T) {
	a := doctorDataTestApp(t)
	if err := fileio.WriteJSON(filepath.Join(a.calDir, "calendars.json"), []any{map[string]any{"url": "calendars/missing.ics"}}); err != nil {
		t.Fatal(err)
	}
	if err := fileio.WriteJSON(filepath.Join(a.cacheDir, "events.cache.json"), map[string]any{"version": 1, "events": []any{}}); err != nil {
		t.Fatal(err)
	}
	if err := fileio.WriteJSON(filepath.Join(a.configDir, "message-cache.json"), map[string]any{"items": "not-an-array"}); err != nil {
		t.Fatal(err)
	}

	findings := a.doctorDataFindings()
	for _, prefix := range []string{"DASHBOARD_FONTS_MISSING:", "CALENDAR_SOURCE_MISSING:calendars/missing.ics", "EVENT_CACHE_INVALID:", "MESSAGE_CACHE_INVALID:"} {
		if !doctorDataHas(findings, prefix) {
			t.Fatalf("expected %q in findings: %#v", prefix, findings)
		}
	}
}

func TestDoctorDataFindingsAcceptValidGeneratedData(t *testing.T) {
	a := doctorDataTestApp(t)
	if err := os.WriteFile(filepath.Join(a.calDir, "home.blue.ics"), []byte("BEGIN:VCALENDAR\nEND:VCALENDAR\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := fileio.WriteJSON(filepath.Join(a.calDir, "calendars.json"), []any{map[string]any{"url": "calendars/home.blue.ics", "name": "Home", "enabled": true}}); err != nil {
		t.Fatal(err)
	}
	if err := fileio.WriteJSON(filepath.Join(a.cacheDir, "events.cache.json"), map[string]any{"version": eventCacheVersion, "generatedAt": 1, "windowStart": 1, "windowEnd": 2, "events": []any{}}); err != nil {
		t.Fatal(err)
	}
	if err := fileio.WriteJSON(filepath.Join(a.configDir, "message-cache.json"), map[string]any{"items": []any{}, "generatedAt": 1, "sources": []any{}, "sourceStatus": []any{}}); err != nil {
		t.Fatal(err)
	}

	findings := a.doctorDataFindings()
	for _, prefix := range []string{"CALENDAR_MANIFEST_OK:", "EVENT_CACHE_OK", "MESSAGE_CACHE_OK"} {
		if !doctorDataHas(findings, prefix) {
			t.Fatalf("expected %q in findings: %#v", prefix, findings)
		}
	}
	for _, prefix := range []string{"CALENDAR_SOURCE_MISSING:", "EVENT_CACHE_INVALID:", "MESSAGE_CACHE_INVALID:"} {
		if doctorDataHas(findings, prefix) {
			t.Fatalf("unexpected %q in valid findings: %#v", prefix, findings)
		}
	}
}
