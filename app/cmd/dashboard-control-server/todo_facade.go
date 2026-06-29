package main

import (
	"context"
	"net/http"
	"time"

	todopkg "github.com/DashDashGoApp/Dash-Go/app/internal/todo"
)

// Microsoft To Do route, CLI, and cross-domain adapters stay in package main.
// The internal/todo service owns all durable To Do/Grocery state, Graph work,
// queueing, recovery, and service-local synchronization.
type (
	todoTokenStore               = todopkg.TokenStore
	todoListInfo                 = todopkg.ListInfo
	todoListsIndex               = todopkg.ListsIndex
	todoTaskAssignment           = todopkg.TaskAssignment
	todoTask                     = todopkg.Task
	todoChecklistItem            = todopkg.ChecklistItem
	todoPendingOp                = todopkg.PendingOp
	todoListCache                = todopkg.ListCache
	todoInboundSyncStatus        = todopkg.InboundSyncStatus
	todoSyncListResult           = todopkg.SyncListResult
	todoSyncResult               = todopkg.SyncResult
	todoBlockedWriteStatus       = todopkg.BlockedWriteStatus
	todoDashboardDockItem        = todopkg.DashboardDockItem
	todoDashboardDockSlot        = todopkg.DashboardDockSlot
	todoDashboardDockSummary     = todopkg.DashboardDockSummary
	todoMigrationArchive         = todopkg.MigrationArchive
	todoMigrationArchiveIndex    = todopkg.MigrationArchiveIndex
	todoMigrationArchiveSnapshot = todopkg.MigrationArchiveSnapshot
	todoAuthPending              = todopkg.AuthPending
	todoGroceryMemoryItem        = todopkg.GroceryMemoryItem
	todoGraphResponseMeta        = todopkg.GraphResponseMeta
	todoGraphFailure             = todopkg.GraphFailure
	todoManualListSyncStatus     = todopkg.ManualListSyncStatus
	todoManualListSyncResult     = todopkg.ManualListSyncResult
	todoTaskPatchRequest         = todopkg.TaskPatchRequest
	todoThrottleError            = todopkg.ThrottleError
	todoInboundBackoffError      = todopkg.InboundBackoffError
	todoPartialSyncError         = todopkg.PartialSyncError
)

const (
	todoStatusFile                = todopkg.StatusFile
	todoScope                     = todopkg.Scope
	todoSyncLocal                 = todopkg.SyncLocal
	todoSyncMicrosoft             = todopkg.SyncMicrosoft
	todoLocalTodoListID           = todopkg.LocalTodoListID
	todoLocalGroceryListID        = todopkg.LocalGroceryListID
	todoListOriginLocal           = todopkg.ListOriginLocal
	todoListOriginMicrosoft       = todopkg.ListOriginMicrosoft
	todoDashboardDockPreviewLimit = todopkg.DashboardDockPreviewLimit
	todoDashboardDockPerSlotLimit = todopkg.DashboardDockPerSlotLimit
	todoInboundSyncFixedSeconds   = todopkg.InboundSyncFixedSeconds
	todoGraphDeltaMaxPages        = todopkg.GraphDeltaMaxPages
	todoGroceryMemoryLimit        = todopkg.GroceryMemoryLimit
	todoGroceryMemoryAliasLimit   = todopkg.GroceryMemoryAliasLimit
	todoGroceryMemoryTitleRunes   = todopkg.GroceryMemoryTitleRunes
	todoManualListSyncCooldown    = todopkg.ManualListSyncCooldown
	todoMigrationArchiveRetention = todopkg.MigrationArchiveRetention
	todoMigrationTaskLimit        = todopkg.MigrationTaskLimit
)

