package todo

import (
	"fmt"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// todo_sync.go owns local-first task mutation and pending Graph-operation
// enqueueing. Grocery Memory persistence/mutation lives in todo_grocery_memory.go;
// settings/index persistence lives in todo_store.go.

func todoTaskFromBody(id string, body map[string]any) todoTask {
	title := jsonutil.BodyString(body, "title")
	if title == "" {
		title = "Untitled task"
	}
	status := jsonutil.BodyString(body, "status")
	if status == "" {
		status = "notStarted"
	}
	importance := jsonutil.BodyString(body, "importance")
	if importance == "" {
		importance = "normal"
	}
	return todoTask{ID: id, Title: title, Status: status, Importance: importance, ETag: todoNowMillis(), LastModifiedDateTime: time.Now().Format(time.RFC3339)}
}
func (a *Service) enqueueTodoOp(cache *todoListCache, op todoPendingOp) {
	if !a.todoListCloudSyncEnabled(op.ListID) {
		return
	}
	op.Created = time.Now().UnixMilli()
	cache.PendingOps = append(cache.PendingOps, op)
}
func (a *Service) upsertTodoTask(listID string, body map[string]any) (todoListCache, error) {
	if err := validateTodoTaskBody(body); err != nil {
		return todoListCache{}, err
	}
	unlock := a.todoListLock(listID)
	defer unlock()
	cache := a.readTodoListCache(listID)
	id := jsonutil.BodyString(body, "id")
	if id == "" {
		id = fmt.Sprintf("local-%d", time.Now().UnixNano())
	}
	next := todoTaskFromBody(id, body)
	if a.todoListCloudSyncEnabled(listID) {
		next.Pending = "create"
		a.enqueueTodoOp(&cache, todoPendingOp{Op: "create", ListID: listID, TaskID: id, Payload: body})
	}
	cache.Tasks = append([]todoTask{next}, cache.Tasks...)
	if err := a.writeTodoListCache(cache); err != nil {
		return cache, err
	}
	if a.todoListCloudSyncEnabled(listID) {
		go a.todoStartDrain(listID)
	}
	return cache, nil
}

func (a *Service) patchTodoTask(listID, taskID string, body map[string]any) (todoListCache, error) {
	if err := validateTodoTaskBody(body); err != nil {
		return todoListCache{}, err
	}
	unlock := a.todoListLock(listID)
	defer unlock()
	cache := a.readTodoListCache(listID)
	if !a.applyTodoTaskPatch(&cache, listID, taskID, body) {
		return cache, fmt.Errorf("task is no longer available")
	}
	if err := a.writeTodoListCache(cache); err != nil {
		return cache, err
	}
	if a.todoListCloudSyncEnabled(listID) {
		go a.todoStartDrain(listID)
	}
	return cache, nil
}

func (a *Service) deleteTodoTask(listID, taskID string) (todoListCache, error) {
	unlock := a.todoListLock(listID)
	defer unlock()
	cache := a.readTodoListCache(listID)
	found := false
	kept := make([]todoTask, 0, len(cache.Tasks))
	for _, task := range cache.Tasks {
		if task.ID == taskID {
			found = true
			continue
		}
		kept = append(kept, task)
	}
	if !found {
		return cache, fmt.Errorf("task is no longer available")
	}
	cache.Tasks = kept
	a.enqueueTodoOp(&cache, todoPendingOp{Op: "delete", ListID: listID, TaskID: taskID})
	if err := a.writeTodoListCache(cache); err != nil {
		return cache, err
	}
	if a.todoListCloudSyncEnabled(listID) {
		go a.todoStartDrain(listID)
	}
	return cache, nil
}

// clearTodoCompletedSnapshot deletes only completed IDs captured before the
// confirmation dialog opened. A legacy caller may omit IDs, in which case the
// server takes its own immediate snapshot; later checkbox taps are never folded
// into an already-confirmed UI snapshot.
func (a *Service) clearTodoCompletedSnapshot(listID string, snapshotIDs []string) (todoListCache, int, error) {
	unlock := a.todoListLock(listID)
	defer unlock()
	cache := a.readTodoListCache(listID)
	wanted := map[string]bool{}
	for _, id := range snapshotIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			wanted[id] = true
		}
	}
	if len(wanted) == 0 {
		for _, task := range cache.Tasks {
			if strings.EqualFold(task.Status, "completed") {
				wanted[task.ID] = true
			}
		}
	}
	completed := make([]todoTask, 0, len(wanted))
	kept := make([]todoTask, 0, len(cache.Tasks))
	for _, task := range cache.Tasks {
		if wanted[task.ID] && strings.EqualFold(task.Status, "completed") {
			completed = append(completed, task)
			a.enqueueTodoOp(&cache, todoPendingOp{Op: "delete", ListID: listID, TaskID: task.ID})
			continue
		}
		kept = append(kept, task)
	}
	if len(completed) == 0 {
		return cache, 0, nil
	}
	cache.Tasks = kept
	if listID == a.todoMap()["grocery"] {
		a.todoRememberGroceryTasks(completed)
	}
	if err := a.writeTodoListCache(cache); err != nil {
		return cache, 0, err
	}
	if a.todoListCloudSyncEnabled(listID) {
		go a.todoStartDrain(listID)
	}
	return cache, len(completed), nil
}
