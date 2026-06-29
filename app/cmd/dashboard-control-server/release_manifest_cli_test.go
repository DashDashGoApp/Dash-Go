package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReleaseInstallFilesExcludesMutableAndMetadata(t *testing.T) {
	manifest := releaseManifest{Version: "1.4.3-beta.71", Files: []releaseManifestFile{
		{Path: "index.html"}, {Path: "ui/js/app-launcher.js"}, {Path: "config/settings.json"},
		{Path: "cache/events.cache.json"}, {Path: "calendars/home.ics"}, {Path: "logs/update.log"},
		{Path: "releases/beta/old.tar.gz"}, {Path: "install.sh"}, {Path: "AI.md"},
	}}
	got := releaseInstallFiles(manifest)
	want := []string{"index.html", "manifest.json", "ui/js/app-launcher.js"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("release install files=%#v want=%#v", got, want)
	}
}

func TestReadReleaseManifestRejectsUnsafePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(path, []byte(`{"version":"1.4.3-beta.71","files":[{"path":"../config/settings.json","sha256":"x"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readReleaseManifest(path); err == nil {
		t.Fatal("unsafe manifest path unexpectedly accepted")
	}
}

func TestReadReleaseManifestRejectsOversizedInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "manifest.json")
	data := make([]byte, maxReleaseManifestBytes+1)
	for i := range data {
		data[i] = 'x'
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readReleaseManifest(path); err == nil {
		t.Fatal("oversized manifest unexpectedly accepted")
	}
}
