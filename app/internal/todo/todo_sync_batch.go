package todo

import (
	"fmt"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// todoTaskPatchRequest is a bounded batch of local-first changes from the
// touch Lists app. It is intentionally a server-side cache transaction, not a
// Graph batch: Dash-Go writes once locally, then the normal cloud coordinator
// drains the resulting durable operations at its own pace.
type todoTaskPatchRequest struct {
	TaskID string
	Body   map[string]any
}

func todoTaskPatchRequests(raw any) ([]todoTaskPatchRequest, error) {
	values, ok := raw.([]any)
	if !ok || len(values) == 0 {
		return nil, fmt.Errorf("patches must contain one or more task changes")
	}
	if len(values) > 32 {
		return nil, fmt.Errorf("too many task changes in one batch")
	}
	out := make([]todoTaskPatchRequest, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		item, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("each task change must be an object")
		}
		id := jsonutil.BodyString(item, "id")
		patch, ok := item["patch"].(map[string]any)
		if id == "" || !ok || len(patch) == 0 {
			return nil, fmt.Errorf("each task change needs an id and patch")
		}
		if err := validateTodoString(id, "task id", maxTodoTaskIDRunes); err != nil {
			return nil, err
		}
		if err := validateTodoTaskBody(patch); err != nil {
			return nil, err
		}
		if seen[id] {
			return nil, fmt.Errorf("a task can appear only once in one batch")
		}
		seen[id] = true
		out = append(out, todoTaskPatchRequest{TaskID: id, Body: patch})
	}
	return out, nil
}

func (a *Service) applyTodoTaskPatch(cache *todoListCache, listID, taskID string, body map[string]any) bool {
	for i := range cache.Tasks {
		if cache.Tasks[i].ID != taskID {
			continue
		}
		if v := jsonutil.BodyString(body, "title"); v != "" {
			cache.Tasks[i].Title = v
		}
		if v := jsonutil.BodyString(body, "status"); v != "" {
			cache.Tasks[i].Status = v
		}
		if v := jsonutil.BodyString(body, "importance"); v != "" {
			cache.Tasks[i].Importance = v
		}
		cache.Tasks[i].ETag = todoNowMillis()
		cache.Tasks[i].LastModifiedDateTime = time.Now().Format(time.RFC3339)
		if a.todoListCloudSyncEnabled(listID) {
			cache.Tasks[i].Pending = "update"
		}
		a.enqueueTodoOp(cache, todoPendingOp{Op: "patch", ListID: listID, TaskID: taskID, Payload: body})
		return true
	}
	return false
}

func (a *Service) patchTodoTasksBatch(listID string, requests []todoTaskPatchRequest) (todoListCache, int, error) {
	unlock := a.todoListLock(listID)
	defer unlock()
	cache := a.readTodoListCache(listID)
	updated := 0
	for _, request := range requests {
		if a.applyTodoTaskPatch(&cache, listID, request.TaskID, request.Body) {
			updated++
		}
	}
	if updated == 0 {
		return cache, 0, fmt.Errorf("selected tasks are no longer available")
	}
	if err := a.writeTodoListCache(cache); err != nil {
		return cache, 0, err
	}
	if a.todoListCloudSyncEnabled(listID) {
		go a.todoStartDrain(listID)
	}
	return cache, updated, nil
}
