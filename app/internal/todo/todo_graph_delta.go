package todo

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"strings"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func todoGraphRemoved(raw map[string]any) bool {
	_, removed := raw["@removed"]
	return removed
}

func todoGraphItemID(raw map[string]any) string {
	return jsonutil.StringValue(raw["id"])
}

func todoPendingTaskIDs(cache todoListCache) map[string]bool {
	out := map[string]bool{}
	for _, operation := range cache.PendingOps {
		if operation.Blocked {
			continue
		}
		if id := strings.TrimSpace(operation.TaskID); id != "" {
			out[id] = true
		}
	}
	return out
}

// todoClearStaleTaskPending repairs an interrupted or legacy queue state. A
// Pending marker protects a task only while a durable, active operation still
// references it; blocked work is intentionally eligible for inbound recovery.
func todoClearStaleTaskPending(cache *todoListCache) int {
	if cache == nil {
		return 0
	}
	pending := todoPendingTaskIDs(*cache)
	cleared := 0
	for i := range cache.Tasks {
		id := strings.TrimSpace(cache.Tasks[i].ID)
		if cache.Tasks[i].Pending != "" && !pending[id] {
			cache.Tasks[i].Pending = ""
			cleared++
		}
	}
	return cleared
}

func todoTaskEqualForSync(left, right todoTask) bool {
	if left.ID != right.ID || left.Title != right.Title || left.Status != right.Status || left.Importance != right.Importance || left.LastModifiedDateTime != right.LastModifiedDateTime {
		return false
	}
	leftDue, _ := json.Marshal(left.DueDateTime)
	rightDue, _ := json.Marshal(right.DueDateTime)
	leftBody, _ := json.Marshal(left.Body)
	rightBody, _ := json.Marshal(right.Body)
	leftAssignment, _ := json.Marshal(todoAssignmentCopy(left.DashGoAssignment))
	rightAssignment, _ := json.Marshal(todoAssignmentCopy(right.DashGoAssignment))
	return string(leftDue) == string(rightDue) && string(leftBody) == string(rightBody) && string(leftAssignment) == string(rightAssignment)
}

func todoGraphString(raw map[string]any, key string) (string, bool) {
	value, exists := raw[key]
	if !exists {
		return "", false
	}
	return jsonutil.StringValue(value), true
}

// todoTaskPatchFromGraph applies only properties present in a delta row. Graph
// can send sparse updates, so replacing a cached task wholesale would erase a
// title, body, or due date that the response intentionally omitted.
func todoTaskPatchFromGraph(current todoTask, raw map[string]any) todoTask {
	next := current
	if id, ok := todoGraphString(raw, "id"); ok && id != "" {
		next.ID = id
	}
	if value, ok := todoGraphString(raw, "title"); ok {
		if value != "" {
			next.Title = value
		}
	}
	if value, ok := todoGraphString(raw, "status"); ok {
		if value != "" {
			next.Status = value
		}
	}
	if value, ok := todoGraphString(raw, "importance"); ok {
		if value != "" {
			next.Importance = value
		}
	}
	if value, exists := raw["dueDateTime"]; exists {
		if due, ok := value.(map[string]any); ok {
			next.DueDateTime = due
		} else {
			next.DueDateTime = nil
		}
	}
	if value, exists := raw["body"]; exists {
		if body, ok := value.(map[string]any); ok {
			next.Body = body
		} else {
			next.Body = nil
		}
	}
	if value, ok := todoGraphString(raw, "lastModifiedDateTime"); ok {
		next.LastModifiedDateTime = value
	}
	return next
}

func todoTaskFromGraph(raw map[string]any) todoTask {
	id, _ := todoGraphString(raw, "id")
	return todoTaskPatchFromGraph(todoTask{ID: id, Title: "Untitled task", Status: "notStarted", Importance: "normal"}, raw)
}

func todoCloneGraphRow(raw map[string]any) map[string]any {
	copy := make(map[string]any, len(raw))
	maps.Copy(copy, raw)
	return copy
}

