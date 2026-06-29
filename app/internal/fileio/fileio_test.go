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
