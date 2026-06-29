package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func groceryMemoryByKey(items []todoGroceryMemoryItem, key string) (todoGroceryMemoryItem, bool) {
	for _, item := range items {
		if item.Key == key {
			return item, true
		}
	}
	return todoGroceryMemoryItem{}, false
}

func TestTodoGroceryMemoryRenameKeepsLearningAlias(t *testing.T) {
	a := newTodoTestApp(t)
	if _, err := a.addTodoGroceryMemoryItem("Milk"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.editTodoGroceryMemoryItem("milk", "Oat milk"); err != nil {
		t.Fatal(err)
	}
	a.todoRememberGroceryTasks([]todoTask{{Title: "Milk"}})
	items := a.todoGroceryMemory()
	if len(items) != 1 {
		t.Fatalf("memory item count = %d, want one: %#v", len(items), items)
	}
	item, ok := groceryMemoryByKey(items, "oat milk")
	if !ok {
		t.Fatalf("renamed item missing: %#v", items)
	}
	if item.Title != "Oat milk" || item.Uses != 1 || !strings.Contains(strings.Join(item.Aliases, ","), "milk") {
		t.Fatalf("renamed item did not retain alias learning: %#v", item)
	}
}

func TestTodoGroceryMemoryHideSuppressesAutomaticReturn(t *testing.T) {
	a := newTodoTestApp(t)
	a.todoRememberGroceryTasks([]todoTask{{Title: "Milk"}})
	if _, err := a.hideTodoGroceryMemoryItem("milk"); err != nil {
		t.Fatal(err)
	}
	a.todoRememberGroceryTasks([]todoTask{{Title: "Milk"}})
	items := a.todoGroceryMemory()
	if len(items) != 1 || !items[0].Hidden || items[0].Uses != 1 {
		t.Fatalf("hidden suggestion returned or re-learned: %#v", items)
	}
	if _, err := a.restoreTodoGroceryMemoryItem("milk"); err != nil {
		t.Fatal(err)
	}
	items = a.todoGroceryMemory()
	if len(items) != 1 || items[0].Hidden {
		t.Fatalf("restore did not make the suggestion available: %#v", items)
	}
}

func TestTodoGroceryMemoryDeleteRequiresHiddenSuggestionAndAllowsFreshLearning(t *testing.T) {
	a := newTodoTestApp(t)
	if _, err := a.addTodoGroceryMemoryItem("Milk"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.deleteTodoGroceryMemoryItem("milk"); err == nil {
		t.Fatal("visible Quick add suggestion must retain the reversible Hide path")
	}
	if _, err := a.hideTodoGroceryMemoryItem("milk"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.deleteTodoGroceryMemoryItem("milk"); err != nil {
		t.Fatal(err)
	}
	if items := a.todoGroceryMemory(); len(items) != 0 {
		t.Fatalf("deleted hidden suggestion persisted: %#v", items)
	}
	a.todoRememberGroceryTasks([]todoTask{{Title: "Milk"}})
	items := a.todoGroceryMemory()
	item, ok := groceryMemoryByKey(items, "milk")
	if !ok || item.Hidden || item.Uses != 1 {
		t.Fatalf("a later completed Grocery task did not learn a fresh suggestion: %#v", items)
	}
}

func TestTodoGroceryMemoryVisibleRenameWinsHiddenDuplicate(t *testing.T) {
	a := newTodoTestApp(t)
	if _, err := a.addTodoGroceryMemoryItem("Milk"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.hideTodoGroceryMemoryItem("milk"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.addTodoGroceryMemoryItem("Oat milk"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.editTodoGroceryMemoryItem("oat milk", "Milk"); err != nil {
		t.Fatal(err)
	}
	items := a.todoGroceryMemory()
	if len(items) != 1 || items[0].Hidden {
		t.Fatalf("visible rename should restore the merged suggestion: %#v", items)
	}
}

func TestTodoGroceryMemoryDuplicateMergeAndPinnedRetention(t *testing.T) {
	a := newTodoTestApp(t)
	if _, err := a.addTodoGroceryMemoryItem("Eggs"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.addTodoGroceryMemoryItem(" eggs "); err != nil {
		t.Fatal(err)
	}
	if _, err := a.setTodoGroceryMemoryPinned("eggs", true); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < todoGroceryMemoryLimit+12; i++ {
		if _, err := a.addTodoGroceryMemoryItem("Item " + string(rune('A'+(i%26))) + " " + string(rune('a'+((i/26)%26))) + " " + string(rune('a'+((i/676)%26)))); err != nil {
			t.Fatal(err)
		}
	}
	items := a.todoGroceryMemory()
	if len(items) > todoGroceryMemoryLimit {
		t.Fatalf("memory limit not enforced: %d", len(items))
	}
	eggs, ok := groceryMemoryByKey(items, "eggs")
	if !ok || !eggs.Pinned {
		t.Fatalf("pinned entry was trimmed or duplicated: %#v", items)
	}
	count := 0
	for _, item := range items {
		if item.Key == "eggs" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("duplicate quick add entry persisted: %#v", items)
	}
}

func TestTodoGroceryMemoryHTTPDeleteIsCatalogOnly(t *testing.T) {
	a := newTodoTestApp(t)
	if _, err := a.addTodoGroceryMemoryItem("Coffee"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.hideTodoGroceryMemoryItem("coffee"); err != nil {
		t.Fatal(err)
	}
	cache := todoListCache{Version: 1, ListID: todoLocalGroceryListID, Tasks: []todoTask{{ID: "keep", Title: "Keep", Status: "notStarted"}}}
	if err := a.writeTodoListCache(cache); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/todo/grocery-memory", nil)
	if !a.handleTodoPost(w, r, "/api/todo/grocery-memory", map[string]any{"action": "delete", "key": "coffee"}) {
		t.Fatal("quick add delete route was not handled")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("quick add delete returned %d: %s", w.Code, w.Body.String())
	}
	if items := a.todoGroceryMemory(); len(items) != 0 {
		t.Fatalf("quick add delete did not remove only the hidden catalog item: %#v", items)
	}
	if next := a.readTodoListCache(todoLocalGroceryListID); len(next.Tasks) != 1 || next.Tasks[0].ID != "keep" || len(next.PendingOps) != 0 {
		t.Fatalf("catalog-only delete touched Grocery tasks or provider queue: %#v", next)
	}
}

func TestTodoGroceryMemoryHTTPIsLocalOnly(t *testing.T) {
	a := newTodoTestApp(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/todo/grocery-memory", nil)
	if !a.handleTodoPost(w, r, "/api/todo/grocery-memory", map[string]any{"action": "add", "title": "Coffee"}) {
		t.Fatal("quick add route was not handled")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("quick add returned %d: %s", w.Code, w.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response["ok"] != true {
		t.Fatalf("unexpected response: %#v", response)
	}
	if cache := a.readTodoListCache(todoLocalGroceryListID); len(cache.PendingOps) != 0 || len(cache.Tasks) != 0 {
		t.Fatalf("catalog-only mutation touched grocery tasks: %#v", cache)
	}
}
