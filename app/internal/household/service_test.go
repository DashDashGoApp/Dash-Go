package household

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func fixedNow() time.Time { return time.Date(2026, 6, 28, 12, 0, 0, 0, time.Local) }

func TestNormalizeAndMergePreserveCanonicalPeople(t *testing.T) {
	base := map[string]any{"revision": 2, "people": []any{map[string]any{"id": "sam", "name": "Sam", "state": "active", "createdAt": fixedNow().Format(time.RFC3339), "updatedAt": fixedNow().Format(time.RFC3339)}}}
	got := Merge(base, fixedNow(), []any{map[string]any{"id": "sam", "name": "Renamed"}, map[string]any{"id": "alex", "name": "Alex"}})
	rows := got["people"].([]any)
	if len(rows) != 2 {
		t.Fatalf("people count = %d, want 2", len(rows))
	}
	_, sam := Find(rows, "sam")
	if sam == nil {
		t.Fatal("canonical person missing")
	}
	if PersonName(sam) != "Sam" {
		t.Fatalf("canonical person was overwritten: %#v", sam)
	}
}

func TestServicePayloadAndDeleteTarget(t *testing.T) {
	dir := t.TempDir()
	service := New(ServiceConfig{ConfigDir: dir, Now: fixedNow})
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	roster := map[string]any{"revision": 1, "people": []any{
		map[string]any{"id": "sam", "name": "Sam", "state": "active", "createdAt": fixedNow().Format(time.RFC3339), "updatedAt": fixedNow().Format(time.RFC3339)},
		map[string]any{"id": "alex", "name": "Alex", "state": "active", "createdAt": fixedNow().Format(time.RFC3339), "updatedAt": fixedNow().Format(time.RFC3339)},
	}}
	if err := service.Write(roster); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "household-people.json")); err != nil {
		t.Fatal(err)
	}
	next, op, id, err := service.NextRoster(service.Payload(), map[string]any{"op": "delete", "id": "sam"})
	if err != nil || op != "delete" || id != "sam" {
		t.Fatalf("delete plan = %#v %q %q %v", next, op, id, err)
	}
	if target, err := DeleteTarget(next, map[string]any{"resolution": "reassign", "reassignTo": "alex"}, "sam"); err != nil || target != "alex" {
		t.Fatalf("target = %q %v", target, err)
	}
}
