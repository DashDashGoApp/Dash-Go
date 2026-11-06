package settings

import (
	"reflect"
	"testing"
)

// TestWarmLoadReturnsSharedSnapshotWithoutClone asserts the O4 contract: after
// the first (cold) read, subsequent warm Loads hand back the very same map
// instead of cloning it.
func TestWarmLoadReturnsSharedSnapshotWithoutClone(t *testing.T) {
	service := testService(t)
	if err := service.Write(map[string]any{"profile": "balanced"}); err != nil {
		t.Fatal(err)
	}
	first := service.Load()
	second := service.Load()
	if got, want := reflect.ValueOf(second).Pointer(), reflect.ValueOf(first).Pointer(); got != want {
		t.Fatal("warm Load must return the shared snapshot, not a clone")
	}
	if first["profile"] != "balanced" {
		t.Fatalf("snapshot content wrong: %#v", first)
	}
}

// TestMutateWorkingCopyDoesNotCorruptSharedSnapshot asserts the O4 contract:
// the map handed to the Mutate callback (and returned by it) is a private
// working clone, so mutation — during or after the call — can never reach
// the service snapshot or the persisted file.
func TestMutateWorkingCopyDoesNotCorruptSharedSnapshot(t *testing.T) {
	service := testService(t)
	if err := service.Write(map[string]any{"profile": "balanced"}); err != nil {
		t.Fatal(err)
	}
	before := service.Load()
	if _, err := service.Mutate(func(values map[string]any) error {
		values["theme"] = "solar"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if before["theme"] != nil {
		t.Fatalf("mutate callback wrote through to the shared snapshot: %#v", before)
	}
	if _, err := service.Mutate(func(values map[string]any) error {
		values["scratch"] = "before"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	mutated, err := service.Mutate(func(values map[string]any) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	mutated["scratch"] = "corrupted"
	mutated["injected"] = true
	got := service.Load()
	if got["scratch"] != "before" || got["injected"] != nil {
		t.Fatalf("post-hoc mutation of the Mutate result reached the snapshot: %#v", got)
	}
	if got["theme"] != "solar" {
		t.Fatalf("persisted mutation missing from snapshot: %#v", got)
	}
}