func (a *app) todoService() *todopkg.Service {
	a.todoInitMu.Lock()
	defer a.todoInitMu.Unlock()
	if a.todo == nil {
		a.todo = todopkg.New(todopkg.ServiceConfig{
			TodoDir:          a.todoDir,
			TokenFile:        a.todoTokenFile,
			LoadSettings:     a.loadSettings,
			UpdateSettings:   a.updateSettings,
			MutateSettings:   a.mutateSettings,
			PeoplePayload:    a.householdPeoplePayload,
			AssignmentLookup: a.householdPeopleAssignmentLookup,
			ActiveAssignment: a.householdPeopleActiveAssignment,
			PersonName:       householdPersonAssignmentName,
			Emit:             a.todoEmit,
			Now:              time.Now,
		})
	}
	return a.todo
}

// Pure contract adapters keep existing main-package route and integration
// tests stable while the implementations live with the service.
func todoNowMillis() int64                                     { return todopkg.NowMillis() }
func todoTaskFromBody(id string, body map[string]any) todoTask { return todopkg.TaskFromBody(id, body) }
func todoTaskPatchRequests(raw any) ([]todoTaskPatchRequest, error) {
	return todopkg.TaskPatchRequests(raw)
}
func todoGraphString(raw map[string]any, key string) (string, bool) {
	return todopkg.GraphString(raw, key)
}
func todoTaskPatchFromGraph(current todoTask, raw map[string]any) todoTask {
	return todopkg.TaskPatchFromGraph(current, raw)
}
func todoTaskGraphPatchBody(body map[string]any) map[string]any {
	return todopkg.TaskGraphPatchBody(body)
}
func todoListInfoPatchFromGraph(current todoListInfo, raw map[string]any) (todoListInfo, bool) {
	return todopkg.ListInfoPatchFromGraph(current, raw)
}
func todoCloneGraphRow(raw map[string]any) map[string]any { return todopkg.CloneGraphRow(raw) }
func todoInitialListsDeltaEndpoint() string               { return todopkg.InitialListsDeltaEndpoint() }
func todoInitialTaskDeltaEndpoint(listID string) string {
	return todopkg.InitialTaskDeltaEndpoint(listID)
}
func todoListOriginOf(item todoListInfo) string       { return todopkg.ListOriginOf(item) }
func todoSyncResultText(result todoSyncResult) string { return todopkg.SyncResultText(result) }
func todoApplyDelta(cache *todoListCache, rows []map[string]any, full bool, protected map[string]bool) todoSyncListResult {
	return todopkg.ApplyDelta(cache, rows, full, protected)
}

func (a *app) readTodoTokenStore() todoTokenStore { return a.todoService().ReadTokenStore() }
func (a *app) writeTodoTokenStore(store todoTokenStore) error {
	return a.todoService().WriteTokenStore(store)
}
func (a *app) unlinkTodo() error { return a.todoService().Unlink() }
func (a *app) cancelTodoAuth()   { a.todoService().CancelAuth() }
func (a *app) startTodoAuth(clientID string) (map[string]any, error) {
	return a.todoService().StartAuth(clientID)
}
func (a *app) todoGraphRequest(ctx context.Context, method, endpoint string, body any, token string) (*http.Request, error) {
	return a.todoService().GraphRequest(ctx, method, endpoint, body, token)
}

