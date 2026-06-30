package main

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestConfigBackupRecordsSortsModifiedTimesDescending(t *testing.T) {
	a := testApp(t)
	older := writeConfigBackupFixture(t, a, "older.zip", map[string][]byte{
		"config/settings.json": []byte(`{"theme":"older"}`),
	})
	newer := writeConfigBackupFixture(t, a, "newer.zip", map[string][]byte{
		"config/settings.json": []byte(`{"theme":"newer"}`),
	})
	base := time.Unix(1_700_000_000, 0)
	if err := os.Chtimes(filepath.Join(a.backupDir(), older), base, base); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filepath.Join(a.backupDir(), newer), base.Add(time.Second), base.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	records := a.configBackupRecords()
	if len(records) != 2 {
		t.Fatalf("record count = %d, want 2", len(records))
	}
	if records[0].Name != newer || records[1].Name != older {
		t.Fatalf("records sort = %#v, want %q before %q", records, newer, older)
	}
}

func TestConfigBackupSelectionIgnoresSymlinkedArchives(t *testing.T) {
	a := testApp(t)
	name := writeConfigBackupFixture(t, a, "safe.zip", map[string][]byte{
		"config/settings.json": []byte(`{"theme":"safe"}`),
	})
	linkName := filepath.Join(a.backupDir(), "linked.zip")
	if err := os.Symlink(filepath.Join(a.backupDir(), name), linkName); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	for _, backup := range a.listConfigBackups() {
		if backup["name"] == "linked.zip" {
			t.Fatalf("symlinked archive appeared in backup list: %#v", backup)
		}
	}
	if _, err := a.restoreConfigBackup("linked.zip"); err == nil || !strings.Contains(err.Error(), "backup not found") {
		t.Fatalf("symlinked backup restore error = %v, want not found", err)
	}
	if _, err := a.deleteConfigBackup("linked.zip"); err == nil || !strings.Contains(err.Error(), "backup not found") {
		t.Fatalf("symlinked backup delete error = %v, want not found", err)
	}
	if _, err := os.Stat(filepath.Join(a.backupDir(), name)); err != nil {
		t.Fatalf("trusted backup changed after linked selection rejection: %v", err)
	}
}
