package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTodoDashboardDockDefaultsSelectBothPermanentLists(t *testing.T) {
	a := newTodoTestApp(t)
	slots := a.todoDashboardDockSlots()
	if !slots["todo"] || !slots["grocery"] {
		t.Fatalf("legacy/missing dock slots = %#v, want both permanent defaults", slots)
	}
	if a.todoDashboardDockEnabled() {
		t.Fatal("Bottom Lists dock must remain default-off even though both source lists are selected")
	}
	if _, err := a.writeTodoSettings(func(todo map[string]any) { todo["dashboardDock"] = true }); err != nil {
		t.Fatal(err)
	}
	status := a.todoStatusPayload()
	statusSlots, ok := status["dashboardDockSlots"].(map[string]bool)
	if !ok || !statusSlots["todo"] || !statusSlots["grocery"] {
		t.Fatalf("status dock slots = %#v, want both selected", status["dashboardDockSlots"])
	}
}

func TestTodoDashboardDockSummaryIsBoundedAndDeduplicatesSharedMappings(t *testing.T) {
	a := newTodoTestApp(t)
	if _, err := a.writeTodoSettings(func(todo map[string]any) {
		todo["dashboardDock"] = true
		todo["dashboardDockSlots"] = map[string]any{"todo": true, "grocery": true}
		todo["map"] = map[string]any{"todo": todoLocalTodoListID, "grocery": todoLocalTodoListID}
	}); err != nil {
		t.Fatal(err)
	}
	for range todoDashboardDockPreviewLimit + 3 {
		if _, err := a.upsertTodoTask(todoLocalTodoListID, map[string]any{"title": "  Grocery\nitem  "}); err != nil {
			t.Fatal(err)
		}
	}
	summary := a.todoDashboardDockSummary()
	if !summary.Enabled || summary.TotalOpenCount != todoDashboardDockPreviewLimit+3 {
		t.Fatalf("summary = %#v, want enabled with every open item counted once", summary)
	}
	if len(summary.Slots) != 1 || summary.Slots[0].Slot != "todo" {
		t.Fatalf("shared list mapping must appear once in stable slot order: %#v", summary.Slots)
	}
	if len(summary.Slots[0].Items) > todoDashboardDockPerSlotLimit || len(summary.Slots[0].Items) > todoDashboardDockPreviewLimit {
		t.Fatalf("dock preview was not bounded: %#v", summary.Slots[0].Items)
	}
	if got := summary.Slots[0].Items[0].Title; got != "Grocery item" {
		t.Fatalf("dock preview text = %q, want normalized bounded title", got)
	}
}

func TestTodoDashboardDockSlotEndpointKeepsOneSelectedList(t *testing.T) {
	a := newTodoTestApp(t)
	request := func(body map[string]any) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/todo/dock/slots", nil)
		r.Header.Set("X-Dashboard-Token", a.issueToken())
		if !a.handleTodoPost(w, r, "/api/todo/dock/slots", body) {
			t.Fatal("dashboard dock slot endpoint was not handled")
		}
		return w
	}

	w := request(map[string]any{"slots": map[string]any{"grocery": false}})
	if w.Code != http.StatusOK {
		t.Fatalf("hide Grocery returned %d: %s", w.Code, w.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	slots, ok := payload["dashboardDockSlots"].(map[string]any)
	if !ok || slots["todo"] != true || slots["grocery"] != false {
		t.Fatalf("slot selection payload = %#v", payload["dashboardDockSlots"])
	}

	w = request(map[string]any{"slots": map[string]any{"todo": false}})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("last selected list returned %d, want 400: %s", w.Code, w.Body.String())
	}
	persisted := a.todoDashboardDockSlots()
	if !persisted["todo"] || persisted["grocery"] {
		t.Fatalf("failed final-slot removal changed persisted selection: %#v", persisted)
	}
}

func TestTodoDashboardDockEndpointReturnsCacheOnlySummary(t *testing.T) {
	a := newTodoTestApp(t)
	if _, err := a.writeTodoSettings(func(todo map[string]any) { todo["dashboardDock"] = true }); err != nil {
		t.Fatal(err)
	}
	if _, err := a.upsertTodoTask(todoLocalGroceryListID, map[string]any{"title": "Milk"}); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/todo/dock", nil)
	if !a.handleTodoGet(w, r, "/api/todo/dock") {
		t.Fatal("dashboard dock summary route was not handled")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("dashboard dock summary returned %d: %s", w.Code, w.Body.String())
	}
	var summary todoDashboardDockSummary
	if err := json.Unmarshal(w.Body.Bytes(), &summary); err != nil {
		t.Fatalf("dock summary returned invalid JSON: %v", err)
	}
	if summary.TotalOpenCount != 1 || len(summary.Slots) != 2 {
		t.Fatalf("dock summary = %#v, want cache-only selected list summaries", summary)
	}
}
