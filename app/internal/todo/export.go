package todo

import (
	"context"
	"net/http"
	"time"
)

// Exported aliases preserve the established route and JSON contract while the
// implementation-owned types and locks remain inside this package.
type (
	TokenStore               = todoTokenStore
	ListInfo                 = todoListInfo
	ListsIndex               = todoListsIndex
	TaskAssignment           = todoTaskAssignment
	Task                     = todoTask
	ChecklistItem            = todoChecklistItem
	PendingOp                = todoPendingOp
	ListCache                = todoListCache
	InboundSyncStatus        = todoInboundSyncStatus
	SyncListResult           = todoSyncListResult
	SyncResult               = todoSyncResult
	BlockedWriteStatus       = todoBlockedWriteStatus
	DashboardDockItem        = todoDashboardDockItem
	DashboardDockSlot        = todoDashboardDockSlot
	DashboardDockSummary     = todoDashboardDockSummary
	MigrationArchive         = todoMigrationArchive
	MigrationArchiveIndex    = todoMigrationArchiveIndex
	MigrationArchiveSnapshot = todoMigrationArchiveSnapshot
	AuthPending              = todoAuthPending
	GroceryMemoryItem        = todoGroceryMemoryItem
	GraphResponseMeta        = todoGraphResponseMeta
	GraphFailure             = todoGraphFailure
	ManualListSyncStatus     = todoManualListSyncStatus
	ManualListSyncResult     = todoManualListSyncResult
	TaskPatchRequest         = todoTaskPatchRequest
	ThrottleError            = todoThrottleError
	InboundBackoffError      = todoInboundBackoffError
	PartialSyncError         = todoPartialSyncError
)

const (
	StatusFile                = todoStatusFile
	Scope                     = todoScope
	SyncLocal                 = todoSyncLocal
	SyncMicrosoft             = todoSyncMicrosoft
	LocalTodoListID           = todoLocalTodoListID
	LocalGroceryListID        = todoLocalGroceryListID
	ListOriginLocal           = todoListOriginLocal
	ListOriginMicrosoft       = todoListOriginMicrosoft
	DashboardDockPreviewLimit = todoDashboardDockPreviewLimit
	DashboardDockPerSlotLimit = todoDashboardDockPerSlotLimit
	InboundSyncFixedSeconds   = todoInboundSyncFixedSeconds
	GraphDeltaMaxPages        = todoGraphDeltaMaxPages
	GroceryMemoryLimit        = todoGroceryMemoryLimit
	GroceryMemoryAliasLimit   = todoGroceryMemoryAliasLimit
	GroceryMemoryTitleRunes   = todoGroceryMemoryTitleRunes
	ManualListSyncCooldown    = todoManualListSyncCooldown
	MigrationArchiveRetention = todoMigrationArchiveRetention
	MigrationTaskLimit        = todoMigrationTaskLimit
)

func NowMillis() int64 { return todoNowMillis() }

func TaskFromBody(id string, body map[string]any) Task          { return todoTaskFromBody(id, body) }
func TaskPatchRequests(raw any) ([]TaskPatchRequest, error)     { return todoTaskPatchRequests(raw) }
func GraphString(raw map[string]any, key string) (string, bool) { return todoGraphString(raw, key) }
func TaskPatchFromGraph(current Task, raw map[string]any) Task {
	return todoTaskPatchFromGraph(current, raw)
}
func TaskGraphPatchBody(body map[string]any) map[string]any { return todoTaskGraphPatchBody(body) }
func ListInfoPatchFromGraph(current ListInfo, raw map[string]any) (ListInfo, bool) {
	return todoListInfoPatchFromGraph(current, raw)
}
func CloneGraphRow(raw map[string]any) map[string]any { return todoCloneGraphRow(raw) }
func InitialListsDeltaEndpoint() string               { return todoInitialListsDeltaEndpoint() }
func InitialTaskDeltaEndpoint(listID string) string   { return todoInitialTaskDeltaEndpoint(listID) }

func ListOriginOf(item ListInfo) string { return todoListOriginOf(item) }

func SyncResultText(result SyncResult) string { return todoSyncResultText(result) }
func ApplyDelta(cache *ListCache, rows []map[string]any, full bool, protected map[string]bool) SyncListResult {
	return todoApplyDelta(cache, rows, full, protected)
}

