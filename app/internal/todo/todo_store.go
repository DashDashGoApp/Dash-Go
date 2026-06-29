package todo

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// todo_store.go owns local Lists settings, list/index metadata, cache persistence,
// and the status payload. Task mutation, Graph transport, and migration each live
// in their own runtime-focused domain files.

func todoDefaultLists() []todoListInfo {
	return []todoListInfo{
		{ID: todoLocalTodoListID, DisplayName: "To Do", Origin: todoListOriginLocal},
		{ID: todoLocalGroceryListID, DisplayName: "Grocery", Origin: todoListOriginLocal},
	}
}
func todoDefaultMap() map[string]string {
	return map[string]string{"todo": todoLocalTodoListID, "grocery": todoLocalGroceryListID}
}

func todoListOriginOf(item todoListInfo) string {
	switch strings.ToLower(strings.TrimSpace(item.Origin)) {
	case todoListOriginMicrosoft:
		return todoListOriginMicrosoft
	case todoListOriginLocal:
		return todoListOriginLocal
	}
	if strings.HasPrefix(strings.TrimSpace(item.ID), "local-") {
		return todoListOriginLocal
	}
	// Older releases persisted Graph list records before `origin` existed. Their
	// non-local IDs retain Microsoft behavior for compatibility; queueing still
	// requires a successful lookup in the explicit active-list index.
	return todoListOriginMicrosoft
}

func normalizeTodoListInfo(item todoListInfo) todoListInfo {
	item.ID = strings.TrimSpace(item.ID)
	item.DisplayName = strings.TrimSpace(item.DisplayName)
	item.Origin = todoListOriginOf(item)
	return item
}

func (a *Service) todoListInfoByID(id string) (todoListInfo, bool) {
	id = strings.TrimSpace(id)
	for _, item := range a.readTodoListsIndex().Lists {
		if item.ID == id {
			return normalizeTodoListInfo(item), true
		}
	}
	return todoListInfo{}, false
}

func (a *Service) todoListCloudSyncEnabled(listID string) bool {
	item, ok := a.todoListInfoByID(listID)
	return ok && todoListOriginOf(item) == todoListOriginMicrosoft && a.todoCloudSyncEnabled()
}

func (a *Service) todoHasMappedMicrosoftList() bool {
	for _, listID := range a.todoMap() {
		if item, ok := a.todoListInfoByID(listID); ok && todoListOriginOf(item) == todoListOriginMicrosoft {
			return true
		}
	}
	return false
}
func (a *Service) todoSettings() map[string]any {
	settings := a.loadSettings()
	raw, _ := settings["todo"].(map[string]any)
	out := make(map[string]any, len(raw))
	maps.Copy(out, raw)
	if _, ok := out["map"].(map[string]any); !ok {
		out["map"] = map[string]any{}
	}
	if _, ok := out["cadence"].(map[string]any); !ok {
		out["cadence"] = map[string]any{}
	}
	return out
}
func (a *Service) writeTodoSettings(mut func(map[string]any)) (map[string]any, error) {
	return a.updateSettings(func(settings map[string]any) {
		raw, _ := settings["todo"].(map[string]any)
		if raw == nil {
			raw = map[string]any{}
		}
		mut(raw)
		settings["todo"] = raw
	})
}

// todoDashboardDockEnabled is intentionally default-off. The dock is an
// optional dashboard layout feature, independent of the Apps launcher tiles.
// A malformed persisted value is treated as disabled rather than creating a
// surprise layout change on boot.
func (a *Service) todoDashboardDockEnabled() bool {
	value, ok := a.todoSettings()["dashboardDock"].(bool)
	return ok && value
}

func todoDefaultDashboardDockSlots() map[string]bool {
	return map[string]bool{"todo": true, "grocery": true}
}

// todoDashboardDockSlots keeps the bottom-row selection separate from permanent
// launcher mappings. Missing legacy settings deliberately show both household
// lists; a malformed all-off value falls back to the safe usable default.
func (a *Service) todoDashboardDockSlots() map[string]bool {
	out := todoDefaultDashboardDockSlots()
	raw, _ := a.todoSettings()["dashboardDockSlots"].(map[string]any)
	for _, slot := range []string{"todo", "grocery"} {
		if value, exists := raw[slot]; exists {
			if enabled, ok := value.(bool); ok {
				out[slot] = enabled
			}
		}
	}
	if !out["todo"] && !out["grocery"] {
		return todoDefaultDashboardDockSlots()
	}
	return out
}

func (a *Service) todoDashboardDockVisibleSlots() []string {
	selected := a.todoDashboardDockSlots()
	out := make([]string, 0, len(selected))
	for _, slot := range []string{"todo", "grocery"} {
		if selected[slot] {
			out = append(out, slot)
		}
	}
	return out
}

func todoDashboardDockText(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if value == "" {
		return "Untitled item"
	}
	chars := []rune(value)
	if len(chars) <= 96 {
		return value
	}
	return string(chars[:95]) + "…"
}

