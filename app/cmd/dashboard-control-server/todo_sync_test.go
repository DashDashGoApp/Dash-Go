package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func newTodoTestApp(t *testing.T) *app {
	t.Helper()
	dir := t.TempDir()
	a := &app{
		dash:          dir,
		home:          dir,
		configDir:     filepath.Join(dir, "config"),
		cacheDir:      filepath.Join(dir, "cache"),
		logDir:        filepath.Join(dir, "logs"),
		todoDir:       filepath.Join(dir, "config", "todo"),
		todoTokenFile: filepath.Join(dir, ".dashboard-todo.json"),
		settingsFile:  filepath.Join(dir, "config", "settings.json"),
		todoStreams:   map[chan []byte]bool{},
	}
	a.ensureDirs()
	return a
}

func TestTodoOpaqueListIDsUseHashedCacheFiles(t *testing.T) {
	a := newTodoTestApp(t)
	path := a.todoListPath("graph/list/id/with/slashes")
	if filepath.Dir(path) != a.todoDir {
		t.Fatalf("todo cache escaped directory: %s", path)
	}
	if filepath.Base(path) == "graph/list/id/with/slashes.json" {
		t.Fatalf("opaque graph id was used directly as filename")
	}
}

func TestTodoLocalDefaultsRemainAvailableWithoutSettingsOrCloudQueue(t *testing.T) {
	a := newTodoTestApp(t)
	status := a.todoStatusPayload()
	if got := status["syncMode"]; got != todoSyncLocal {
		t.Fatalf("default sync mode = %#v, want local", got)
	}
	mapping, _ := status["map"].(map[string]string)
	if mapping["todo"] != todoLocalTodoListID || mapping["grocery"] != todoLocalGroceryListID {
		t.Fatalf("local default mapping = %#v", mapping)
	}
	if _, err := os.Stat(a.settingsFile); !os.IsNotExist(err) {
		t.Fatalf("status read should not seed settings: %v", err)
	}

	cache, err := a.upsertTodoTask(todoLocalTodoListID, map[string]any{"title": "Milk"})
	if err != nil {
		t.Fatal(err)
	}
	if len(cache.Tasks) != 1 || cache.Tasks[0].Title != "Milk" {
		t.Fatalf("unexpected local task cache: %#v", cache.Tasks)
	}
	if cache.Tasks[0].Pending != "" || len(cache.PendingOps) != 0 {
		t.Fatalf("local-only create must not show cloud pending state: %#v", cache)
	}
	cache, err = a.patchTodoTask(todoLocalTodoListID, cache.Tasks[0].ID, map[string]any{"status": "completed"})
	if err != nil {
		t.Fatal(err)
	}
	if cache.Tasks[0].Status != "completed" || cache.Tasks[0].Pending != "" || len(cache.PendingOps) != 0 {
		t.Fatalf("local-only patch must stay local: %#v", cache)
	}
}

func TestTodoPendingQueueRequiresLinkedRegisteredMicrosoftList(t *testing.T) {
	a := newTodoTestApp(t)
	cache := todoListCache{Version: 1, ListID: "remote-list", PendingOps: []todoPendingOp{}}
	a.enqueueTodoOp(&cache, todoPendingOp{Op: "create", ListID: "remote-list", TaskID: "local-1"})
	if len(cache.PendingOps) != 0 {
		t.Fatalf("local default unexpectedly queued cloud work: %#v", cache.PendingOps)
	}
	if _, err := a.writeTodoSettings(func(todo map[string]any) { todo["syncMode"] = todoSyncMicrosoft }); err != nil {
		t.Fatal(err)
	}
	if err := a.writeTodoTokenStore(todoTokenStore{ClientID: "client", RefreshToken: "refresh"}); err != nil {
		t.Fatal(err)
	}

	// beta.13 deliberately does not infer cloud eligibility from an opaque ID.
	// A linked account alone must not queue a write until the active list index
	// explicitly records this list as Microsoft-origin.
	a.enqueueTodoOp(&cache, todoPendingOp{Op: "create", ListID: "remote-list", TaskID: "local-1"})
	if len(cache.PendingOps) != 0 {
		t.Fatalf("unregistered list unexpectedly queued cloud work: %#v", cache.PendingOps)
	}
	if err := a.todoUpsertListInfo(todoListInfo{ID: "remote-list", DisplayName: "Remote list", Origin: todoListOriginMicrosoft}); err != nil {
		t.Fatal(err)
	}
	a.enqueueTodoOp(&cache, todoPendingOp{Op: "create", ListID: "remote-list", TaskID: "local-1"})
	if len(cache.PendingOps) != 1 || cache.PendingOps[0].Op != "create" {
		t.Fatalf("linked registered Microsoft list must queue mirror work: %#v", cache.PendingOps)
	}
}

