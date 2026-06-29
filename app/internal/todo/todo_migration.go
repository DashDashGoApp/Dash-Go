package todo

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
)

const (
	todoMigrationArchiveRetention = 90 * 24 * time.Hour
	todoMigrationTaskLimit        = 250
)

func (a *Service) todoArchiveDir() string { return filepath.Join(a.todoDir, "archives") }
func (a *Service) todoArchiveIndexPath() string {
	return filepath.Join(a.todoArchiveDir(), "_archives.json")
}
func (a *Service) todoArchiveSnapshotPath(id string) string {
	return filepath.Join(a.todoArchiveDir(), todoSanitizedID(id)+".json")
}

func (a *Service) readTodoArchiveIndexLocked() todoMigrationArchiveIndex {
	out := todoMigrationArchiveIndex{Version: 1, Items: []todoMigrationArchive{}}
	if b, err := os.ReadFile(a.todoArchiveIndexPath()); err == nil {
		_ = json.Unmarshal(b, &out)
	}
	if out.Version == 0 {
		out.Version = 1
	}
	if out.Items == nil {
		out.Items = []todoMigrationArchive{}
	}
	return out
}

func (a *Service) readTodoArchiveIndex() todoMigrationArchiveIndex {
	a.todoArchiveMu.Lock()
	defer a.todoArchiveMu.Unlock()
	return a.readTodoArchiveIndexLocked()
}

func (a *Service) todoArchivedSourceIDs() map[string]bool {
	idx := a.readTodoArchiveIndex()
	out := map[string]bool{}
	for _, item := range idx.Items {
		// The snapshot expires after 90 days, but the deliberate archive action
		// permanently retires the source from Dash-Go mappings. This prevents a
		// built-in local default or an unchanged Microsoft source list from quietly
		// returning after the safety-retention window closes.
		if item.Source.ID != "" {
			out[item.Source.ID] = true
		}
	}
	return out
}

func (a *Service) todoArchiveStatus() []todoMigrationArchive {
	idx := a.readTodoArchiveIndex()
	now := todoNowMillis()
	out := make([]todoMigrationArchive, 0, len(idx.Items))
	for _, item := range idx.Items {
		if item.ExpiresAt > now {
			out = append(out, item)
		}
	}
	return out
}

func (a *Service) todoStageArchive(source todoListInfo, cache todoListCache, reason string) (todoMigrationArchive, error) {
	source = normalizeTodoListInfo(source)
	if source.ID == "" {
		return todoMigrationArchive{}, fmt.Errorf("migration source is missing")
	}
	if err := os.MkdirAll(a.todoArchiveDir(), 0700); err != nil {
		return todoMigrationArchive{}, err
	}
	now := todoNowMillis()
	archive := todoMigrationArchive{
		ID:         fmt.Sprintf("archive-%d-%s", now, todoSanitizedID(source.ID)[:8]),
		Source:     source,
		Reason:     reason,
		TaskCount:  len(cache.Tasks),
		ArchivedAt: now,
		ExpiresAt:  time.Now().Add(todoMigrationArchiveRetention).UnixMilli(),
	}
	snapshot := todoMigrationArchiveSnapshot{Version: 1, Archive: archive, Cache: cache}
	if err := fileio.WriteJSON(a.todoArchiveSnapshotPath(archive.ID), snapshot); err != nil {
		return todoMigrationArchive{}, err
	}
	return archive, nil
}

func (a *Service) todoDiscardStagedArchive(archive todoMigrationArchive) {
	if archive.ID != "" {
		_ = os.Remove(a.todoArchiveSnapshotPath(archive.ID))
	}
}

// todoCommitArchive promotes a staged immutable snapshot into the 90-day
// archive index and removes the source from the active Dash-Go list registry.
// The source snapshot remains local only; it never deletes a Microsoft list.
func (a *Service) todoCommitArchive(archive todoMigrationArchive) error {
	if archive.ID == "" || archive.Source.ID == "" {
		return fmt.Errorf("invalid staged migration archive")
	}
	a.todoArchiveMu.Lock()
	idx := a.readTodoArchiveIndexLocked()
	old := append([]todoMigrationArchive{}, idx.Items...)
	idx.Items = append(idx.Items, archive)
	if err := fileio.WriteJSON(a.todoArchiveIndexPath(), idx); err != nil {
		a.todoArchiveMu.Unlock()
		return err
	}
	a.todoArchiveMu.Unlock()

	if err := a.todoRemoveActiveList(archive.Source.ID); err != nil {
		a.todoArchiveMu.Lock()
		rollback := todoMigrationArchiveIndex{Version: 1, Items: old}
		_ = fileio.WriteJSON(a.todoArchiveIndexPath(), rollback)
		a.todoArchiveMu.Unlock()
		return err
	}
	_ = os.Remove(a.todoListPath(archive.Source.ID))
	a.todoEmit(map[string]any{"type": "list.archived", "listId": archive.Source.ID, "archiveId": archive.ID, "expiresAt": archive.ExpiresAt})
	return nil
}

