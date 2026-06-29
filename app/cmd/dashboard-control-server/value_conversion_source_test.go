package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTrimmedFmtSprintExtractionIsCentralized(t *testing.T) {
	projectRoot := filepath.Clean(filepath.Join(".", "..", ".."))
	sources := []string{
		filepath.Join(projectRoot, "cmd", "dashboard-control-server"),
		filepath.Join(projectRoot, "internal", "jsonutil"),
	}
	const antiPattern = "strings.TrimSpace(fmt.Sprint("
	const allowed = "internal/jsonutil/jsonutil.go"
	for _, dir := range sources {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			rel, err := filepath.Rel(projectRoot, path)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(data), antiPattern) && filepath.ToSlash(rel) != allowed {
				t.Fatalf("%s reintroduced direct %s extraction", rel, antiPattern)
			}
		}
	}
}