func TestTodoTokenStoreOwnerOnly(t *testing.T) {
	a := newTodoTestApp(t)
	if err := a.writeTodoTokenStore(todoTokenStore{ClientID: "client", RefreshToken: "refresh"}); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(a.todoTokenFile)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0600 {
		t.Fatalf("token store mode = %o, want 0600", st.Mode().Perm())
	}
}

func TestTodoLegacyVisibilityDoesNotHidePermanentMappings(t *testing.T) {
	a := newTodoTestApp(t)
	if _, err := a.writeTodoSettings(func(todo map[string]any) {
		todo["enabled"] = false // beta.36 legacy state must be ignored by beta.37.
		todo["map"] = map[string]any{"todo": "", "grocery": ""}
	}); err != nil {
		t.Fatal(err)
	}
	status := a.todoStatusPayload()
	mapping, _ := status["map"].(map[string]string)
	if mapping["todo"] != todoLocalTodoListID || mapping["grocery"] != todoLocalGroceryListID {
		t.Fatalf("legacy visibility/empty mappings hid permanent slots: %#v", mapping)
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/todo/apps", nil)
	if !a.handleTodoPost(w, r, "/api/todo/apps", map[string]any{"enabled": false}) {
		t.Fatal("retired Todo endpoint was not rejected by the Todo router")
	}
	if w.Code != http.StatusNotFound {
		t.Fatalf("retired apps visibility route returned %d, want 404: %s", w.Code, w.Body.String())
	}
	status = a.todoStatusPayload()
	mapping, _ = status["map"].(map[string]string)
	if mapping["todo"] != todoLocalTodoListID || mapping["grocery"] != todoLocalGroceryListID {
		t.Fatalf("retired route changed permanent mappings: %#v", mapping)
	}
}

func TestTodoDashboardDockDefaultsOffAndPersistsIndependentlyOfPermanentApps(t *testing.T) {
	a := newTodoTestApp(t)
	if a.todoDashboardDockEnabled() {
		t.Fatal("dashboard Lists dock must default off")
	}
	if _, err := a.writeTodoSettings(func(todo map[string]any) {
		todo["dashboardDock"] = true
	}); err != nil {
		t.Fatal(err)
	}
	if !a.todoDashboardDockEnabled() {
		t.Fatal("persisted dashboard Lists dock was not enabled")
	}
	status := a.todoStatusPayload()
	if dock, _ := status["dashboardDock"].(bool); !dock {
		t.Fatalf("status omitted persisted dashboard dock state: %#v", status)
	}
	if _, exists := status["enabled"]; exists {
		t.Fatalf("dashboard dock status must not expose retired app visibility: %#v", status)
	}
	dockMap, _ := status["dockMap"].(map[string]string)
	if dockMap["todo"] != todoLocalTodoListID || dockMap["grocery"] != todoLocalGroceryListID {
		t.Fatalf("dashboard dock must retain local defaults independently of permanent Apps tiles: %#v", dockMap)
	}
}

func enableTodoMicrosoftTestList(t *testing.T, a *app, listID string) {
	t.Helper()
	if _, err := a.writeTodoSettings(func(todo map[string]any) {
		todo["syncMode"] = todoSyncMicrosoft
		todo["map"] = map[string]any{"todo": listID, "grocery": listID}
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.writeTodoTokenStore(todoTokenStore{ClientID: "client", RefreshToken: "refresh"}); err != nil {
		t.Fatal(err)
	}
	if err := a.todoUpsertListInfo(todoListInfo{ID: listID, DisplayName: "Grocery", Origin: todoListOriginMicrosoft}); err != nil {
		t.Fatal(err)
	}
}

func TestTodoInboundSyncUsesFixedTwentyFiveSecondCadenceAndMigratesLegacyPreference(t *testing.T) {
	a := newTodoTestApp(t)
	if todoInboundSyncFixedSeconds != 25 {
		t.Fatalf("fixed inbound cadence = %d, want 25 seconds", todoInboundSyncFixedSeconds)
	}
	if got := a.todoInboundSyncSeconds(); got != todoInboundSyncFixedSeconds {
		t.Fatalf("default inbound cadence = %d, want fixed %d", got, todoInboundSyncFixedSeconds)
	}
	for _, legacySeconds := range []int{0, 15, 30, 300, 7} {
		if _, err := a.writeTodoSettings(func(todo map[string]any) { todo["inboundSyncSeconds"] = legacySeconds }); err != nil {
			t.Fatal(err)
		}
		if got := a.todoInboundSyncSeconds(); got != todoInboundSyncFixedSeconds {
			t.Fatalf("legacy cadence %d changed runtime safety net: got %d", legacySeconds, got)
		}
		if err := a.todoNormalizeInboundSyncSetting(); err != nil {
			t.Fatal(err)
		}
		if raw := jsonutil.Int(a.todoSettings()["inboundSyncSeconds"], -1); raw != todoInboundSyncFixedSeconds {
			t.Fatalf("legacy cadence %d was not persisted as fixed 25-second value: %d", legacySeconds, raw)
		}
	}
}

func TestTodoInboundSyncNormalizationDoesNotSeedFreshSettings(t *testing.T) {
	a := newTodoTestApp(t)
	if err := a.todoNormalizeInboundSyncSetting(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(a.settingsFile); !os.IsNotExist(err) {
		t.Fatalf("fixed cadence migration seeded fresh settings: %v", err)
	}
}

func TestTodoMappedMicrosoftListsDeduplicateAndStatus(t *testing.T) {
	a := newTodoTestApp(t)
	enableTodoMicrosoftTestList(t, a, "remote-grocery")
	ids := a.todoMappedMicrosoftListIDs()
	if len(ids) != 1 || ids[0] != "remote-grocery" {
		t.Fatalf("mapped Microsoft IDs = %#v, want one deduplicated list", ids)
	}
	status := a.todoInboundSyncStatus()
	if !status.Enabled || status.ConfiguredSeconds != todoInboundSyncFixedSeconds || status.Mode != "automatic" {
		t.Fatalf("unexpected inbound status: %#v", status)
	}
}

func TestTodoInboundCandidatesDoNotPollUnmappedMicrosoftLists(t *testing.T) {
	a := newTodoTestApp(t)
	if _, err := a.writeTodoSettings(func(todo map[string]any) {
		todo["syncMode"] = todoSyncMicrosoft
		todo["map"] = map[string]any{"todo": todoLocalTodoListID, "grocery": todoLocalGroceryListID}
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.writeTodoTokenStore(todoTokenStore{ClientID: "client", RefreshToken: "refresh"}); err != nil {
		t.Fatal(err)
	}
	if err := a.todoUpsertListInfo(todoListInfo{ID: "remote-grocery", DisplayName: "Grocery", Origin: todoListOriginMicrosoft}); err != nil {
		t.Fatal(err)
	}
	if ids := a.todoInboundMicrosoftListIDs(); len(ids) != 0 {
		t.Fatalf("unmapped discovered list must not become a scheduled candidate: %#v", ids)
	}
	status := a.todoInboundSyncStatus()
	if status.Enabled || !strings.Contains(status.LastError, "not mapped") {
		t.Fatalf("linked but unmapped status must explain mapping: %#v", status)
	}
}

func TestTodoInboundCandidatesKeepEstablishedUnmappedMicrosoftListTracked(t *testing.T) {
	a := newTodoTestApp(t)
	if _, err := a.writeTodoSettings(func(todo map[string]any) {
		todo["syncMode"] = todoSyncMicrosoft
		todo["map"] = map[string]any{"todo": todoLocalTodoListID, "grocery": todoLocalGroceryListID}
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.writeTodoTokenStore(todoTokenStore{ClientID: "client", RefreshToken: "refresh"}); err != nil {
		t.Fatal(err)
	}
	for _, item := range []todoListInfo{
		{ID: "tracked", DisplayName: "Tracked elsewhere", Origin: todoListOriginMicrosoft},
		{ID: "untouched", DisplayName: "Untouched", Origin: todoListOriginMicrosoft},
	} {
		if err := a.todoUpsertListInfo(item); err != nil {
			t.Fatal(err)
		}
	}
	tracked := a.readTodoListCache("tracked")
	tracked.DeltaLink = "https://graph.microsoft.com/v1.0/me/todo/lists/tracked/tasks/delta?$deltatoken=opaque"
	if err := a.writeTodoListCache(tracked); err != nil {
		t.Fatal(err)
	}
	ids := a.todoInboundMicrosoftListIDs()
	if len(ids) != 1 || ids[0] != "tracked" {
		t.Fatalf("only established non-slot list should remain tracked: %#v", ids)
	}
	if !a.todoInboundSyncReady() {
		t.Fatal("saved cursor must make a linked list eligible for bounded automatic pull")
	}
}

func TestTodoInboundLinkedWithoutKnownListExplainsWhatToDo(t *testing.T) {
	a := newTodoTestApp(t)
	if _, err := a.writeTodoSettings(func(todo map[string]any) { todo["syncMode"] = todoSyncMicrosoft }); err != nil {
		t.Fatal(err)
	}
	if err := a.writeTodoTokenStore(todoTokenStore{ClientID: "client", RefreshToken: "refresh"}); err != nil {
		t.Fatal(err)
	}
	status := a.todoInboundSyncStatus()
	if status.Enabled || !strings.Contains(status.LastError, "not mapped") {
		t.Fatalf("linked-but-empty inbound status must be actionable: %#v", status)
	}
}

func TestTodoApplyDeltaMergesRemoteChangesAndPreservesPendingLocalWrite(t *testing.T) {
	cache := todoListCache{
		Version:     1,
		ListID:      "remote",
		DisplayName: "Grocery",
		Tasks: []todoTask{
			{ID: "keep", Title: "Old title", Status: "notStarted", Importance: "normal", LastModifiedDateTime: "2026-01-01T00:00:00Z"},
			{ID: "remove", Title: "Delete me", Status: "notStarted", Importance: "normal"},
			{ID: "pending", Title: "Local wins", Status: "completed", Importance: "normal", Pending: "update"},
		},
		PendingOps: []todoPendingOp{{Op: "patch", ListID: "remote", TaskID: "pending"}},
	}
	rows := []map[string]any{
		{"id": "keep", "title": "New title", "status": "notStarted", "importance": "normal", "lastModifiedDateTime": "2026-01-02T00:00:00Z"},
		{"id": "new", "title": "Added remotely", "status": "notStarted", "importance": "normal", "lastModifiedDateTime": "2026-01-02T00:00:00Z"},
		{"id": "remove", "@removed": map[string]any{"reason": "deleted"}},
		{"id": "pending", "title": "Remote stale title", "status": "notStarted", "importance": "normal", "lastModifiedDateTime": "2026-01-02T00:00:00Z"},
	}
	result := todoApplyDelta(&cache, rows, false, nil)
	if result.Added != 1 || result.Updated != 1 || result.Removed != 1 {
		t.Fatalf("delta result = %#v, want one add/update/remove", result)
	}
	byID := map[string]todoTask{}
	for _, task := range cache.Tasks {
		byID[task.ID] = task
	}
	if byID["keep"].Title != "New title" || byID["new"].Title != "Added remotely" {
		t.Fatalf("delta did not merge remote changes: %#v", byID)
	}
	if _, found := byID["remove"]; found {
		t.Fatalf("remote deletion remained in cache: %#v", byID)
	}
	if pending := byID["pending"]; pending.Title != "Local wins" || pending.Pending != "update" {
		t.Fatalf("pending local write was overwritten by inbound delta: %#v", pending)
	}
}

func TestTodoApplyDeltaClearsStalePendingFlagBeforeRemoteMerge(t *testing.T) {
	cache := todoListCache{Version: 1, ListID: "remote", Tasks: []todoTask{{ID: "stale", Title: "Old", Status: "notStarted", Importance: "normal", Pending: "update"}}}
	result := todoApplyDelta(&cache, []map[string]any{{"id": "stale", "title": "Changed on phone", "status": "completed", "importance": "normal", "lastModifiedDateTime": "2026-01-02T00:00:00Z"}}, false, nil)
	if result.Updated != 1 || len(cache.Tasks) != 1 {
		t.Fatalf("stale pending task was not reconciled: result=%#v cache=%#v", result, cache.Tasks)
	}
	if got := cache.Tasks[0]; got.Pending != "" || got.Title != "Changed on phone" || got.Status != "completed" {
		t.Fatalf("stale pending flag blocked remote change: %#v", got)
	}
}

func TestTodoApplyDeltaBaselineRemovesMissingRemoteTasksButKeepsPending(t *testing.T) {
	cache := todoListCache{
		Version: 1,
		ListID:  "remote",
		Tasks: []todoTask{
			{ID: "present", Title: "Present", Status: "notStarted", Importance: "normal"},
			{ID: "missing", Title: "Missing", Status: "notStarted", Importance: "normal"},
			{ID: "pending", Title: "Still local", Status: "notStarted", Importance: "normal", Pending: "create"},
		},
		PendingOps: []todoPendingOp{{Op: "create", ListID: "remote", TaskID: "pending"}},
	}
	result := todoApplyDelta(&cache, []map[string]any{{"id": "present", "title": "Present", "status": "notStarted", "importance": "normal"}}, true, nil)
	if result.Removed != 1 {
		t.Fatalf("baseline removed %d tasks, want one", result.Removed)
	}
	byID := map[string]bool{}
	for _, task := range cache.Tasks {
		byID[task.ID] = true
	}
	if !byID["present"] || !byID["pending"] || byID["missing"] {
		t.Fatalf("baseline cache = %#v, expected present + pending only", byID)
	}
}

func TestTodoApplyDeltaProtectsJustSettledLocalWriteForCurrentRound(t *testing.T) {
	cache := todoListCache{Version: 1, ListID: "remote", Tasks: []todoTask{{ID: "newly-created", Title: "Milk", Status: "notStarted", Importance: "normal"}}}
	result := todoApplyDelta(&cache, []map[string]any{}, true, map[string]bool{"newly-created": true})
	if result.Removed != 0 || len(cache.Tasks) != 1 || cache.Tasks[0].ID != "newly-created" {
		t.Fatalf("fresh local task was erased before Graph echoed it: result=%#v cache=%#v", result, cache.Tasks)
	}
}