func (a *Service) todoPurgeExpiredArchives() {
	now := todoNowMillis()
	a.todoArchiveMu.Lock()
	idx := a.readTodoArchiveIndexLocked()
	expired := make([]todoMigrationArchive, 0)
	changed := false
	for i := range idx.Items {
		item := &idx.Items[i]
		if item.PurgedAt == 0 && item.ExpiresAt > 0 && item.ExpiresAt <= now {
			item.PurgedAt = now
			expired = append(expired, *item)
			changed = true
		}
	}
	if changed {
		_ = fileio.WriteJSON(a.todoArchiveIndexPath(), idx)
	}
	a.todoArchiveMu.Unlock()
	for _, item := range expired {
		_ = os.Remove(a.todoArchiveSnapshotPath(item.ID))
		a.todoEmit(map[string]any{"type": "archive.purged", "archiveId": item.ID})
	}
}

func (a *Service) startTodoArchiveJanitor() {
	a.todoPurgeExpiredArchives()
	go func() {
		ticker := time.NewTicker(12 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			a.todoPurgeExpiredArchives()
		}
	}()
}

func (a *Service) todoMappedListInfo(slot string) (todoListInfo, error) {
	if slot != "todo" && slot != "grocery" {
		return todoListInfo{}, fmt.Errorf("slot must be todo or grocery")
	}
	listID := a.todoMap()[slot]
	if listID == "" {
		return todoListInfo{}, fmt.Errorf("%s is not mapped", slot)
	}
	item, ok := a.todoListInfoByID(listID)
	if !ok {
		return todoListInfo{}, fmt.Errorf("mapped list is not available")
	}
	return item, nil
}

func todoMigrationTaskBody(task todoTask) map[string]any {
	body := map[string]any{"title": task.Title, "status": task.Status, "importance": task.Importance}
	if task.DueDateTime != nil {
		body["dueDateTime"] = task.DueDateTime
	}
	if task.Body != nil {
		body["body"] = task.Body
	}
	return body
}

func (a *Service) todoCopyCacheToMicrosoft(ctx context.Context, destinationListID string, cache todoListCache) error {
	if len(cache.Tasks) > todoMigrationTaskLimit {
		return fmt.Errorf("migration is limited to %d items at a time", todoMigrationTaskLimit)
	}
	for _, task := range cache.Tasks {
		payload, status, err := a.todoGraph(ctx, "POST", "/me/todo/lists/"+todoGraphPathID(destinationListID)+"/tasks", todoMigrationTaskBody(task))
		if err != nil {
			return err
		}
		if status < 200 || status >= 300 {
			return todoGraphError(payload, "Microsoft To Do could not copy a list item")
		}
	}
	return nil
}

func todoCloneCacheForLocal(destination todoListInfo, source todoListCache) todoListCache {
	out := todoListCache{Version: 1, ListID: destination.ID, DisplayName: destination.DisplayName, Tasks: make([]todoTask, 0, len(source.Tasks)), PendingOps: []todoPendingOp{}}
	for i, task := range source.Tasks {
		copy := todoTaskFromBody(fmt.Sprintf("local-%d-%d", time.Now().UnixNano(), i), todoMigrationTaskBody(task))
		copy.DueDateTime = task.DueDateTime
		copy.Body = task.Body
		copy.ChecklistItems = append([]todoChecklistItem{}, task.ChecklistItems...)
		copy.Pending = ""
		copy.SyncFailed = false
		copy.DashGoAssignment = todoAssignmentCopy(task.DashGoAssignment)
		out.Tasks = append(out.Tasks, copy)
	}
	return out
}

func (a *Service) todoSetMappedSlot(slot, listID string) error {
	_, err := a.writeTodoSettings(func(todo map[string]any) {
		m, _ := todo["map"].(map[string]any)
		if m == nil {
			m = map[string]any{}
		}
		m[slot] = listID
		todo["map"] = m
	})
	return err
}