// todoFinalDeltaRows coalesces repeated, sparse Graph delta rows in arrival
// order. A later sparse update must not erase a property carried by an earlier
// update for the same task. A delete followed by a later live row represents a
// restored/recreated task, so the later live state wins.
func todoFinalDeltaRows(rows []map[string]any, pending, protected map[string]bool) (map[string]map[string]any, map[string]bool) {
	final := map[string]map[string]any{}
	seen := map[string]bool{}
	for _, raw := range rows {
		id := todoGraphItemID(raw)
		if id == "" || pending[id] || protected[id] {
			continue
		}
		if todoGraphRemoved(raw) {
			final[id] = todoCloneGraphRow(raw)
			delete(seen, id)
			continue
		}
		merged := map[string]any{"id": id}
		if previous, exists := final[id]; exists && !todoGraphRemoved(previous) {
			merged = todoCloneGraphRow(previous)
		}
		for key, value := range raw {
			merged[key] = value
		}
		final[id] = merged
		seen[id] = true
	}
	return final, seen
}

func todoApplyDelta(cache *todoListCache, rows []map[string]any, fullBaseline bool, protected map[string]bool) todoSyncListResult {
	result := todoSyncListResult{ListID: cache.ListID, Title: cache.DisplayName}
	if result.Title == "" {
		result.Title = cache.ListID
	}
	if protected == nil {
		protected = map[string]bool{}
	}
	todoClearStaleTaskPending(cache)
	pending := todoPendingTaskIDs(*cache)
	byID := map[string]int{}
	for i := range cache.Tasks {
		byID[cache.Tasks[i].ID] = i
	}
	final, seen := todoFinalDeltaRows(rows, pending, protected)
	for id, raw := range final {
		if todoGraphRemoved(raw) {
			continue
		}
		if index, exists := byID[id]; exists {
			if cache.Tasks[index].Pending != "" || cache.Tasks[index].CloudIgnored {
				continue
			}
			next := todoTaskPatchFromGraph(cache.Tasks[index], raw)
			if !todoTaskEqualForSync(cache.Tasks[index], next) {
				cache.Tasks[index] = next
				result.Updated++
			}
			continue
		}
		cache.Tasks = append(cache.Tasks, todoTaskFromGraph(raw))
		byID[id] = len(cache.Tasks) - 1
		result.Added++
	}
	kept := make([]todoTask, 0, len(cache.Tasks))
	for _, task := range cache.Tasks {
		if pending[task.ID] || protected[task.ID] || task.Pending != "" || task.CloudIgnored {
			kept = append(kept, task)
			continue
		}
		raw, returned := final[task.ID]
		if (returned && todoGraphRemoved(raw)) || (fullBaseline && !seen[task.ID]) {
			result.Removed++
			continue
		}
		kept = append(kept, task)
	}
	cache.Tasks = kept
	return result
}

// todoInitialTaskDeltaEndpoint deliberately begins a new delta round on the
// bare documented route. $select is optional in Graph but certain Microsoft
// request brokers reject a $select-bearing To Do delta URI before it reaches the
// delta service. Once Graph responds, its opaque next/delta URLs are reused
// exactly as supplied and may contain Graph-owned query state.
func todoInitialTaskDeltaEndpoint(listID string) string {
	return "/me/todo/lists/" + todoGraphPathID(listID) + "/tasks/delta"
}

func (a *Service) todoDeltaFailure(listID, stage string, result todoSyncListResult, payload map[string]any, meta todoGraphResponseMeta, err error) (todoSyncListResult, error) {
	failure := todoGraphFailureFor(stage, listID, "Microsoft To Do task pull failed", payload, meta, err)
	todoApplyGraphFailure(&result, stage, failure)
	unlock := a.todoListLock(listID)
	cache := a.readTodoListCache(listID)
	cache.LastError = result.Error
	_ = a.writeTodoListCache(cache)
	unlock()
	a.todoEmit(map[string]any{"type": "sync.state", "listId": listID, "lastError": result.Error, "stage": stage})
	return result, failure
}

func (a *Service) syncTodoListDeltaNow(ctx context.Context, listID string) (todoSyncListResult, error) {
	return a.syncTodoListDeltaWithProtected(ctx, listID, nil)
}

