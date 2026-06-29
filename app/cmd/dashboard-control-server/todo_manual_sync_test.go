package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTodoManualListSyncQueuesOneRequestAndEnforcesPerListCooldown(t *testing.T) {
	if todoManualListSyncCooldown != 25*time.Second {
		t.Fatalf("manual sync cooldown = %s, want 25s", todoManualListSyncCooldown)
	}
	a := newTodoTestApp(t)
	enableTodoMicrosoftTestList(t, a, "remote")
	a.todoSetInboundRunningForTest(true)

	first := a.todoRequestManualListSync("remote")
	if !first.Accepted || !first.Queued || first.Started {
		t.Fatalf("first manual sync result=%#v, want accepted queued request", first)
	}
	if first.ManualSync.CooldownSeconds < 1 || !first.ManualSync.Running {
		t.Fatalf("first manual sync status=%#v, want running cooldown", first.ManualSync)
	}
	second := a.todoRequestManualListSync("remote")
	if second.Accepted || second.ManualSync.CooldownSeconds < 1 {
		t.Fatalf("second manual sync bypassed cooldown: %#v", second)
	}
	_, queued, queuedListsMap := a.todoInboundRuntimeForTest()
	queuedLists := queuedListsMap["remote"]
	if !queued || !queuedLists {
		t.Fatalf("manual sync did not use the bounded coordinator queue")
	}
}

func TestTodoManualListSyncEndpointOnlyAdmitsMicrosoftBackedList(t *testing.T) {
	a := newTodoTestApp(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/todo/lists/local-grocery/sync-now", nil)
	if !a.handleTodoPost(w, r, "/api/todo/lists/local-grocery/sync-now", map[string]any{}) {
		t.Fatal("manual list sync route was not handled")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("local manual sync returned %d: %s", w.Code, w.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("local manual sync payload: %v", err)
	}
	if accepted, _ := payload["accepted"].(bool); accepted {
		t.Fatalf("local-only list should not admit a Microsoft manual sync: %#v", payload)
	}
}

func TestTodoManualListSyncCooldownDoesNotPersistAcrossProcessStart(t *testing.T) {
	a := newTodoTestApp(t)
	enableTodoMicrosoftTestList(t, a, "remote")
	a.todoSetManualSyncUntilForTest(map[string]time.Time{"remote": time.Now().Add(todoManualListSyncCooldown)})
	if status := a.todoManualListSyncStatus("remote"); status.CooldownSeconds < 1 {
		t.Fatalf("expected active runtime cooldown, got %#v", status)
	}
	fresh := newTodoTestApp(t)
	enableTodoMicrosoftTestList(t, fresh, "remote")
	if status := fresh.todoManualListSyncStatus("remote"); status.CooldownSeconds != 0 || !status.Available {
		t.Fatalf("manual cooldown must remain process-local, got %#v", status)
	}
}