func (a *app) todoCreateCloudList(ctx context.Context, name string) (todoListInfo, error) {
	return a.todoService().CreateCloudList(ctx, name)
}
func (a *app) syncTodoListsNow(ctx context.Context) error { return a.todoService().SyncListsNow(ctx) }
func (a *app) todoResolveBlockedPendingOps(listID, action string) (todoListCache, int, error) {
	return a.todoService().ResolveBlockedPendingOps(listID, action)
}
func (a *app) todoGroceryMemory() []todoGroceryMemoryItem { return a.todoService().GroceryMemory() }
func (a *app) addTodoGroceryMemoryItem(title string) ([]todoGroceryMemoryItem, error) {
	return a.todoService().AddGroceryMemoryItem(title)
}
func (a *app) editTodoGroceryMemoryItem(key, title string) ([]todoGroceryMemoryItem, error) {
	return a.todoService().EditGroceryMemoryItem(key, title)
}
func (a *app) setTodoGroceryMemoryPinned(key string, pinned bool) ([]todoGroceryMemoryItem, error) {
	return a.todoService().SetGroceryMemoryPinned(key, pinned)
}
func (a *app) hideTodoGroceryMemoryItem(key string) ([]todoGroceryMemoryItem, error) {
	return a.todoService().HideGroceryMemoryItem(key)
}
func (a *app) restoreTodoGroceryMemoryItem(key string) ([]todoGroceryMemoryItem, error) {
	return a.todoService().RestoreGroceryMemoryItem(key)
}
func (a *app) deleteTodoGroceryMemoryItem(key string) ([]todoGroceryMemoryItem, error) {
	return a.todoService().DeleteGroceryMemoryItem(key)
}
func (a *app) todoRememberGroceryTasks(tasks []todoTask) { a.todoService().RememberGroceryTasks(tasks) }
func (a *app) todoInboundSyncSeconds() int               { return a.todoService().InboundSyncSeconds() }
func (a *app) todoNormalizeInboundSyncSetting() error {
	return a.todoService().NormalizeInboundSyncSetting()
}
func (a *app) todoInboundSyncStatus() todoInboundSyncStatus {
	return a.todoService().InboundSyncStatus()
}
func (a *app) todoNotifyInboundScheduler()          { a.todoService().NotifyInboundScheduler() }
func (a *app) startTodoInboundScheduler()           { a.todoService().StartInboundScheduler() }
func (a *app) todoMappedMicrosoftListIDs() []string { return a.todoService().MappedMicrosoftListIDs() }
func (a *app) todoInboundMicrosoftListIDs() []string {
	return a.todoService().InboundMicrosoftListIDs()
}
func (a *app) todoInboundSyncReady() bool { return a.todoService().InboundSyncReady() }

func (a *app) todoBeginInboundRun(listIDs []string, reason string) ([]string, bool, error) {
	return a.todoService().BeginInboundRun(listIDs, reason)
}

func (a *app) todoStartInboundSyncForList(listID string) {
	a.todoService().StartInboundSyncForList(listID)
}

func (a *app) todoRunInboundSync(ctx context.Context) (todoSyncResult, error) {
	return a.todoService().RunInboundSync(ctx)
}

func (a *app) syncTodoListDeltaNow(ctx context.Context, listID string) (todoSyncListResult, error) {
	return a.todoService().SyncListDeltaNow(ctx, listID)
}
func (a *app) syncTodoListIDsNow(ctx context.Context, ids []string) (todoSyncResult, error) {
	return a.todoService().SyncListIDsNow(ctx, ids)
}
func (a *app) todoListLock(listID string) func() { return a.todoService().ListLock(listID) }
func (a *app) todoManualListSyncStatus(listID string) todoManualListSyncStatus {
	return a.todoService().ManualListSyncStatus(listID)
}

func (a *app) todoRequestManualListSync(listID string) todoManualListSyncResult {
	return a.todoService().RequestManualListSync(listID)
}

func (a *app) todoArchiveIndexPath() string { return a.todoService().ArchiveIndexPath() }
func (a *app) todoArchiveSnapshotPath(id string) string {
	return a.todoService().ArchiveSnapshotPath(id)
}
func (a *app) readTodoArchiveIndex() todoMigrationArchiveIndex {
	return a.todoService().ReadArchiveIndex()
}

func (a *app) todoStageArchive(source todoListInfo, cache todoListCache, reason string) (todoMigrationArchive, error) {
	return a.todoService().StageArchive(source, cache, reason)
}

func (a *app) todoCommitArchive(archive todoMigrationArchive) error {
	return a.todoService().CommitArchive(archive)
}
func (a *app) todoPurgeExpiredArchives() { a.todoService().PurgeExpiredArchives() }
func (a *app) startTodoArchiveJanitor()  { a.todoService().StartArchiveJanitor() }
func (a *app) migrateTodoSlot(ctx context.Context, action, slot string) (map[string]any, error) {
	return a.todoService().MigrateSlot(ctx, action, slot)
}
func (a *app) todoTaskAssignmentName(task todoTask) string {
	return a.todoService().TaskAssignmentName(task)
}

