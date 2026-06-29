package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUpdaterCapabilitiesDeclareRequiredReleaseOperations(t *testing.T) {
	for _, required := range []string{
		updaterCapabilityProtocol,
		"release-manifest-v1",
		"github-release-resolution-v3",
		"release-file-list-v1",
		"stale-source-purge-v1",
		"update-status-v1",
		"update-job-v1",
		"update-action-history-v1",
	} {
		if !updaterHasCapability(required) {
			t.Fatalf("missing updater capability %q", required)
		}
	}
}

func TestWriteUpdaterMigrationReceiptIsPrivateAndVersioned(t *testing.T) {
	dir := t.TempDir()
	a := &app{releaseVersion: "1.4.3-beta.72"}
	path := filepath.Join(dir, "cache", "updater-migration-v1.json")
	if rc := a.runWriteUpdaterMigrationCLI([]string{"--file", path, "--previous-version", "1.4.3-beta.71", "--architecture", "armv6l"}); rc != 0 {
		t.Fatalf("migration receipt command returned %d", rc)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("migration receipt mode = %o, want 0600", info.Mode().Perm())
	}
	value := readHealthFile(path)
	if value["installedVersion"] != "1.4.3-beta.72" || value["previousVersion"] != "1.4.3-beta.71" {
		t.Fatalf("unexpected migration receipt: %#v", value)
	}
	if value["releaseManifestReady"] != true {
		t.Fatalf("migration receipt did not record release-manifest capability: %#v", value)
	}
}
