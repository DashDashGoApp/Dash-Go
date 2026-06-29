package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func enableTodoMicrosoftTestLists(t *testing.T, a *app, todoID, groceryID string) {
	t.Helper()
	if _, err := a.writeTodoSettings(func(todo map[string]any) {
		todo["syncMode"] = todoSyncMicrosoft
		todo["map"] = map[string]any{"todo": todoID, "grocery": groceryID}
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.writeTodoTokenStore(todoTokenStore{
		ClientID:        "client",
		RefreshToken:    "refresh",
		AccessToken:     "access",
		AccessExpiresAt: time.Now().Add(time.Hour).UnixMilli(),
	}); err != nil {
		t.Fatal(err)
	}
	for id, name := range map[string]string{todoID: "To Do", groceryID: "Grocery"} {
		if err := a.todoUpsertListInfo(todoListInfo{ID: id, DisplayName: name, Origin: todoListOriginMicrosoft}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestTodoGraphRequestUsesDocumentedHeadersForDeltaGET(t *testing.T) {
	a := newTodoTestApp(t)
	req, err := a.todoGraphRequest(context.Background(), http.MethodGet, "https://example.invalid/delta", nil, "access")
	if err != nil {
		t.Fatal(err)
	}
	if got := req.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("GET Content-Type = %q, want application/json", got)
	}
	if got := req.Header.Get("Accept"); got != "application/json" {
		t.Fatalf("GET Accept = %q, want application/json", got)
	}
	if got := req.Header.Get("Prefer"); got != "odata.maxpagesize=50" {
		t.Fatalf("GET Prefer = %q, want bounded Graph page size", got)
	}
}

func TestTodoInitialDeltaEndpointsUseBareCompatibleRoutes(t *testing.T) {
	const listID = "AQMk+with/slash=="
	if got, want := todoInitialListsDeltaEndpoint(), "/me/todo/lists/delta"; got != want {
		t.Fatalf("initial list delta endpoint=%q, want %q", got, want)
	}
	if got, want := todoInitialTaskDeltaEndpoint(listID), "/me/todo/lists/AQMk+with%2Fslash==/tasks/delta"; got != want {
		t.Fatalf("initial task delta endpoint=%q, want %q", got, want)
	}
	for name, endpoint := range map[string]string{
		"lists": todoInitialListsDeltaEndpoint(),
		"tasks": todoInitialTaskDeltaEndpoint(listID),
	} {
		if strings.Contains(endpoint, "?") || strings.Contains(endpoint, "$select") {
			t.Fatalf("%s initial delta endpoint must be bare, got %q", name, endpoint)
		}
	}
}

func TestTodoApplyDeltaCoalescesSparseReplayAndRestoredTask(t *testing.T) {
	cache := todoListCache{Version: 1, ListID: "remote", Tasks: []todoTask{{ID: "same", Title: "Old", Status: "notStarted", Importance: "normal"}, {ID: "restore", Title: "Will return", Status: "notStarted", Importance: "normal"}}}
	rows := []map[string]any{
		{"id": "same", "status": "completed"},
		{"id": "same", "title": "Updated on phone"},
		{"id": "restore", "@removed": map[string]any{"reason": "changed"}},
		{"id": "restore", "title": "Restored", "status": "notStarted", "importance": "normal"},
	}
	result := todoApplyDelta(&cache, rows, false, nil)
	if result.Updated != 2 || result.Removed != 0 {
		t.Fatalf("delta result=%#v, want two updates and no removal", result)
	}
	byID := map[string]todoTask{}
	for _, task := range cache.Tasks {
		byID[task.ID] = task
	}
	if got := byID["same"]; got.Title != "Updated on phone" || got.Status != "completed" {
		t.Fatalf("sparse replay lost a changed property: %#v", got)
	}
	if got := byID["restore"]; got.Title != "Restored" || got.Status != "notStarted" {
		t.Fatalf("delete/recreate final state was not retained: %#v", got)
	}
}

func TestTodoDeltaResetUsesLocationAndStoresNewCursor(t *testing.T) {
	a := newTodoTestApp(t)
	enableTodoMicrosoftTestList(t, a, "remote")
	store := a.readTodoTokenStore()
	store.AccessToken = "access"
	store.AccessExpiresAt = time.Now().Add(time.Hour).UnixMilli()
	if err := a.writeTodoTokenStore(store); err != nil {
		t.Fatal(err)
	}
	requests := []string{}
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type=%q, want application/json", got)
		}
		requests = append(requests, r.URL.Path)
		switch r.URL.Path {
		case "/expired":
			w.Header().Set("Location", server.URL+"/baseline")
			w.WriteHeader(http.StatusGone)
			_, _ = w.Write([]byte(`{"error":{"code":"syncStateNotFound"}}`))
		case "/baseline":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"value":            []map[string]any{{"id": "remote-task", "title": "From phone", "status": "notStarted", "importance": "normal"}},
				"@odata.deltaLink": server.URL + "/cursor?opaque=1",
			})
		default:
			t.Errorf("unexpected delta URL: %s", r.URL.String())
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	cache := a.readTodoListCache("remote")
	cache.DeltaLink = server.URL + "/expired"
	if err := a.writeTodoListCache(cache); err != nil {
		t.Fatal(err)
	}
	result, err := a.syncTodoListDeltaNow(context.Background(), "remote")
	if err != nil {
		t.Fatalf("delta reset: %v", err)
	}
	if result.DeltaMode != "reset-baseline" || result.Added != 1 {
		t.Fatalf("reset result=%#v", result)
	}
	cache = a.readTodoListCache("remote")
	if cache.DeltaLink != server.URL+"/cursor?opaque=1" || len(cache.Tasks) != 1 || cache.Tasks[0].Title != "From phone" {
		t.Fatalf("delta reset cache=%#v", cache)
	}
	if got := strings.Join(requests, ","); got != "/expired,/baseline" {
		t.Fatalf("reset request order=%q", got)
	}
}

func TestTodoSyncContinuesWhenOneMappedListFails(t *testing.T) {
	a := newTodoTestApp(t)
	enableTodoMicrosoftTestLists(t, a, "bad", "good")
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type=%q, want application/json", got)
		}
		switch r.URL.Path {
		case "/bad":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"code":"invalidRequest"}}`))
		case "/good":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"value":            []map[string]any{{"id": "phone-task", "title": "Arrived", "status": "notStarted", "importance": "normal"}},
				"@odata.deltaLink": server.URL + "/good-cursor",
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	for id, endpoint := range map[string]string{"bad": server.URL + "/bad", "good": server.URL + "/good"} {
		cache := a.readTodoListCache(id)
		cache.DeltaLink = endpoint
		if err := a.writeTodoListCache(cache); err != nil {
			t.Fatal(err)
		}
	}
	result, err := a.syncTodoListIDsNow(context.Background(), []string{"bad", "good"})
	if err != nil {
		t.Fatalf("partial list failure should not fail successful pull: %v", err)
	}
	if !result.Partial || len(result.Lists) != 2 {
		t.Fatalf("per-list result=%#v", result)
	}
	good := a.readTodoListCache("good")
	if len(good.Tasks) != 1 || good.Tasks[0].Title != "Arrived" {
		t.Fatalf("good list did not complete after bad list failure: %#v", good)
	}
	bad := a.readTodoListCache("bad")
	if bad.LastError == "" || !strings.Contains(bad.LastError, "HTTP 400") {
		t.Fatalf("bad list did not retain safe Graph diagnostic: %#v", bad)
	}
}

func TestTodoBlockedPendingOperationDoesNotBlockInboundDelta(t *testing.T) {
	cache := todoListCache{
		Version:    1,
		ListID:     "remote",
		Tasks:      []todoTask{{ID: "task", Title: "Local", Status: "notStarted", Importance: "normal", SyncFailed: true}},
		PendingOps: []todoPendingOp{{Op: "patch", ListID: "remote", TaskID: "task", Blocked: true, Attempts: 3}},
	}
	result := todoApplyDelta(&cache, []map[string]any{{"id": "task", "title": "Phone wins after blocked write", "status": "completed"}}, false, nil)
	if result.Updated != 1 || cache.Tasks[0].Title != "Phone wins after blocked write" || cache.Tasks[0].Status != "completed" {
		t.Fatalf("blocked write still suppressed inbound update: result=%#v cache=%#v", result, cache)
	}
	result = todoApplyDelta(&cache, []map[string]any{{"id": "task", "@removed": map[string]any{"reason": "deleted"}}}, false, nil)
	if result.Removed != 1 || len(cache.Tasks) != 0 {
		t.Fatalf("blocked write still suppressed inbound deletion: result=%#v cache=%#v", result, cache)
	}
}

func TestTodoListsDeltaDiscoversAndPersistsOpaqueCursor(t *testing.T) {
	a := newTodoTestApp(t)
	store := a.readTodoTokenStore()
	store.ClientID = "client"
	store.RefreshToken = "refresh"
	store.AccessToken = "access"
	store.AccessExpiresAt = time.Now().Add(time.Hour).UnixMilli()
	if err := a.writeTodoTokenStore(store); err != nil {
		t.Fatal(err)
	}
	if _, err := a.writeTodoSettings(func(todo map[string]any) { todo["syncMode"] = todoSyncMicrosoft }); err != nil {
		t.Fatal(err)
	}
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type=%q, want application/json", got)
		}
		if r.URL.Path != "/lists" {
			t.Errorf("unexpected list delta path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"value": []map[string]any{{
				"id": "new-list", "displayName": "From Microsoft", "wellknownListName": "none",
			}},
			"@odata.deltaLink": server.URL + "/lists-cursor?opaque=1",
		})
	}))
	defer server.Close()
	idx := a.readTodoListsIndex()
	idx.DeltaLink = server.URL + "/lists"
	if err := a.writeTodoListsIndex(idx); err != nil {
		t.Fatal(err)
	}
	if err := a.syncTodoListsNow(context.Background()); err != nil {
		t.Fatalf("list delta: %v", err)
	}
	idx = a.readTodoListsIndex()
	if idx.DeltaLink != server.URL+"/lists-cursor?opaque=1" {
		t.Fatalf("list delta cursor=%q", idx.DeltaLink)
	}
	found := false
	for _, item := range idx.Lists {
		if item.ID == "new-list" {
			found = item.DisplayName == "From Microsoft" && item.Origin == todoListOriginMicrosoft && item.WellknownName == "none"
		}
	}
	if !found {
		t.Fatalf("list delta did not retain discovered Microsoft list: %#v", idx)
	}
}

func TestTodoRunInboundSyncReturnsSafeResultWhenEveryListFails(t *testing.T) {
	a := newTodoTestApp(t)
	enableTodoMicrosoftTestLists(t, a, "bad", "bad")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":"invalidRequest"}}`))
	}))
	defer server.Close()
	cache := a.readTodoListCache("bad")
	cache.DeltaLink = server.URL + "/bad"
	if err := a.writeTodoListCache(cache); err != nil {
		t.Fatal(err)
	}
	result, err := a.todoRunInboundSync(context.Background())
	if err != nil {
		t.Fatalf("manual Sync now should keep safe list result: %v", err)
	}
	if result.OK || !result.Partial || len(result.Lists) != 1 || result.Lists[0].HTTPStatus != http.StatusBadRequest {
		t.Fatalf("all-failed result lost structured diagnostics: %#v", result)
	}
	status := a.todoInboundSyncStatus()
	if status.LastError == "" || status.BackoffSeconds < 1 {
		t.Fatalf("all-failed sync did not retain bounded inbound failure state: %#v", status)
	}
}
