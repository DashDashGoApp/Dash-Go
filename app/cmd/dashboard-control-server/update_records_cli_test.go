package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteJSONPrivateFileUsesPrivateModesAndAtomicContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private", "status.json")
	value := map[string]any{"state": "queued", "count": 2}
	if err := writeJSONPrivateFile(path, value); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("file mode=%#o want 0600", info.Mode().Perm())
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if dirInfo.Mode().Perm() != 0700 {
		t.Fatalf("directory mode=%#o want 0700", dirInfo.Mode().Perm())
	}
	var got map[string]any
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got["state"] != "queued" || got["count"] != float64(2) {
		t.Fatalf("private JSON=%#v", got)
	}
}

func TestUpdateRecordKeepsJobIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "update-job.json")
	if got := runUpdateRecordCLI([]string{"--file", path, "--job-id", "job-123", "--state", "queued"}, "update-job"); got != 0 {
		t.Fatalf("first record exit=%d", got)
	}
	if got := runUpdateRecordCLI([]string{"--file", path, "--state", "running"}, "update-job"); got != 0 {
		t.Fatalf("second record exit=%d", got)
	}
	var record map[string]any
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	if record["id"] != "job-123" || record["state"] != "running" {
		t.Fatalf("job record=%#v", record)
	}
}
