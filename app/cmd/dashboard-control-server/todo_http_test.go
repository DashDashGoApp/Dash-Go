package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTodoStatusEndpointReturnsLocalFirstPayloadWithoutSavedSettings(t *testing.T) {
	a := newTodoTestApp(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/todo/status", nil)
	if !a.handleTodoGet(w, r, "/api/todo/status") {
		t.Fatal("todo status route was not handled")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("todo status returned %d: %s", w.Code, w.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("todo status returned invalid JSON: %v", err)
	}
	if _, exists := payload["enabled"]; exists {
		t.Fatalf("todo status must not expose retired app visibility: %#v", payload)
	}
	mapping, ok := payload["map"].(map[string]any)
	if !ok {
		t.Fatalf("todo status map has wrong JSON shape: %#v", payload["map"])
	}
	if got, _ := mapping["todo"].(string); got != todoLocalTodoListID {
		t.Fatalf("todo status map[todo]=%#v, want %q", mapping["todo"], todoLocalTodoListID)
	}
	if got, _ := mapping["grocery"].(string); got != todoLocalGroceryListID {
		t.Fatalf("todo status map[grocery]=%#v, want %q", mapping["grocery"], todoLocalGroceryListID)
	}
	if got := payload["syncMode"]; got != todoSyncLocal {
		t.Fatalf("todo status syncMode=%#v, want local", got)
	}
	if _, leaksAccessToken := payload["accessToken"]; leaksAccessToken {
		t.Fatalf("todo status leaked access token: %#v", payload)
	}
	if _, leaksRefreshToken := payload["refreshToken"]; leaksRefreshToken {
		t.Fatalf("todo status leaked refresh token: %#v", payload)
	}
}

func TestTodoInboundSyncEndpointNormalizesLegacyCadenceToFixedTwentyFiveSeconds(t *testing.T) {
	a := newTodoTestApp(t)
	for _, legacySeconds := range []int{0, 15, 30, 300, 7} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/todo/inbound-sync", nil)
		if !a.handleTodoPost(w, r, "/api/todo/inbound-sync", map[string]any{"seconds": legacySeconds}) {
			t.Fatal("inbound sync route was not handled")
		}
		if w.Code != http.StatusOK {
			t.Fatalf("legacy cadence %d returned %d: %s", legacySeconds, w.Code, w.Body.String())
		}
		if got := a.todoInboundSyncSeconds(); got != todoInboundSyncFixedSeconds {
			t.Fatalf("legacy cadence %d stored runtime value %d, want fixed %d", legacySeconds, got, todoInboundSyncFixedSeconds)
		}
	}
}

func TestTodoTaskCacheGETDoesNotStartCloudWork(t *testing.T) {
	a := newTodoTestApp(t)
	enableTodoMicrosoftTestList(t, a, "remote")
	if err := a.writeTodoListCache(todoListCache{Version: 1, ListID: "remote", Tasks: []todoTask{{ID: "phone", Title: "Cached", Status: "notStarted"}}}); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/todo/lists/remote/tasks", nil)
	if !a.handleTodoGet(w, r, "/api/todo/lists/remote/tasks") {
		t.Fatal("task cache route was not handled")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("task cache returned %d: %s", w.Code, w.Body.String())
	}
	running, _, _ := a.todoInboundRuntimeForTest()
	if running {
		t.Fatal("cache GET must not start a Microsoft sync")
	}
}

func TestTodoClearCompletedSnapshotLeavesLaterCompletedItems(t *testing.T) {
	a := newTodoTestApp(t)
	cache := todoListCache{Version: 1, ListID: todoLocalGroceryListID, Tasks: []todoTask{
		{ID: "shown", Title: "Shown", Status: "completed"},
		{ID: "later", Title: "Later", Status: "completed"},
		{ID: "open", Title: "Open", Status: "notStarted"},
	}}
	if err := a.writeTodoListCache(cache); err != nil {
		t.Fatal(err)
	}
	next, cleared, err := a.clearTodoCompletedSnapshot(todoLocalGroceryListID, []string{"shown"})
	if err != nil {
		t.Fatal(err)
	}
	if cleared != 1 {
		t.Fatalf("cleared=%d, want one snapshot item", cleared)
	}
	ids := map[string]bool{}
	for _, task := range next.Tasks {
		ids[task.ID] = true
	}
	if !ids["later"] || !ids["open"] || ids["shown"] {
		t.Fatalf("snapshot clear changed the wrong tasks: %#v", next.Tasks)
	}
}

func TestTodoTaskBatchPatchesSelectedCacheRows(t *testing.T) {
	a := newTodoTestApp(t)
	cache := todoListCache{Version: 1, ListID: todoLocalGroceryListID, Tasks: []todoTask{
		{ID: "one", Title: "Milk", Status: "notStarted"},
		{ID: "two", Title: "Eggs", Status: "notStarted"},
	}}
	if err := a.writeTodoListCache(cache); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/todo/lists/local-grocery/tasks/batch", nil)
	body := map[string]any{"patches": []any{
		map[string]any{"id": "one", "patch": map[string]any{"status": "completed"}},
		map[string]any{"id": "two", "patch": map[string]any{"status": "completed"}},
	}}
	if !a.handleTodoPost(w, r, "/api/todo/lists/local-grocery/tasks/batch", body) {
		t.Fatal("task batch route was not handled")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("task batch returned %d: %s", w.Code, w.Body.String())
	}
	next := a.readTodoListCache(todoLocalGroceryListID)
	if len(next.Tasks) != 2 || next.Tasks[0].Status != "completed" || next.Tasks[1].Status != "completed" {
		t.Fatalf("task batch did not persist both local-first changes: %#v", next.Tasks)
	}
}

func TestTodoTaskWriteRejectsOversizedFieldsBeforeCacheMutation(t *testing.T) {
	a := newTodoTestApp(t)
	cache := todoListCache{Version: 1, ListID: todoLocalGroceryListID, Tasks: []todoTask{{ID: "milk", Title: "Milk", Status: "notStarted"}}}
	if err := a.writeTodoListCache(cache); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	if !a.handleTodoPost(w, httptest.NewRequest(http.MethodPost, "/api/todo/lists/local-grocery/tasks", nil), "/api/todo/lists/local-grocery/tasks", map[string]any{"title": strings.Repeat("x", 256)}) {
		t.Fatal("todo task route was not handled")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("oversized title returned %d: %s", w.Code, w.Body.String())
	}
	if got := a.readTodoListCache(todoLocalGroceryListID); len(got.Tasks) != 1 || got.Tasks[0].Title != "Milk" {
		t.Fatalf("oversized write mutated local cache: %#v", got.Tasks)
	}

	w = httptest.NewRecorder()
	path := "/api/todo/lists/" + strings.Repeat("x", 513) + "/tasks"
	if !a.handleTodoPost(w, httptest.NewRequest(http.MethodPost, path, nil), path, map[string]any{"title": "Bread"}) {
		t.Fatal("oversized list id route was not handled")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("oversized list id returned %d: %s", w.Code, w.Body.String())
	}
}