func (a *Service) ReadTokenStore() TokenStore             { return a.readTodoTokenStore() }
func (a *Service) WriteTokenStore(store TokenStore) error { return a.writeTodoTokenStore(store) }
func (a *Service) Unlink() error                          { return a.unlinkTodo() }
func (a *Service) CancelAuth() {
	a.todoMu.Lock()
	if a.todoAuthCancel != nil {
		a.todoAuthCancel()
		a.todoAuthCancel = nil
	}
	a.todoAuthState = todoAuthPending{}
	a.todoMu.Unlock()
}
func (a *Service) StartAuth(clientID string) (map[string]any, error) {
	return a.startTodoAuth(clientID)
}
func (a *Service) GraphRequest(ctx context.Context, method, endpoint string, body any, token string) (*http.Request, error) {
	return a.todoGraphRequest(ctx, method, endpoint, body, token)
}

func (a *Service) CreateCloudList(ctx context.Context, name string) (ListInfo, error) {
	return a.todoCreateCloudList(ctx, name)
}
func (a *Service) SyncListsNow(ctx context.Context) error { return a.syncTodoListsNow(ctx) }
func (a *Service) ResolveBlockedPendingOps(listID, action string) (ListCache, int, error) {
	return a.todoResolveBlockedPendingOps(listID, action)
}
func (a *Service) GroceryMemory() []GroceryMemoryItem { return a.todoGroceryMemory() }
func (a *Service) AddGroceryMemoryItem(title string) ([]GroceryMemoryItem, error) {
	return a.addTodoGroceryMemoryItem(title)
}
func (a *Service) EditGroceryMemoryItem(key, title string) ([]GroceryMemoryItem, error) {
	return a.editTodoGroceryMemoryItem(key, title)
}
func (a *Service) SetGroceryMemoryPinned(key string, pinned bool) ([]GroceryMemoryItem, error) {
	return a.setTodoGroceryMemoryPinned(key, pinned)
}
func (a *Service) HideGroceryMemoryItem(key string) ([]GroceryMemoryItem, error) {
	return a.hideTodoGroceryMemoryItem(key)
}
func (a *Service) RestoreGroceryMemoryItem(key string) ([]GroceryMemoryItem, error) {
	return a.restoreTodoGroceryMemoryItem(key)
}
func (a *Service) DeleteGroceryMemoryItem(key string) ([]GroceryMemoryItem, error) {
	return a.deleteTodoGroceryMemoryItem(key)
}
func (a *Service) RememberGroceryTasks(tasks []Task)    { a.todoRememberGroceryTasks(tasks) }
func (a *Service) InboundSyncSeconds() int              { return a.todoInboundSyncSeconds() }
func (a *Service) NormalizeInboundSyncSetting() error   { return a.todoNormalizeInboundSyncSetting() }
func (a *Service) InboundSyncStatus() InboundSyncStatus { return a.todoInboundSyncStatus() }
func (a *Service) NotifyInboundScheduler()              { a.todoNotifyInboundScheduler() }
func (a *Service) StartInboundScheduler()               { a.startTodoInboundScheduler() }
func (a *Service) MappedMicrosoftListIDs() []string     { return a.todoMappedMicrosoftListIDs() }
func (a *Service) InboundMicrosoftListIDs() []string    { return a.todoInboundMicrosoftListIDs() }
func (a *Service) InboundSyncReady() bool               { return a.todoInboundSyncReady() }

func (a *Service) BeginInboundRun(listIDs []string, reason string) ([]string, bool, error) {
	return a.todoBeginInboundRun(listIDs, reason)
}

func (a *Service) StartInboundSyncForList(listID string) { a.todoStartInboundSyncForList(listID) }

func (a *Service) RunInboundSync(ctx context.Context) (SyncResult, error) {
	return a.todoRunInboundSync(ctx)
}

func (a *Service) SyncListDeltaNow(ctx context.Context, listID string) (SyncListResult, error) {
	return a.syncTodoListDeltaNow(ctx, listID)
}
func (a *Service) SyncListIDsNow(ctx context.Context, listIDs []string) (SyncResult, error) {
	return a.syncTodoListIDsNow(ctx, listIDs)
}
func (a *Service) ListLock(listID string) func() { return a.todoListLock(listID) }
func (a *Service) ManualListSyncStatus(listID string) ManualListSyncStatus {
	return a.todoManualListSyncStatus(listID)
}

func (a *Service) RequestManualListSync(listID string) ManualListSyncResult {
	return a.todoRequestManualListSync(listID)
}

func (a *Service) ArchiveIndexPath() string                { return a.todoArchiveIndexPath() }
func (a *Service) ArchiveSnapshotPath(id string) string    { return a.todoArchiveSnapshotPath(id) }
func (a *Service) ReadArchiveIndex() MigrationArchiveIndex { return a.readTodoArchiveIndex() }

func (a *Service) StageArchive(source ListInfo, cache ListCache, reason string) (MigrationArchive, error) {
	return a.todoStageArchive(source, cache, reason)
}