func (a *app) todoAssignTaskPerson(listID, taskID, personID string) (todoListCache, error) {
	return a.todoService().AssignTaskPerson(listID, taskID, personID)
}

func (a *app) todoListInfoByID(id string) (todoListInfo, bool) {
	return a.todoService().ListInfoByID(id)
}
func (a *app) todoListCloudSyncEnabled(id string) bool {
	return a.todoService().ListCloudSyncEnabled(id)
}
func (a *app) todoHasMappedMicrosoftList() bool { return a.todoService().HasMappedMicrosoftList() }
func (a *app) todoSettings() map[string]any     { return a.todoService().Settings() }
func (a *app) writeTodoSettings(mut func(map[string]any)) (map[string]any, error) {
	return a.todoService().WriteSettings(mut)
}
func (a *app) todoDashboardDockEnabled() bool          { return a.todoService().DashboardDockEnabled() }
func (a *app) todoDashboardDockSlots() map[string]bool { return a.todoService().DashboardDockSlots() }

func (a *app) todoDashboardDockSummary() todoDashboardDockSummary {
	return a.todoService().DashboardDockSummary()
}

func (a *app) todoCloudSyncEnabled() bool { return a.todoService().CloudSyncEnabled() }

func (a *app) todoMap() map[string]string { return a.todoService().Map() }

func (a *app) todoListPath(id string) string { return a.todoService().ListPath(id) }

func (a *app) readTodoListsIndex() todoListsIndex { return a.todoService().ReadListsIndex() }
func (a *app) writeTodoListsIndex(index todoListsIndex) error {
	return a.todoService().WriteListsIndex(index)
}
func (a *app) todoUpsertListInfo(info todoListInfo) error {
	return a.todoService().UpsertListInfo(info)
}

func (a *app) readTodoListCache(id string) todoListCache { return a.todoService().ReadListCache(id) }
func (a *app) writeTodoListCache(cache todoListCache) error {
	return a.todoService().WriteListCache(cache)
}

func (a *app) todoStatusPayload() map[string]any { return a.todoService().StatusPayload() }
func (a *app) enqueueTodoOp(cache *todoListCache, op todoPendingOp) {
	a.todoService().EnqueueOp(cache, op)
}
func (a *app) upsertTodoTask(listID string, body map[string]any) (todoListCache, error) {
	return a.todoService().UpsertTask(listID, body)
}
func (a *app) patchTodoTask(listID, taskID string, body map[string]any) (todoListCache, error) {
	return a.todoService().PatchTask(listID, taskID, body)
}
func (a *app) deleteTodoTask(listID, taskID string) (todoListCache, error) {
	return a.todoService().DeleteTask(listID, taskID)
}
func (a *app) clearTodoCompletedSnapshot(listID string, ids []string) (todoListCache, int, error) {
	return a.todoService().ClearCompletedSnapshot(listID, ids)
}
func (a *app) patchTodoTasksBatch(listID string, req []todoTaskPatchRequest) (todoListCache, int, error) {
	return a.todoService().PatchTasksBatch(listID, req)
}

// Focused main-package test adapters keep runtime locks service-owned.
func (a *app) todoSetInboundRunningForTest(value bool) {
	a.todoService().SetInboundRunningForTest(value)
}
func (a *app) todoQueueInboundForTest(ids []string, reason string, now time.Time) {
	a.todoService().QueueInboundForTest(ids, reason, now)
}
func (a *app) todoFinishInboundRunForTest(err error, now time.Time) []string {
	return a.todoService().FinishInboundRunForTest(err, now)
}
func (a *app) todoInboundRuntimeForTest() (bool, bool, map[string]bool) {
	return a.todoService().InboundRuntimeForTest()
}
func (a *app) todoSetManualSyncUntilForTest(values map[string]time.Time) {
	a.todoService().SetManualSyncUntilForTest(values)
}
