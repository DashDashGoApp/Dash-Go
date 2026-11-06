package todo

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// captureStdLog redirects the standard logger for the duration of the test
// and returns the buffer so corrupt-state diagnostics can be asserted.
func captureStdLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	prev := log.Writer()
	log.SetOutput(buf)
	t.Cleanup(func() { log.SetOutput(prev) })
	return buf
}

func TestReadTodoTokenStoreFallsBackOnCorruption(t *testing.T) {
	service := testTodoService(t)
	buf := captureStdLog(t)
	if err := os.WriteFile(service.todoTokenFile, []byte("{not json"), 0600); err != nil {
		t.Fatal(err)
	}
	store := service.readTodoTokenStore()
	if store.AccessToken != "" || store.RefreshToken != "" || store.Account != "" {
		t.Fatalf("corrupt token store must fall back to unlinked: %#v", store)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 || !strings.Contains(lines[0], "token store") {
		t.Fatalf("expected exactly one diagnostic, got %q", buf.String())
	}
}

func TestReadTodoListsIndexFallsBackOnCorruption(t *testing.T) {
	service := testTodoService(t)
	if err := os.MkdirAll(filepath.Dir(service.todoIndexPath()), 0755); err != nil {
		t.Fatal(err)
	}
	buf := captureStdLog(t)
	if err := os.WriteFile(service.todoIndexPath(), []byte("[{broken"), 0600); err != nil {
		t.Fatal(err)
	}
	idx := service.readTodoListsIndex()
	if len(idx.Lists) == 0 {
		t.Fatal("fallback must merge the default local lists")
	}
	found := map[string]bool{}
	for _, item := range idx.Lists {
		found[item.ID] = true
	}
	if !found[LocalTodoListID] || !found[LocalGroceryListID] {
		t.Fatalf("default lists missing from fallback: %#v", idx.Lists)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 || !strings.Contains(lines[0], "list index") {
		t.Fatalf("expected exactly one diagnostic, got %q", buf.String())
	}
}
