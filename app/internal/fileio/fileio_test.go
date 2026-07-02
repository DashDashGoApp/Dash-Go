package fileio

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteJSONAndReadString(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "nested", "settings.json")
	if err := WriteJSON(path, map[string]any{"name": "Family"}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(body), "\n") || !strings.Contains(string(body), "\"name\": \"Family\"") {
		t.Fatalf("WriteJSON body = %q", body)
	}
	if got := ReadString(path, "fallback"); !strings.Contains(got, "\"name\": \"Family\"") {
		t.Fatalf("ReadString JSON = %q", got)
	}
	if got := ReadString(filepath.Join(root, "missing"), "fallback"); got != "fallback" {
		t.Fatalf("ReadString missing = %q", got)
	}
	if err := os.WriteFile(filepath.Join(root, "empty"), []byte("  \n"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := ReadString(filepath.Join(root, "empty"), "fallback"); got != "fallback" {
		t.Fatalf("ReadString blank = %q", got)
	}
}

func TestWriteAtomicAndExists(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "nested", "payload.txt")
	if err := WriteAtomic(path, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	if !Exists(path) || Exists(filepath.Dir(path)) {
		t.Fatalf("Exists path/dir contract failed")
	}
	if err := WriteAtomic(path, []byte("second"), 0600); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "second" {
		t.Fatalf("WriteAtomic replacement = %q", body)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("WriteAtomic mode = %o", info.Mode().Perm())
	}
}

func TestAtomicWritersLeaveNoStagingFilesAfterDurableReplacement(t *testing.T) {
	root := t.TempDir()
	atomicPath := filepath.Join(root, "state", "payload.txt")
	jsonPath := filepath.Join(root, "state", "payload.json")
	if err := WriteAtomic(atomicPath, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := WriteAtomic(atomicPath, []byte("second"), 0640); err != nil {
		t.Fatal(err)
	}
	if err := WriteJSON(jsonPath, map[string]any{"ok": true}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(atomicPath)
	if err != nil || string(body) != "second" {
		t.Fatalf("atomic replacement body=%q err=%v", body, err)
	}
	info, err := os.Stat(atomicPath)
	if err != nil || info.Mode().Perm() != 0640 {
		t.Fatalf("atomic replacement mode=%v err=%v", info.Mode(), err)
	}
	entries, err := os.ReadDir(filepath.Dir(atomicPath))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp-") {
			t.Fatalf("durable writer left temporary staging file %q", entry.Name())
		}
	}
}
