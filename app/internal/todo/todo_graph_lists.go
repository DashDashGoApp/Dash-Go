package todo

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func todoTaskGraphBody(task todoTask, body map[string]any) map[string]any {
	out := map[string]any{"title": task.Title, "status": task.Status, "importance": task.Importance}
	for _, key := range []string{"title", "status", "importance", "dueDateTime", "body"} {
		if value, ok := body[key]; ok && value != nil {
			out[key] = value
		}
	}
	return out
}

// todoTaskGraphPatchBody is intentionally an allowlist. Dash-Go-only
// responsibility metadata must never be sent to Microsoft Graph.
func todoTaskGraphPatchBody(body map[string]any) map[string]any {
	out := map[string]any{}
	for _, key := range []string{"title", "status", "importance", "dueDateTime", "body"} {
		if value, ok := body[key]; ok && value != nil {
			out[key] = value
		}
	}
	return out
}

func (a *Service) todoDeleteCloudList(ctx context.Context, listID string) error {
	payload, status, err := a.todoGraph(ctx, http.MethodDelete, "/me/todo/lists/"+todoGraphPathID(listID), nil)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return todoGraphError(payload, "Microsoft To Do could not clean up the incomplete migration list")
	}
	return nil
}

func (a *Service) todoCreateCloudList(ctx context.Context, name string) (todoListInfo, error) {
	payload, status, err := a.todoGraph(ctx, http.MethodPost, "/me/todo/lists", map[string]any{"displayName": name})
	if err != nil {
		return todoListInfo{}, err
	}
	if status < 200 || status >= 300 {
		return todoListInfo{}, todoGraphError(payload, "Microsoft To Do could not create the list")
	}
	id := jsonutil.StringValue(payload["id"])
	if id == "" {
		return todoListInfo{}, fmt.Errorf("Microsoft To Do returned no list ID")
	}
	return todoListInfo{ID: id, DisplayName: jsonutil.StringValue(payload["displayName"]), Origin: todoListOriginMicrosoft}, nil
}

// todoInitialListsDeltaEndpoint deliberately uses Graph's bare documented
// bootstrap route. Optional $select is avoided only for the first request;
// Graph-returned opaque continuation URLs remain untouched thereafter.
func todoInitialListsDeltaEndpoint() string {
	return "/me/todo/lists/delta"
}

func todoFinalListDeltaRows(rows []map[string]any) map[string]map[string]any {
	final := map[string]map[string]any{}
	for _, raw := range rows {
		id := todoGraphItemID(raw)
		if id == "" {
			continue
		}
		if todoGraphRemoved(raw) {
			final[id] = todoCloneGraphRow(raw)
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
	}
	return final
}

func todoListInfoPatchFromGraph(current todoListInfo, raw map[string]any) (todoListInfo, bool) {
	id := todoGraphItemID(raw)
	if id == "" {
		return todoListInfo{}, false
	}
	current.ID = id
	current.Origin = todoListOriginMicrosoft
	if value, exists := raw["displayName"]; exists {
		if name := jsonutil.StringValue(value); name != "" {
			current.DisplayName = name
		}
	}
	if value, exists := raw["wellknownListName"]; exists {
		if _, ok := value.(string); ok {
			current.WellknownName = jsonutil.StringValue(value)
		}
	}
	return normalizeTodoListInfo(current), current.DisplayName != ""
}

func todoApplyListsDelta(idx *todoListsIndex, rows []map[string]any, fullBaseline bool, archived map[string]bool) {
	final := todoFinalListDeltaRows(rows)
	if fullBaseline {
		kept := make([]todoListInfo, 0, len(idx.Lists)+len(final))
		for _, item := range idx.Lists {
			if todoListOriginOf(item) != todoListOriginMicrosoft {
				kept = append(kept, item)
			}
		}
		for _, raw := range final {
			if todoGraphRemoved(raw) {
				continue
			}
			if item, ok := todoListInfoPatchFromGraph(todoListInfo{}, raw); ok && !archived[item.ID] {
				kept = append(kept, item)
			}
		}
		idx.Lists = kept
		return
	}
	byID := map[string]todoListInfo{}
	for _, item := range idx.Lists {
		byID[item.ID] = item
	}
	for id, raw := range final {
		if todoGraphRemoved(raw) {
			if item, exists := byID[id]; exists && todoListOriginOf(item) == todoListOriginMicrosoft {
				delete(byID, id)
			}
			continue
		}
		if item, ok := todoListInfoPatchFromGraph(byID[id], raw); ok && !archived[item.ID] {
			byID[item.ID] = item
		}
	}
	idx.Lists = make([]todoListInfo, 0, len(byID))
	for _, item := range byID {
		idx.Lists = append(idx.Lists, item)
	}
}

// syncTodoListsNow tracks the list collection with Graph delta state. A list
// refresh after linking establishes an authoritative set, then later refreshes
// only reconcile changes. Opaque next/delta URLs are always reused verbatim.
func (a *Service) syncTodoListsNow(ctx context.Context) error {
	if !a.todoCloudSyncEnabled() {
		return nil
	}
	a.todoCloudMu.Lock()
	defer a.todoCloudMu.Unlock()
	idx := a.readTodoListsIndex()
	fullBaseline := strings.TrimSpace(idx.DeltaLink) == ""
	endpoint := idx.DeltaLink
	if endpoint == "" {
		endpoint = todoInitialListsDeltaEndpoint()
	}
	rows := make([]map[string]any, 0)
	for range todoGraphDeltaMaxPages {
		payload, meta, err := a.todoGraphResponse(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return todoGraphFailureFor("list-discovery", "", "Microsoft To Do list refresh failed", payload, meta, err)
		}
		if todoGraphNeedsDeltaReset(meta) && !fullBaseline {
			fullBaseline = true
			rows = rows[:0]
			endpoint = meta.Location
			if endpoint == "" {
				endpoint = todoInitialListsDeltaEndpoint()
			}
			continue
		}
		if todoGraphIsThrottle(meta) {
			failure := todoGraphFailureFor("list-discovery", "", "Microsoft To Do list refresh was throttled", payload, meta, nil)
			return &todoThrottleError{RetryAfter: meta.RetryAfter, Err: failure}
		}
		if meta.Status < 200 || meta.Status >= 300 {
			return todoGraphFailureFor("list-discovery", "", "Microsoft To Do list refresh failed", payload, meta, nil)
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
			return fmt.Errorf("Microsoft To Do list delta response did not include a continuation token")
		}
		todoApplyListsDelta(&idx, rows, fullBaseline, a.todoArchivedSourceIDs())
		idx.DeltaLink = deltaLink
		if err := a.writeTodoListsIndex(idx); err != nil {
			return err
		}
		a.todoEmit(map[string]any{"type": "list.index"})
		return nil
	}
	return fmt.Errorf("Microsoft To Do returned too many list delta pages")
}