// syncTodoListDeltaWithProtected gives a just-settled local write precedence for
// this reconciliation round. Graph can be briefly eventually consistent after a
// successful create/patch; treating the completed local action as authoritative
// prevents a fresh baseline from erasing it before Graph echoes it back.
func (a *Service) syncTodoListDeltaWithProtected(ctx context.Context, listID string, protected map[string]bool) (todoSyncListResult, error) {
	result := todoSyncListResult{ListID: listID}
	if !a.todoListCloudSyncEnabled(listID) {
		return result, nil
	}
	initial := a.readTodoListCache(listID)
	if initial.DisplayName == "" {
		if info, ok := a.todoListInfoByID(listID); ok {
			initial.DisplayName = info.DisplayName
		}
	}
	result.Title = initial.DisplayName
	if result.Title == "" {
		result.Title = listID
	}
	fullBaseline := strings.TrimSpace(initial.DeltaLink) == ""
	if fullBaseline {
		result.DeltaMode = "baseline"
	} else {
		result.DeltaMode = "incremental"
	}
	endpoint := initial.DeltaLink
	if endpoint == "" {
		endpoint = todoInitialTaskDeltaEndpoint(listID)
	}
	rows := make([]map[string]any, 0)
	for range todoGraphDeltaMaxPages {
		payload, meta, err := a.todoGraphResponse(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return a.todoDeltaFailure(listID, "task-delta", result, payload, meta, err)
		}
		if todoGraphNeedsDeltaReset(meta) && !fullBaseline {
			// Graph can invalidate a cursor with 410 or syncStateNotFound. Restart
			// only this list and prefer Graph's replacement baseline URL if present.
			fullBaseline = true
			result.DeltaMode = "reset-baseline"
			rows = rows[:0]
			endpoint = meta.Location
			if endpoint == "" {
				endpoint = todoInitialTaskDeltaEndpoint(listID)
			}
			continue
		}
		if todoGraphIsThrottle(meta) {
			failure := todoGraphFailureFor("task-delta", listID, "Microsoft To Do task pull was throttled", payload, meta, nil)
			item, _ := a.todoDeltaFailure(listID, "task-delta", result, payload, meta, failure)
			return item, &todoThrottleError{RetryAfter: meta.RetryAfter, Err: failure}
		}
		if meta.Status < 200 || meta.Status >= 300 {
			return a.todoDeltaFailure(listID, "task-delta", result, payload, meta, nil)
		}
		if values, ok := payload["value"].([]any); ok {
			for _, value := range values {
				if raw, ok := value.(map[string]any); ok {
					rows = append(rows, raw)
				}
			}
		}
		next := jsonutil.StringValue(payload["@odata.nextLink"])
		if next != "" {
			endpoint = next
			continue
		}
		deltaLink := jsonutil.StringValue(payload["@odata.deltaLink"])
		if deltaLink == "" {
			return a.todoDeltaFailure(listID, "task-delta", result, payload, meta, fmt.Errorf("delta response did not include a continuation token"))
		}
		// Re-read under the list lock so a local checkbox/save that landed while
		// Graph was in flight is never overwritten by an older cache snapshot.
		unlock := a.todoListLock(listID)
		cache := a.readTodoListCache(listID)
		if cache.DisplayName == "" {
			cache.DisplayName = result.Title
		}
		merged := todoApplyDelta(&cache, rows, fullBaseline, protected)
		merged.DeltaMode = result.DeltaMode
		cache.DeltaLink = deltaLink
		cache.LastSyncAt = todoNowMillis()
		cache.LastError = todoBlockedPendingSummary(cache)
		merged.LastSyncAt = cache.LastSyncAt
		merged.QueueBlocked = todoBlockedPendingCount(cache)
		writeErr := a.writeTodoListCache(cache)
		unlock()
		if writeErr != nil {
			return merged, writeErr
		}
		a.todoEmit(map[string]any{"type": "task.sync", "listId": listID, "added": merged.Added, "updated": merged.Updated, "removed": merged.Removed, "lastSyncAt": cache.LastSyncAt})
		return merged, nil
	}
	return a.todoDeltaFailure(listID, "task-delta", result, nil, todoGraphResponseMeta{}, fmt.Errorf("Microsoft To Do returned too many task delta pages"))
}

// syncTodoListNow is retained for callers that only need the durable cache
// result. All task pulls now use Graph delta state rather than full list reads.
func (a *Service) syncTodoListNow(ctx context.Context, listID string) error {
	a.todoCloudMu.Lock()
	defer a.todoCloudMu.Unlock()
	_, err := a.syncTodoListDeltaNow(ctx, listID)
	return err
}
