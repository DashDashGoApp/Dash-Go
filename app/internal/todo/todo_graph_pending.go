package todo

import (
	"context"
	"fmt"
	"net/http"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func (a *Service) todoStartDrain(listID string) {
	if !a.todoListCloudSyncEnabled(listID) {
		return
	}
	// Local mutations are durable before this call. Put outbound draining and
	// inbound reconciliation through the same coalescing coordinator so a rapid
	// series of checkbox taps cannot create competing cloud waits.
	a.todoStartInboundSyncForLists([]string{listID}, "local-write")
}

func todoPendingOpMatches(left, right todoPendingOp) bool {
	return left.Op == right.Op && left.ListID == right.ListID && left.TaskID == right.TaskID && left.Created == right.Created
}

func todoPendingOpIndex(ops []todoPendingOp, target todoPendingOp) int {
	for i := range ops {
		if todoPendingOpMatches(ops[i], target) {
			return i
		}
	}
	return -1
}

func todoActivePendingOpIndex(ops []todoPendingOp) int {
	for i := range ops {
		if !ops[i].Blocked {
			return i
		}
	}
	return -1
}

func todoBlockedPendingCount(cache todoListCache) int {
	count := 0
	for _, op := range cache.PendingOps {
		if op.Blocked {
			count++
		}
	}
	return count
}

func todoBlockedPendingSummary(cache todoListCache) string {
	count := todoBlockedPendingCount(cache)
	if count == 0 {
		return ""
	}
	if count == 1 {
		return "1 Dash-Go change could not be sent to Microsoft"
	}
	return fmt.Sprintf("%d Dash-Go changes could not be sent to Microsoft", count)
}

func todoGraphWriteRequest(ctx context.Context, a *Service, op todoPendingOp, cache todoListCache) (map[string]any, todoGraphResponseMeta, error) {
	switch op.Op {
	case "create":
		var task todoTask
		for _, candidate := range cache.Tasks {
			if candidate.ID == op.TaskID {
				task = candidate
				break
			}
		}
		return a.todoGraphResponse(ctx, http.MethodPost, "/me/todo/lists/"+todoGraphPathID(op.ListID)+"/tasks", todoTaskGraphBody(task, op.Payload))
	case "patch":
		return a.todoGraphResponse(ctx, http.MethodPatch, "/me/todo/lists/"+todoGraphPathID(op.ListID)+"/tasks/"+todoGraphPathID(op.TaskID), todoTaskGraphPatchBody(op.Payload))
	case "delete":
		return a.todoGraphResponse(ctx, http.MethodDelete, "/me/todo/lists/"+todoGraphPathID(op.ListID)+"/tasks/"+todoGraphPathID(op.TaskID), nil)
	default:
		return nil, todoGraphResponseMeta{}, fmt.Errorf("unknown pending operation")
	}
}

func todoMarkTaskSyncFailed(cache *todoListCache, taskID string) {
	for i := range cache.Tasks {
		if cache.Tasks[i].ID == taskID {
			cache.Tasks[i].SyncFailed = true
			cache.Tasks[i].Pending = ""
		}
	}
}

func todoSettlePendingCreate(cache *todoListCache, index int, op todoPendingOp, payload map[string]any, settled map[string]bool) error {
	remoteID := jsonutil.StringValue(payload["id"])
	if remoteID == "" {
		return fmt.Errorf("Microsoft To Do returned no task ID")
	}
	for i := range cache.Tasks {
		if cache.Tasks[i].ID == op.TaskID {
			cache.Tasks[i].ID = remoteID
			cache.Tasks[i].Pending = ""
			cache.Tasks[i].SyncFailed = false
			cache.Tasks[i].CloudIgnored = false
		}
	}
	for i := range cache.PendingOps {
		if i != index && cache.PendingOps[i].TaskID == op.TaskID {
			cache.PendingOps[i].TaskID = remoteID
		}
	}
	settled[remoteID] = true
	return nil
}

func todoSettlePendingExisting(cache *todoListCache, op todoPendingOp, settled map[string]bool) {
	for i := range cache.Tasks {
		if cache.Tasks[i].ID == op.TaskID {
			cache.Tasks[i].Pending = ""
			cache.Tasks[i].SyncFailed = false
			cache.Tasks[i].CloudIgnored = false
		}
	}
	if op.Op != "delete" {
		settled[op.TaskID] = true
	}
}

// todoFlushPending sends one durable local operation at a time. After three
// failed attempts a write is blocked—not discarded—and inbound delta processing
// may continue. A historical bad Dash-Go write must never freeze phone->Dash-Go
// reconciliation for an entire list.
func (a *Service) todoFlushPending(ctx context.Context, listID string) (map[string]bool, error) {
	settled := map[string]bool{}
	if !a.todoListCloudSyncEnabled(listID) {
		return settled, nil
	}
	for {
		// Do not hold a list mutation lock during a Graph request. The settlement
		// below re-reads the latest cache under lock, so checkbox taps remain fast.
		cache := a.readTodoListCache(listID)
		activeIndex := todoActivePendingOpIndex(cache.PendingOps)
		if activeIndex < 0 {
			return settled, nil
		}
		op := cache.PendingOps[activeIndex]
		payload, meta, err := todoGraphWriteRequest(ctx, a, op, cache)

		unlock := a.todoListLock(listID)
		latest := a.readTodoListCache(listID)
		index := todoPendingOpIndex(latest.PendingOps, op)
		if index < 0 {
			unlock()
			// The cloud lane serializes requests. A missing queued op means another
			// local mutation superseded it; never replay a success and risk a duplicate.
			return settled, fmt.Errorf("Microsoft To Do queue changed before an operation could be settled")
		}
		if err != nil || meta.Status < 200 || meta.Status >= 300 {
			failure := todoGraphFailureFor("queue-drain", listID, "Microsoft To Do change could not be sent", payload, meta, err)
			latest.PendingOps[index].Attempts++
			latest.PendingOps[index].LastError = failure.Error()
			latest.PendingOps[index].LastStatus = meta.Status
			latest.LastError = failure.Error()
			if latest.PendingOps[index].Attempts >= 3 {
				latest.PendingOps[index].Blocked = true
				todoMarkTaskSyncFailed(&latest, op.TaskID)
				latest.LastError = todoBlockedPendingSummary(latest)
			}
			writeErr := a.writeTodoListCache(latest)
			unlock()
			if writeErr != nil {
				return settled, writeErr
			}
			a.todoEmit(map[string]any{"type": "sync.state", "listId": listID, "lastError": latest.LastError, "blockedWrites": todoBlockedPendingCount(latest)})
			if todoGraphIsThrottle(meta) {
				return settled, &todoThrottleError{RetryAfter: meta.RetryAfter, Err: failure}
			}
			if latest.PendingOps[index].Blocked {
				continue
			}
			return settled, failure
		}

		if op.Op == "create" {
			err = todoSettlePendingCreate(&latest, index, op, payload, settled)
		} else {
			todoSettlePendingExisting(&latest, op, settled)
		}
		if err == nil {
			latest.PendingOps = append(latest.PendingOps[:index], latest.PendingOps[index+1:]...)
			latest.LastError = todoBlockedPendingSummary(latest)
			latest.LastSyncAt = todoNowMillis()
			err = a.writeTodoListCache(latest)
		}
		unlock()
		if err != nil {
			return settled, err
		}
		a.todoEmit(map[string]any{"type": "sync.state", "listId": listID, "lastSyncAt": latest.LastSyncAt, "blockedWrites": todoBlockedPendingCount(latest)})
	}
}