func (a *Service) CommitArchive(archive MigrationArchive) error { return a.todoCommitArchive(archive) }
func (a *Service) PurgeExpiredArchives()                        { a.todoPurgeExpiredArchives() }
func (a *Service) StartArchiveJanitor()                         { a.startTodoArchiveJanitor() }
func (a *Service) MigrateSlot(ctx context.Context, action, slot string) (map[string]any, error) {
	return a.migrateTodoSlot(ctx, action, slot)
}
func (a *Service) TaskAssignmentName(task Task) string { return a.todoTaskAssignmentName(task) }

func (a *Service) AssignTaskPerson(listID, taskID, personID string) (ListCache, error) {
	return a.todoAssignTaskPerson(listID, taskID, personID)
}

func (a *Service) ListInfoByID(id string) (ListInfo, bool) { return a.todoListInfoByID(id) }
func (a *Service) ListCloudSyncEnabled(id string) bool     { return a.todoListCloudSyncEnabled(id) }
func (a *Service) HasMappedMicrosoftList() bool            { return a.todoHasMappedMicrosoftList() }
func (a *Service) Settings() map[string]any                { return a.todoSettings() }
func (a *Service) WriteSettings(mut func(map[string]any)) (map[string]any, error) {
	return a.writeTodoSettings(mut)
}
func (a *Service) DashboardDockEnabled() bool          { return a.todoDashboardDockEnabled() }
func (a *Service) DashboardDockSlots() map[string]bool { return a.todoDashboardDockSlots() }

func (a *Service) DashboardDockSummary() DashboardDockSummary { return a.todoDashboardDockSummary() }

func (a *Service) CloudSyncEnabled() bool { return a.todoCloudSyncEnabled() }

func (a *Service) Map() map[string]string { return a.todoMap() }

func (a *Service) ListPath(id string) string { return a.todoListPath(id) }

func (a *Service) ReadListsIndex() ListsIndex             { return a.readTodoListsIndex() }
func (a *Service) WriteListsIndex(index ListsIndex) error { return a.writeTodoListsIndex(index) }
func (a *Service) UpsertListInfo(info ListInfo) error     { return a.todoUpsertListInfo(info) }

func (a *Service) ReadListCache(id string) ListCache    { return a.readTodoListCache(id) }
func (a *Service) WriteListCache(cache ListCache) error { return a.writeTodoListCache(cache) }

func (a *Service) StatusPayload() map[string]any            { return a.todoStatusPayload() }
func (a *Service) EnqueueOp(cache *ListCache, op PendingOp) { a.enqueueTodoOp(cache, op) }
func (a *Service) UpsertTask(listID string, body map[string]any) (ListCache, error) {
	return a.upsertTodoTask(listID, body)
}
func (a *Service) PatchTask(listID, taskID string, body map[string]any) (ListCache, error) {
	return a.patchTodoTask(listID, taskID, body)
}
func (a *Service) DeleteTask(listID, taskID string) (ListCache, error) {
	return a.deleteTodoTask(listID, taskID)
}
func (a *Service) ClearCompletedSnapshot(listID string, ids []string) (ListCache, int, error) {
	return a.clearTodoCompletedSnapshot(listID, ids)
}
func (a *Service) PatchTasksBatch(listID string, requests []TaskPatchRequest) (ListCache, int, error) {
	return a.patchTodoTasksBatch(listID, requests)
}

// Focused test seams preserve service ownership of runtime locks.
func (a *Service) SetInboundRunningForTest(value bool) {
	a.todoInboundMu.Lock()
	a.todoInboundRunning = value
	a.todoInboundMu.Unlock()
}
func (a *Service) QueueInboundForTest(ids []string, reason string, now time.Time) {
	a.todoInboundMu.Lock()
	a.todoQueueInboundLocked(ids, reason, now)
	a.todoInboundMu.Unlock()
}
func (a *Service) FinishInboundRunForTest(err error, now time.Time) []string {
	return a.todoFinishInboundRun(err, now)
}
func (a *Service) InboundRuntimeForTest() (bool, bool, map[string]bool) {
	a.todoInboundMu.Lock()
	defer a.todoInboundMu.Unlock()
	lists := map[string]bool{}
	for id, on := range a.todoInboundQueuedLists {
		lists[id] = on
	}
	return a.todoInboundRunning, a.todoInboundQueued, lists
}
func (a *Service) SetManualSyncUntilForTest(values map[string]time.Time) {
	a.todoInboundMu.Lock()
	a.todoManualSyncUntil = values
	a.todoInboundMu.Unlock()
}