// todoDashboardDockSummary reads only already-persisted local list caches. It
// deduplicates shared list mappings, reports the real open-item total, and
// bounds preview payloads for the dashboard ticker.
func (a *Service) todoDashboardDockSummary() todoDashboardDockSummary {
	summary := todoDashboardDockSummary{
		Enabled: a.todoDashboardDockEnabled(),
		Slots:   []todoDashboardDockSlot{},
	}
	selected := a.todoDashboardDockVisibleSlots()
	mapping := a.todoDashboardDockMap()
	byID := map[string]todoListInfo{}
	for _, item := range a.readTodoListsIndex().Lists {
		byID[item.ID] = normalizeTodoListInfo(item)
	}
	seenListIDs := map[string]bool{}
	previewRemaining := todoDashboardDockPreviewLimit
	for _, slot := range selected {
		listID := strings.TrimSpace(mapping[slot])
		if listID == "" || seenListIDs[listID] {
			continue
		}
		seenListIDs[listID] = true
		info, ok := byID[listID]
		title := listID
		if ok && strings.TrimSpace(info.DisplayName) != "" {
			title = info.DisplayName
		}
		entry := todoDashboardDockSlot{
			Slot: slot, ListID: listID, Title: todoDashboardDockText(title), Items: []todoDashboardDockItem{},
		}
		for _, task := range a.readTodoListCache(listID).Tasks {
			if task.Status == "completed" {
				continue
			}
			entry.OpenCount++
			summary.TotalOpenCount++
			if previewRemaining > 0 && len(entry.Items) < todoDashboardDockPerSlotLimit {
				entry.Items = append(entry.Items, todoDashboardDockItem{ID: task.ID, Title: todoDashboardDockText(task.Title), Status: task.Status, Assignee: a.todoTaskAssignmentName(task)})
				previewRemaining--
			}
		}
		summary.Slots = append(summary.Slots, entry)
	}
	return summary
}
func (a *Service) todoSyncMode() string {
	mode := strings.ToLower(jsonutil.StringValue(a.todoSettings()["syncMode"]))
	if mode == todoSyncMicrosoft || mode == todoSyncLocal {
		return mode
	}
	// beta.3 compatibility: a linked token before syncMode existed was opt-in.
	if a.readTodoTokenStore().RefreshToken != "" {
		return todoSyncMicrosoft
	}
	return todoSyncLocal
}
func (a *Service) todoCloudSyncEnabled() bool {
	return a.todoSyncMode() == todoSyncMicrosoft && a.readTodoTokenStore().RefreshToken != ""
}
func (a *Service) todoClientID() string {
	return jsonutil.StringValue(a.todoSettings()["clientId"])
}
func (a *Service) todoMap() map[string]string {
	raw, _ := a.todoSettings()["map"].(map[string]any)
	out := map[string]string{}
	defaults := todoDefaultMap()
	for _, slot := range []string{"todo", "grocery"} {
		if value := jsonutil.StringValue(raw[slot]); value != "" {
			out[slot] = value
			continue
		}
		out[slot] = defaults[slot]
	}
	return out
}

