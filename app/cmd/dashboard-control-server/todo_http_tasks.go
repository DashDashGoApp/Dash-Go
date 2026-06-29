package main

import (
	"net/http"
	"strings"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
	todopkg "github.com/DashDashGoApp/Dash-Go/app/internal/todo"
)

// handleTodoTasksGet is cache-only by contract. Rendering the dashboard or
// handling an SSE refresh must never start a Microsoft Graph request.
func (a *app) handleTodoTasksGet(w http.ResponseWriter, _ *http.Request, path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 5 || parts[0] != "api" || parts[1] != "todo" || parts[2] != "lists" || parts[4] != "tasks" {
		return false
	}
	a.json(w, a.readTodoListCache(parts[3]))
	return true
}

func todoTaskIDsFromBody(raw any) []string {
	values, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, rawID := range values {
		id := jsonutil.StringValue(rawID)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func (a *app) handleTodoTasksPost(w http.ResponseWriter, _ *http.Request, parts []string, body map[string]any) bool {
	if len(parts) < 5 || parts[0] != "api" || parts[1] != "todo" || parts[2] != "lists" {
		return false
	}
	listID := parts[3]
	if err := todopkg.ValidateListID(listID); err != nil {
		a.err(w, err.Error(), http.StatusBadRequest)
		return true
	}
	if len(parts) == 5 && parts[4] == "sync-now" {
		result := a.todoRequestManualListSync(listID)
		a.json(w, map[string]any{
			"ok":          result.Accepted,
			"accepted":    result.Accepted,
			"started":     result.Started,
			"queued":      result.Queued,
			"reason":      result.Reason,
			"manualSync":  result.ManualSync,
			"inboundSync": a.todoInboundSyncStatus(),
			"cache":       a.readTodoListCache(listID),
		})
		return true
	}
	if len(parts) == 5 && parts[4] == "sync" {
		if !a.todoListCloudSyncEnabled(listID) {
			a.json(w, map[string]any{"ok": true, "queued": false, "reason": "list is local only", "cache": a.readTodoListCache(listID)})
			return true
		}
		a.todoStartInboundSyncForList(listID)
		a.json(w, map[string]any{"ok": true, "queued": true, "cache": a.readTodoListCache(listID), "inboundSync": a.todoInboundSyncStatus()})
		return true
	}
	if parts[4] != "tasks" {
		return false
	}
	if len(parts) == 6 && parts[5] == "batch" {
		requests, err := todoTaskPatchRequests(body["patches"])
		if err != nil {
			a.err(w, err.Error(), http.StatusBadRequest)
			return true
		}
		cache, updated, err := a.patchTodoTasksBatch(listID, requests)
		if err != nil {
			a.err(w, err.Error(), http.StatusConflict)
			return true
		}
		a.todoEmit(map[string]any{"type": "task.batch", "listId": listID, "count": updated})
		a.json(w, map[string]any{"ok": true, "updated": updated, "cache": cache})
		return true
	}
	if len(parts) == 5 {
		if err := todopkg.ValidateTaskBody(body); err != nil {
			a.err(w, err.Error(), http.StatusBadRequest)
			return true
		}
		cache, err := a.upsertTodoTask(listID, body)
		if err != nil {
			a.err(w, err.Error(), http.StatusInternalServerError)
			return true
		}
		a.todoEmit(map[string]any{"type": "task.upsert", "listId": listID})
		a.json(w, cache)
		return true
	}
	if len(parts) == 6 {
		if err := todopkg.ValidateTaskID(parts[5]); err != nil {
			a.err(w, err.Error(), http.StatusBadRequest)
			return true
		}
		if err := todopkg.ValidateTaskBody(body); err != nil {
			a.err(w, err.Error(), http.StatusBadRequest)
			return true
		}
		cache, err := a.patchTodoTask(listID, parts[5], body)
		if err != nil {
			a.err(w, err.Error(), http.StatusConflict)
			return true
		}
		a.todoEmit(map[string]any{"type": "task.upsert", "listId": listID, "taskId": parts[5]})
		a.json(w, cache)
		return true
	}
	if len(parts) == 7 && parts[6] == "assignment" {
		if err := todopkg.ValidateTaskID(parts[5]); err != nil {
			a.err(w, err.Error(), http.StatusBadRequest)
			return true
		}
		personID := routinesID(body["personId"])
		cache, err := a.todoAssignTaskPerson(listID, parts[5], personID)
		if err != nil {
			a.err(w, err.Error(), http.StatusConflict)
			return true
		}
		a.todoEmit(map[string]any{"type": "task.assignment", "listId": listID, "taskId": parts[5]})
		a.json(w, cache)
		return true
	}
	if len(parts) == 7 && parts[6] == "delete" {
		if err := todopkg.ValidateTaskID(parts[5]); err != nil {
			a.err(w, err.Error(), http.StatusBadRequest)
			return true
		}
		cache, err := a.deleteTodoTask(listID, parts[5])
		if err != nil {
			a.err(w, err.Error(), http.StatusConflict)
			return true
		}
		a.todoEmit(map[string]any{"type": "task.remove", "listId": listID, "taskId": parts[5]})
		a.json(w, cache)
		return true
	}
	return false
}
