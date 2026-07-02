package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Runtime state belongs below a configured dashboard data directory, never in
// the command package. This rejects accidentally committed task/list JSON and
// keeps source handoffs free of household-like test fallout.
func TestControlServerSourceContainsNoRuntimeJSONArtifacts(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate control-server source directory")
	}
	dir := filepath.Dir(thisFile)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		t.Fatalf("runtime JSON artifact is committed beside command source: %s", entry.Name())
	}
}

func TestSharedTestAppUsesDedicatedTodoRuntimeDirectory(t *testing.T) {
	a := testApp(t)
	if strings.TrimSpace(a.todoDir) == "" {
		t.Fatal("shared test app has no Todo runtime directory")
	}
	if err := a.writeTodoListCache(todoListCache{Version: 1, ListID: todoLocalGroceryListID}); err != nil {
		t.Fatal(err)
	}
	if got := filepath.Dir(a.todoListPath(todoLocalGroceryListID)); got != a.todoDir {
		t.Fatalf("Todo test cache directory=%q, want %q", got, a.todoDir)
	}
}
