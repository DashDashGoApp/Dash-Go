package todo

import (
	"fmt"
	"strings"
)

func (a *Service) todoResolveBlockedPendingOps(listID, action string) (todoListCache, int, error) {
	listID = strings.TrimSpace(listID)
	action = strings.ToLower(strings.TrimSpace(action))
	if listID == "" || !a.todoListCloudSyncEnabled(listID) {
		return todoListCache{}, 0, fmt.Errorf("Microsoft list is not available")
	}
	if action != "retry" && action != "keep-local" && action != "discard" {
		return todoListCache{}, 0, fmt.Errorf("action must be retry, keep-local, or discard")
	}
	unlock := a.todoListLock(listID)
	cache := a.readTodoListCache(listID)
	blockedTaskIDs := map[string]bool{}
	blockedCreateTaskIDs := map[string]bool{}
	blockedCount := 0
	for _, op := range cache.PendingOps {
		if !op.Blocked {
			continue
		}
		blockedCount++
		if op.TaskID != "" {
			blockedTaskIDs[op.TaskID] = true
			if op.Op == "create" {
				blockedCreateTaskIDs[op.TaskID] = true
			}
		}
	}
	if blockedCount == 0 {
		unlock()
		return cache, 0, nil
	}
	resolved := blockedCount
	switch action {
	case "retry":
		for i := range cache.PendingOps {
			if cache.PendingOps[i].Blocked {
				cache.PendingOps[i].Blocked = false
				cache.PendingOps[i].Attempts = 0
				cache.PendingOps[i].LastError = ""
				cache.PendingOps[i].LastStatus = 0
			}
		}
		for i := range cache.Tasks {
			if blockedTaskIDs[cache.Tasks[i].ID] {
				cache.Tasks[i].Pending = "retry"
				cache.Tasks[i].SyncFailed = false
				cache.Tasks[i].CloudIgnored = false
			}
		}
	case "keep-local":
		kept := make([]todoPendingOp, 0, len(cache.PendingOps))
		for _, op := range cache.PendingOps {
			if op.Blocked {
				continue
			}
			kept = append(kept, op)
		}
		cache.PendingOps = kept
		for i := range cache.Tasks {
			if blockedTaskIDs[cache.Tasks[i].ID] {
				cache.Tasks[i].Pending = ""
				cache.Tasks[i].SyncFailed = false
				cache.Tasks[i].CloudIgnored = true
			}
		}
	case "discard":
		keptOps := make([]todoPendingOp, 0, len(cache.PendingOps))
		for _, op := range cache.PendingOps {
			if op.Blocked {
				continue
			}
			keptOps = append(keptOps, op)
		}
		cache.PendingOps = keptOps
		keptTasks := make([]todoTask, 0, len(cache.Tasks))
		for _, task := range cache.Tasks {
			if blockedCreateTaskIDs[task.ID] {
				continue
			}
			if blockedTaskIDs[task.ID] {
				task.Pending = ""
				task.SyncFailed = false
				task.CloudIgnored = false
			}
			keptTasks = append(keptTasks, task)
		}
		cache.Tasks = keptTasks
		// A full baseline safely restores a remote patch/delete after the user
		// discards its failed local operation; a failed create remains absent.
		cache.DeltaLink = ""
	}
	cache.LastError = todoBlockedPendingSummary(cache)
	if err := a.writeTodoListCache(cache); err != nil {
		unlock()
		return cache, 0, err
	}
	unlock()
	a.todoEmit(map[string]any{"type": "sync.state", "listId": listID, "blockedWrites": todoBlockedPendingCount(cache), "lastError": cache.LastError})
	if action == "retry" {
		a.todoStartDrain(listID)
	} else if action == "discard" {
		a.todoStartInboundSyncForList(listID)
	}
	return cache, resolved, nil
}