// todoDashboardDockMap is intentionally independent of the permanent App Launcher.
// A missing or legacy-empty mapping receives the normal local default so the
// optional bottom dock remains a layout choice, not an app availability switch.
func (a *Service) todoDashboardDockMap() map[string]string {
	raw, _ := a.todoSettings()["map"].(map[string]any)
	out := map[string]string{}
	defaults := todoDefaultMap()
	for _, slot := range []string{"todo", "grocery"} {
		value, exists := raw[slot]
		id := jsonutil.StringValue(value)
		if exists && id != "" {
			out[slot] = id
			continue
		}
		out[slot] = defaults[slot]
	}
	return out
}
func todoSanitizedID(id string) string {
	sum := sha256.Sum256([]byte(id))
	return hex.EncodeToString(sum[:])[:32]
}
func (a *Service) todoListPath(listID string) string {
	return filepath.Join(a.todoDir, todoSanitizedID(listID)+".json")
}
func (a *Service) todoIndexPath() string { return filepath.Join(a.todoDir, todoStatusFile) }
func mergeTodoDefaultLists(items []todoListInfo, archived map[string]bool) []todoListInfo {
	seen := map[string]bool{}
	out := make([]todoListInfo, 0, len(items)+2)
	for _, raw := range items {
		item := normalizeTodoListInfo(raw)
		if item.ID != "" && !seen[item.ID] {
			seen[item.ID] = true
			out = append(out, item)
		}
	}
	for _, item := range todoDefaultLists() {
		if !seen[item.ID] && !archived[item.ID] {
			seen[item.ID] = true
			out = append(out, item)
		}
	}
	slices.SortStableFunc(out, func(left, right todoListInfo) int {
		return compareFoldedText(left.DisplayName, right.DisplayName)
	})
	return out
}
func (a *Service) readTodoListsIndex() todoListsIndex {
	idx := todoListsIndex{Lists: []todoListInfo{}}
	b, err := os.ReadFile(a.todoIndexPath())
	if err == nil {
		_ = json.Unmarshal(b, &idx)
	}
	idx.Lists = mergeTodoDefaultLists(idx.Lists, a.todoArchivedSourceIDs())
	return idx
}
func (a *Service) writeTodoListsIndex(idx todoListsIndex) error {
	idx.Lists = mergeTodoDefaultLists(idx.Lists, a.todoArchivedSourceIDs())
	idx.UpdatedAt = time.Now().UnixMilli()
	return fileio.WriteJSON(a.todoIndexPath(), idx)
}
func (a *Service) todoUpsertListInfo(item todoListInfo) error {
	item = normalizeTodoListInfo(item)
	if item.ID == "" || item.DisplayName == "" {
		return fmt.Errorf("list ID and display name are required")
	}
	idx := a.readTodoListsIndex()
	for i := range idx.Lists {
		if idx.Lists[i].ID == item.ID {
			idx.Lists[i] = item
			return a.writeTodoListsIndex(idx)
		}
	}
	idx.Lists = append(idx.Lists, item)
	return a.writeTodoListsIndex(idx)
}
func (a *Service) todoRemoveActiveList(listID string) error {
	idx := a.readTodoListsIndex()
	kept := make([]todoListInfo, 0, len(idx.Lists))
	for _, item := range idx.Lists {
		if item.ID != listID {
			kept = append(kept, item)
		}
	}
	idx.Lists = kept
	return a.writeTodoListsIndex(idx)
}
func (a *Service) readTodoListCache(listID string) todoListCache {
	cache := todoListCache{Version: 1, ListID: listID, Tasks: []todoTask{}, PendingOps: []todoPendingOp{}}
	b, err := os.ReadFile(a.todoListPath(listID))
	if err != nil {
		return cache
	}
	if err := json.Unmarshal(b, &cache); err != nil {
		return todoListCache{Version: 1, ListID: listID, Tasks: []todoTask{}, PendingOps: []todoPendingOp{}}
	}
	if cache.Version == 0 {
		cache.Version = 1
	}
	if cache.ListID == "" {
		cache.ListID = listID
	}
	if cache.Tasks == nil {
		cache.Tasks = []todoTask{}
	}
	if cache.PendingOps == nil {
		cache.PendingOps = []todoPendingOp{}
	}
	return cache
}
func (a *Service) writeTodoListCache(cache todoListCache) error {
	if cache.Version == 0 {
		cache.Version = 1
	}
	return fileio.WriteJSON(a.todoListPath(cache.ListID), cache)
}
func (a *Service) todoBlockedWriteStatuses() []todoBlockedWriteStatus {
	statuses := make([]todoBlockedWriteStatus, 0)
	for _, raw := range a.readTodoListsIndex().Lists {
		item := normalizeTodoListInfo(raw)
		if item.ID == "" || todoListOriginOf(item) != todoListOriginMicrosoft {
			continue
		}
		count := todoBlockedPendingCount(a.readTodoListCache(item.ID))
		if count == 0 {
			continue
		}
		statuses = append(statuses, todoBlockedWriteStatus{ListID: item.ID, Title: item.DisplayName, Count: count})
	}
	slices.SortStableFunc(statuses, func(left, right todoBlockedWriteStatus) int {
		return compareFoldedText(left.Title, right.Title)
	})
	return statuses
}

func (a *Service) todoStatusPayload() map[string]any {
	token := a.readTodoTokenStore()
	settings := a.todoSettings()
	syncMode := a.todoSyncMode()
	state := todoSyncLocal
	if syncMode == todoSyncMicrosoft {
		state = "unlinked"
	}
	if token.RefreshToken != "" && syncMode == todoSyncMicrosoft {
		state = "linked"
	}
	a.todoMu.Lock()
	pending := a.todoAuthState
	a.todoMu.Unlock()
	if pending.State == "pending" {
		state = "pending"
	}
	a.todoPurgeExpiredArchives()
	return map[string]any{
		"dashboardDock": a.todoDashboardDockEnabled(), "dashboardDockSlots": a.todoDashboardDockSlots(), "source": "local", "state": state, "syncMode": syncMode,
		"syncActive": a.todoCloudSyncEnabled(), "account": token.Account, "clientId": settings["clientId"],
		"map": a.todoMap(), "dockMap": a.todoDashboardDockMap(), "cadence": settings["cadence"], "inboundSync": a.todoInboundSyncStatus(),
		"people":         a.todoHouseholdPeople(),
		"manualListSync": a.todoManualListSyncStatuses(), "lists": a.readTodoListsIndex().Lists,
		"blockedWrites": a.todoBlockedWriteStatuses(),
		"archives":      a.todoArchiveStatus(), "groceryMemory": a.todoGroceryMemory(), "auth": pending, "linkedAt": token.LinkedAt, "accessExpiresAt": token.AccessExpiresAt,
	}
}
