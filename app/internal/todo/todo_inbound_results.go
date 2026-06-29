package todo

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// syncTodoListIDsNow executes the one coordinator-owned Graph pass. The global
// cloud lane keeps Graph calls bounded; per-list locks protect only the brief
// local read-modify-write phases and are never held across network waits.
func (a *Service) syncTodoListIDsNow(ctx context.Context, requested []string) (todoSyncResult, error) {
	result := todoSyncResult{OK: true, Lists: []todoSyncListResult{}}
	listIDs := todoUniqueMicrosoftListIDs(a, requested)
	if len(listIDs) == 0 {
		result.Skipped = true
		result.Reason = "no Microsoft list is ready to sync"
		return result, nil
	}
	a.todoCloudMu.Lock()
	defer a.todoCloudMu.Unlock()
	successes := 0
	var firstFailure error
	for _, listID := range listIDs {
		item := todoSyncListResult{ListID: listID}
		if info, ok := a.todoListInfoByID(listID); ok {
			item.Title = info.DisplayName
		}
		settled, queueErr := a.todoFlushPending(ctx, listID)
		if queueErr != nil {
			if throttled := (*todoThrottleError)(nil); errors.As(queueErr, &throttled) {
				todoApplyGraphFailure(&item, "queue-drain", queueErr)
				result.Lists = append(result.Lists, item)
				return result, queueErr
			}
			queueFailure := todoGraphFailureFromError("queue-drain", listID, "Dash-Go change needs attention", queueErr)
			item.QueueError = queueFailure.Error()
			if firstFailure == nil {
				firstFailure = queueFailure
			}
		}
		pulled, deltaErr := a.syncTodoListDeltaWithProtected(ctx, listID, settled)
		if pulled.Title == "" {
			pulled.Title = item.Title
		}
		if pulled.Title == "" {
			pulled.Title = listID
		}
		pulled.QueueError = item.QueueError
		pulled.QueueBlocked = todoBlockedPendingCount(a.readTodoListCache(listID))
		if deltaErr != nil {
			result.Lists = append(result.Lists, pulled)
			if throttled := (*todoThrottleError)(nil); errors.As(deltaErr, &throttled) {
				return result, deltaErr
			}
			if firstFailure == nil {
				firstFailure = deltaErr
			}
			continue
		}
		successes++
		result.Lists = append(result.Lists, pulled)
		result.LastSyncAt = max(result.LastSyncAt, pulled.LastSyncAt)
	}
	slices.SortStableFunc(result.Lists, func(left, right todoSyncListResult) int { return compareText(left.Title, right.Title) })
	for _, item := range result.Lists {
		if item.Error != "" || item.QueueError != "" || item.QueueBlocked > 0 {
			result.Partial = true
			break
		}
	}
	if successes == 0 && firstFailure != nil {
		return result, firstFailure
	}
	return result, nil
}

func todoInboundResultError(result todoSyncResult, runErr error) error {
	if runErr != nil {
		return runErr
	}
	if !result.Partial {
		return nil
	}
	for _, item := range result.Lists {
		if item.Error != "" {
			return &todoPartialSyncError{Message: item.Title + ": " + item.Error}
		}
		if item.QueueError != "" {
			return &todoPartialSyncError{Message: item.Title + ": " + item.QueueError}
		}
		if item.QueueBlocked > 0 {
			return &todoPartialSyncError{Message: item.Title + ": " + fmt.Sprintf("%d Dash-Go change(s) need a decision", item.QueueBlocked)}
		}
	}
	return &todoPartialSyncError{}
}

func todoSyncResultText(result todoSyncResult) string {
	if result.Skipped {
		return result.Reason
	}
	if result.AlreadyRunning {
		return "Microsoft To Do sync is already running"
	}
	parts := make([]string, 0, len(result.Lists))
	for _, item := range result.Lists {
		title := item.Title
		if title == "" {
			title = item.ListID
		}
		if item.Error != "" {
			parts = append(parts, title+": "+item.Error)
			continue
		}
		part := fmt.Sprintf("%s: %d added, %d updated, %d removed", title, item.Added, item.Updated, item.Removed)
		if item.QueueError != "" {
			part += " · " + item.QueueError
		}
		if item.QueueBlocked > 0 {
			part += fmt.Sprintf(" · %d blocked Dash-Go change(s)", item.QueueBlocked)
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		return "Microsoft To Do is current"
	}
	return strings.Join(parts, " · ")
}
