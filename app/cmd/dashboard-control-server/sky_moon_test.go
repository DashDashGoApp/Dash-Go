package main

import (
	"os"
	"path/filepath"
	"testing"
)

func testApp(t *testing.T) *app {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	dash := filepath.Join(root, "dashboard")
	for _, dir := range []string{home, dash, filepath.Join(dash, "config"), filepath.Join(dash, "calendars"), filepath.Join(dash, "cache"), filepath.Join(dash, "logs"), filepath.Join(dash, "bin")} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	a := &app{dash: dash, home: home, configDir: filepath.Join(dash, "config"), calDir: filepath.Join(dash, "calendars"), cacheDir: filepath.Join(dash, "cache"), logDir: filepath.Join(dash, "logs"), binDir: filepath.Join(dash, "bin"), settingsFile: filepath.Join(dash, "config", "settings.json"), configLocal: filepath.Join(dash, "config", "config.local.js"), celebrationsFile: filepath.Join(home, ".dashboard-celebrations")}
	if err := os.WriteFile(a.configLocal, []byte(`window.DASH_CONFIG={lat:41.8781,lon:-87.6298,locationName:"Chicago"};\n`), 0644); err != nil {
		t.Fatal(err)
	}
	return a
}

func TestGenerateMoonCalendarGo(t *testing.T) {
	a := testApp(t)
	meta := a.generateMoonCalendar(true)
	if meta["ok"] != true {
		t.Fatalf("moon generation failed: %#v", meta)
	}
	if int(meta["eventCount"].(int)) < 100 {
		t.Fatalf("expected many moon events, got %#v", meta["eventCount"])
	}
	if _, err := os.Stat(filepath.Join(a.calDir, "moon.violet.ics")); err != nil {
		t.Fatalf("moon ics missing: %v", err)
	}
	st := a.moonCalendarStatus()
	if st["generator"] != "go" {
		t.Fatalf("expected go status, got %#v", st)
	}
}

func TestGenerateStaticSkyCalendarsGo(t *testing.T) {
	a := testApp(t)
	if err := os.WriteFile(filepath.Join(a.home, ".dashboard-default-calendars"), []byte("DEFAULT_METEOR_SHOWERS=1\nDEFAULT_SUPERMOONS=1\nDEFAULT_ECLIPSES=1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	res := a.generateStaticSkyCalendars()
	if res["ok"] != true {
		t.Fatalf("sky generation failed: %#v", res)
	}
	for _, name := range []string{"meteor-showers.indigo.sky.ics", "supermoons.violet.sky.ics", "eclipses.slate.sky.ics"} {
		if _, err := os.Stat(filepath.Join(a.calDir, name)); err != nil {
			t.Fatalf("%s missing: %v", name, err)
		}
	}
}
