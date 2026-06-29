package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func readBackupZipEntry(t *testing.T, path, name string) []byte {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, readErr := io.ReadAll(rc)
		closeErr := rc.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
		return data
	}
	t.Fatalf("backup entry missing: %s", name)
	return nil
}

func TestConfigBackupPreservesDirectCalendarSymlinkAsMetadata(t *testing.T) {
	a := testApp(t)
	outside := filepath.Join(t.TempDir(), "external.ics")
	original := []byte("BEGIN:VCALENDAR\nSUMMARY:external-only-marker\nEND:VCALENDAR\n")
	if err := os.WriteFile(outside, original, 0644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(a.calDir, "family.ics")
	target, err := filepath.Rel(a.calDir, outside)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	created, err := a.createConfigBackup("manual", "calendar link fixture", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if got := jsonutil.Int(created["calendarLinks"], 0); got != 1 {
		t.Fatalf("calendarLinks = %d, want 1: %#v", got, created)
	}
	archive := created["file"].(string)
	meta := map[string]json.RawMessage{}
	if err := json.Unmarshal(readBackupZipEntry(t, archive, "backup-meta.json"), &meta); err != nil {
		t.Fatal(err)
	}
	var links []calendarBackupLink
	if err := json.Unmarshal(meta["calendarLinks"], &links); err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 || links[0].Path != "family.ics" || links[0].Target != target {
		t.Fatalf("calendar link metadata = %#v, want literal path and target", links)
	}
	zr, err := zip.OpenReader(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name == "calendars/family.ics" {
			t.Fatalf("calendar symlink target was incorrectly copied into backup")
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, readErr := io.ReadAll(rc)
		closeErr := rc.Close()
		if readErr != nil || closeErr != nil {
			t.Fatalf("read %s: %v / %v", f.Name, readErr, closeErr)
		}
		if bytes.Contains(data, []byte("external-only-marker")) {
			t.Fatalf("backup copied linked target content into %s", f.Name)
		}
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.calDir, "stale.ics"), calendarBody("stale"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := a.restoreConfigBackup(created["name"].(string)); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(link)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("calendar link was not restored: info=%#v err=%v", info, err)
	}
	gotTarget, err := os.Readlink(link)
	if err != nil || gotTarget != target {
		t.Fatalf("restored target = %q / %v, want %q", gotTarget, err, target)
	}
	if body, err := os.ReadFile(outside); err != nil || !bytes.Equal(body, original) {
		t.Fatalf("restore touched external target: %v / %q", err, body)
	}
	if _, err := os.Lstat(filepath.Join(a.calDir, "stale.ics")); !os.IsNotExist(err) {
		t.Fatalf("authoritative calendar restore retained stale entry: %v", err)
	}
}

func TestConfigBackupPreservesBrokenCalendarSymlink(t *testing.T) {
	a := testApp(t)
	link := filepath.Join(a.calDir, "offline.ics")
	const target = "../../not-mounted/family.ics"
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	created, err := a.createConfigBackup("manual", "broken calendar link fixture", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if _, err := a.restoreConfigBackup(created["name"].(string)); err != nil {
		t.Fatal(err)
	}
	got, err := os.Readlink(link)
	if err != nil || got != target {
		t.Fatalf("broken link was not restored literally: %q / %v", got, err)
	}
}

func TestConfigBackupRejectsUnsupportedSymlinks(t *testing.T) {
	a := testApp(t)
	target := filepath.Join(t.TempDir(), "target.ics")
	if err := os.WriteFile(target, calendarBody("target"), 0644); err != nil {
		t.Fatal(err)
	}
	configLink := filepath.Join(a.configDir, "linked.json")
	if err := os.Symlink(target, configLink); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := a.createConfigBackup("manual", "config symlink rejection", "", false); err == nil || !strings.Contains(err.Error(), "backup refuses non-regular file") {
		t.Fatalf("config symlink must stay fail-closed, got %v", err)
	}
	if err := os.Remove(configLink); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(a.calDir, "nested"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(a.calDir, "nested", "linked.ics")); err != nil {
		t.Fatal(err)
	}
	if _, err := a.createConfigBackup("manual", "nested calendar link rejection", "", false); err == nil || !strings.Contains(err.Error(), "unsupported calendar symlink") {
		t.Fatalf("nested calendar link must stay fail-closed, got %v", err)
	}
}

func TestRestoreConfigBackupWithoutCalendarLinkMetadataRemainsSupported(t *testing.T) {
	a := testApp(t)
	live := filepath.Join(a.configDir, "settings.json")
	if err := os.WriteFile(live, []byte(`{"theme":"live"}`), 0644); err != nil {
		t.Fatal(err)
	}
	name := writeConfigBackupFixture(t, a, "legacy-no-calendar-link-metadata.zip", map[string][]byte{
		"config/settings.json": []byte(`{"theme":"legacy"}`),
		"calendars/legacy.ics": calendarBody("legacy"),
	})
	if _, err := a.restoreConfigBackup(name); err != nil {
		t.Fatalf("legacy backup without metadata must remain restorable: %v", err)
	}
	settings, err := os.ReadFile(live)
	if err != nil || !bytes.Contains(settings, []byte("legacy")) {
		t.Fatalf("legacy settings were not restored: %q / %v", settings, err)
	}
	if _, err := os.Stat(filepath.Join(a.calDir, "legacy.ics")); err != nil {
		t.Fatalf("legacy calendar was not restored: %v", err)
	}
}

func TestRestoreConfigBackupRejectsUnsafeOrConflictingCalendarLinkMetadata(t *testing.T) {
	for _, tc := range []struct {
		name  string
		meta  string
		files map[string][]byte
		want  string
	}{
		{
			name:  "unsafe path",
			meta:  `{"calendarLinks":[{"path":"../outside.ics","target":"/tmp/outside.ics"}]}`,
			files: map[string][]byte{"config/settings.json": []byte(`{"theme":"backup"}`)},
			want:  "unsafe calendar link path",
		},
		{
			name:  "duplicate calendar file",
			meta:  `{"calendarLinks":[{"path":"shared.ics","target":"/tmp/shared.ics"}]}`,
			files: map[string][]byte{"calendars/shared.ics": calendarBody("regular")},
			want:  "conflicts with archived file",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := testApp(t)
			live := filepath.Join(a.configDir, "settings.json")
			if err := os.WriteFile(live, []byte(`{"theme":"live"}`), 0644); err != nil {
				t.Fatal(err)
			}
			files := map[string][]byte{"backup-meta.json": []byte(tc.meta)}
			for name, data := range tc.files {
				files[name] = data
			}
			name := writeConfigBackupFixture(t, a, "calendar-link-metadata.zip", files)
			if _, err := a.restoreConfigBackup(name); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("restore error = %v, want %q", err, tc.want)
			}
			got, err := os.ReadFile(live)
			if err != nil || !bytes.Contains(got, []byte("live")) {
				t.Fatalf("live config changed after rejected calendar metadata: %q / %v", got, err)
			}
		})
	}
}
