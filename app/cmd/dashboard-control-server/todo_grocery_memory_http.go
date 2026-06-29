package main

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// handleTodoGroceryMemoryPost owns the local Quick add catalog. These actions
// never mutate a Grocery task, never enqueue Graph work, and remain usable in
// the touch app whether or not Microsoft To Do is linked.
func (a *app) handleTodoGroceryMemoryPost(w http.ResponseWriter, _ *http.Request, path string, body map[string]any) bool {
	if path != "/api/todo/grocery-memory" {
		return false
	}
	action := strings.ToLower(jsonutil.BodyString(body, "action"))
	key := jsonutil.BodyString(body, "key")
	var (
		items []todoGroceryMemoryItem
		err   error
	)
	switch action {
	case "add":
		items, err = a.addTodoGroceryMemoryItem(jsonutil.BodyString(body, "title"))
	case "edit":
		items, err = a.editTodoGroceryMemoryItem(key, jsonutil.BodyString(body, "title"))
	case "pin":
		pinned, ok := body["pinned"].(bool)
		if !ok {
			err = fmt.Errorf("pinned must be true or false")
			break
		}
		items, err = a.setTodoGroceryMemoryPinned(key, pinned)
	case "hide":
		items, err = a.hideTodoGroceryMemoryItem(key)
	case "restore":
		items, err = a.restoreTodoGroceryMemoryItem(key)
	case "delete":
		items, err = a.deleteTodoGroceryMemoryItem(key)
	default:
		err = fmt.Errorf("unknown quick add action")
	}
	if err != nil {
		a.err(w, err.Error(), http.StatusBadRequest)
		return true
	}
	a.todoEmit(map[string]any{"type": "grocery.memory"})
	a.json(w, map[string]any{"ok": true, "groceryMemory": items})
	return true
}