func (a *Service) migrateTodoSlot(ctx context.Context, action, slot string) (map[string]any, error) {
	// A migration changes one map entry, an active-list index, cache files, and a
	// retention snapshot. Serialize the short workflow so two confirmation taps or
	// two Control sessions cannot interleave those state changes.
	a.todoMigrationMu.Lock()
	defer a.todoMigrationMu.Unlock()
	a.todoPurgeExpiredArchives()
	source, err := a.todoMappedListInfo(slot)
	if err != nil {
		return nil, err
	}
	for mappedSlot, mappedID := range a.todoMap() {
		if mappedSlot != slot && mappedID == source.ID {
			return nil, fmt.Errorf("%s is mapped to both launcher tiles; map the other tile to a different list before migrating it", source.DisplayName)
		}
	}
	sourceCache := a.readTodoListCache(source.ID)
	action = strings.ToLower(strings.TrimSpace(action))
	copyItems := strings.HasSuffix(action, "-copy")
	localToMicrosoft := strings.HasPrefix(action, "local-to-microsoft-")
	microsoftToLocal := strings.HasPrefix(action, "microsoft-to-local-")
	if !copyItems && !strings.HasSuffix(action, "-fresh") {
		return nil, fmt.Errorf("unknown migration action")
	}

	var destination todoListInfo
	if localToMicrosoft {
		if todoListOriginOf(source) != todoListOriginLocal {
			return nil, fmt.Errorf("the selected list is already Microsoft-backed")
		}
		if !a.todoCloudSyncEnabled() {
			return nil, fmt.Errorf("link Microsoft before moving a list to Microsoft")
		}
		destination, err = a.todoCreateCloudList(ctx, source.DisplayName)
		if err != nil {
			return nil, err
		}
		if copyItems {
			if err := a.todoCopyCacheToMicrosoft(ctx, destination.ID, sourceCache); err != nil {
				if cleanupErr := a.todoDeleteCloudList(ctx, destination.ID); cleanupErr != nil {
					return nil, fmt.Errorf("copy stopped before switching the mapped list: %w (the local source is untouched; remove the un-mapped Microsoft list manually: %v)", err, cleanupErr)
				}
				return nil, fmt.Errorf("copy stopped before switching the mapped list; the local source is untouched: %w", err)
			}
		}
	} else if microsoftToLocal {
		if todoListOriginOf(source) != todoListOriginMicrosoft {
			return nil, fmt.Errorf("the selected list is already local")
		}
		if !a.todoCloudSyncEnabled() {
			return nil, fmt.Errorf("Microsoft must still be linked to return a Microsoft list to local storage")
		}
		if err := a.syncTodoListNow(ctx, source.ID); err != nil {
			return nil, fmt.Errorf("could not refresh the Microsoft list before migration: %w", err)
		}
		sourceCache = a.readTodoListCache(source.ID)
		destination = todoListInfo{ID: fmt.Sprintf("local-list-%d", time.Now().UnixNano()), DisplayName: source.DisplayName, Origin: todoListOriginLocal}
	} else {
		return nil, fmt.Errorf("unknown migration action")
	}

	archive, err := a.todoStageArchive(source, sourceCache, action)
	if err != nil {
		return nil, err
	}
	if err := a.todoUpsertListInfo(destination); err != nil {
		a.todoDiscardStagedArchive(archive)
		return nil, err
	}
	if microsoftToLocal {
		cache := todoListCache{Version: 1, ListID: destination.ID, DisplayName: destination.DisplayName, Tasks: []todoTask{}, PendingOps: []todoPendingOp{}}
		if copyItems {
			cache = todoCloneCacheForLocal(destination, sourceCache)
		}
		if err := a.writeTodoListCache(cache); err != nil {
			a.todoDiscardStagedArchive(archive)
			return nil, err
		}
	}
	if err := a.todoSetMappedSlot(slot, destination.ID); err != nil {
		a.todoDiscardStagedArchive(archive)
		return nil, err
	}
	if err := a.todoCommitArchive(archive); err != nil {
		return nil, fmt.Errorf("the new mapping was saved but the old list could not be archived: %w", err)
	}
	if microsoftToLocal && !a.todoHasMappedMicrosoftList() {
		if err := a.unlinkTodo(); err != nil {
			return nil, fmt.Errorf("the local list is ready but Microsoft could not be unlinked: %w", err)
		}
	}
	a.todoNotifyInboundScheduler()
	a.todoEmit(map[string]any{"type": "list.migrated", "slot": slot, "sourceListId": source.ID, "destinationListId": destination.ID, "archiveId": archive.ID})
	return map[string]any{
		"ok":          true,
		"slot":        slot,
		"source":      source,
		"destination": destination,
		"archive":     archive,
		"status":      a.todoStatusPayload(),
	}, nil
}
