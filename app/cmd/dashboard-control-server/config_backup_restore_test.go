package main

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfigBackupFixture(t *testing.T, a *app, name string, files map[string][]byte) string {
	t.Helper()
	if err := os.MkdirAll(a.backupDir(), 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(a.backupDir(), name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	for entry, data := range files {
		w, err := z.Create(entry)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return name
}

func TestRestoreConfigBackupReplacesAuthoritativeTrees(t *testing.T) {
	a := testApp(t)
	if err := os.WriteFile(filepath.Join(a.configDir, "settings.json"), []byte(`{"theme":"before"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.calDir, "before.ics"), []byte("BEGIN:VCALENDAR\nEND:VCALENDAR\n"), 0644); err != nil {
		t.Fatal(err)
	}
	created, err := a.createConfigBackup("manual", "fixture", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.configDir, "settings.json"), []byte(`{"theme":"after"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.configDir, "stray.json"), []byte(`{"stray":true}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.calDir, "after.ics"), []byte("BEGIN:VCALENDAR\nEND:VCALENDAR\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := a.restoreConfigBackup(created["name"].(string)); err != nil {
		t.Fatal(err)
	}
	settings, err := os.ReadFile(filepath.Join(a.configDir, "settings.json"))
	if err != nil || !bytes.Contains(settings, []byte("before")) {
		t.Fatalf("settings were not restored faithfully: %q / %v", settings, err)
	}
	for _, path := range []string{filepath.Join(a.configDir, "stray.json"), filepath.Join(a.calDir, "after.ics")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("stale path survived authoritative restore: %s (%v)", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(a.calDir, "before.ics")); err != nil {
		t.Fatalf("backup calendar missing after restore: %v", err)
	}
}

func TestRestoreConfigBackupRejectsOversizedEntryWithoutTouchingLiveState(t *testing.T) {
	a := testApp(t)
	live := filepath.Join(a.configDir, "settings.json")
	if err := os.WriteFile(live, []byte(`{"theme":"live"}`), 0644); err != nil {
		t.Fatal(err)
	}
	name := writeConfigBackupFixture(t, a, "oversized.zip", map[string][]byte{
		"config/too-large.json": bytes.Repeat([]byte("x"), int(maxConfigBackupEntryBytes)+1),
	})
	if _, err := a.restoreConfigBackup(name); err == nil || !strings.Contains(err.Error(), "entry too large") {
		t.Fatalf("expected oversized backup rejection, got %v", err)
	}
	got, err := os.ReadFile(live)
	if err != nil || !bytes.Contains(got, []byte("live")) {
		t.Fatalf("live settings changed after rejected archive: %q / %v", got, err)
	}
}
