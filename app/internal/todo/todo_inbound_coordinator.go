package todo

import (
	"context"
	"errors"
	"time"
)

// todoBeginInboundRun reserves the one server-owned cloud reconciliation lane.
// If work is already running, callers add one coalesced follow-up instead of
// dropping a cadence tick or opening a second Graph request stack.
func (a *Service) todoBeginInboundRun(listIDs []string, reason string) ([]string, bool, error) {
	unique := todoUniqueMicrosoftListIDs(a, listIDs)
	if !a.todoCloudSyncEnabled() {
		return unique, false, nil
	}
	a.todoInboundMu.Lock()
	now := time.Now()
	if a.todoInboundBackoffUntil.After(now) {
		until := a.todoInboundBackoffUntil
		a.todoInboundMu.Unlock()
		return unique, false, &todoInboundBackoffError{Until: until}
	}
	if a.todoInboundRunning {
		a.todoQueueInboundLocked(unique, reason, now)
		status := a.todoInboundSyncStatusLocked(now)
		a.todoInboundMu.Unlock()
		a.todoEmit(map[string]any{"type": "sync.state", "running": true, "queued": true, "inboundSync": status})
		return unique, false, nil
	}
	if a.todoInboundQueued {
		for listID := range a.todoInboundQueuedLists {
			unique = append(unique, listID)
		}
		a.todoInboundQueued = false
		a.todoInboundQueuedAt = time.Time{}
		a.todoInboundQueuedLists = map[string]bool{}
		a.todoInboundCoalesced = 0
		unique = todoUniqueMicrosoftListIDs(a, unique)
	}
	a.todoInboundRunning = true
	a.todoInboundLastQueueWait = 0
	status := a.todoInboundSyncStatusLocked(now)
	a.todoInboundMu.Unlock()
	a.todoEmit(map[string]any{"type": "sync.state", "running": true, "queued": false, "reason": reason, "inboundSync": status})
	return unique, true, nil
}

func (a *Service) todoQueueInboundLocked(listIDs []string, reason string, now time.Time) {
	if a.todoInboundQueuedLists == nil {
		a.todoInboundQueuedLists = map[string]bool{}
	}
	for _, listID := range listIDs {
		if listID != "" {
			a.todoInboundQueuedLists[listID] = true
		}
	}
	if !a.todoInboundQueued {
		a.todoInboundQueued = true
		a.todoInboundQueuedAt = now
	}
	// A count is diagnostics only. It is bounded by actual incoming work and
	// reset when the coalesced pass begins, so it cannot become a task history.
	a.todoInboundCoalesced++
	_ = reason
}

func (a *Service) todoTakeQueuedInboundLocked(now time.Time, allow bool) []string {
	if !allow || !a.todoInboundQueued {
		return nil
	}
	ids := make([]string, 0, len(a.todoInboundQueuedLists))
	for id := range a.todoInboundQueuedLists {
		ids = append(ids, id)
	}
	queuedAt := a.todoInboundQueuedAt
	a.todoInboundQueued = false
	a.todoInboundQueuedAt = time.Time{}
	a.todoInboundQueuedLists = map[string]bool{}
	if !queuedAt.IsZero() {
		a.todoInboundLastQueueWait = time.Since(queuedAt).Milliseconds()
	}
	a.todoInboundCoalesced = 0
	a.todoInboundRunning = true
	return todoUniqueMicrosoftListIDs(a, ids)
}

// todoFinishInboundRun updates durable runtime diagnostics and atomically hands
// one coalesced follow-up to the caller. Error/backoff leaves no hidden loop;
// the normal timer or an explicit Sync now may retry after the reported pause.
func (a *Service) todoFinishInboundRun(err error, started time.Time) []string {
	now := time.Now()
	a.todoInboundMu.Lock()
	a.todoInboundRunning = false
	a.todoInboundLastDuration = now.Sub(started).Milliseconds()
	partial, throttled := todoInboundFinishKind(err)
	if partial {
		a.todoInboundFailures = 0
		a.todoInboundBackoffUntil = time.Time{}
		a.todoInboundLastError = err.Error()
		a.todoInboundLastAt = now.UnixMilli()
	} else if err == nil {
		a.todoInboundFailures = 0
		a.todoInboundBackoffUntil = time.Time{}
		a.todoInboundLastError = ""
		a.todoInboundLastAt = now.UnixMilli()
	} else {
		a.todoInboundFailures++
		a.todoInboundLastError = err.Error()
		if throttled != nil {
			delay := throttled.RetryAfter
			if delay <= 0 {
				delay = time.Minute
			}
			a.todoInboundBackoffUntil = now.Add(delay)
		} else {
			a.todoInboundBackoffUntil = now.Add(todoInboundBackoffForFailure(a.todoInboundFailures))
		}
	}
	allowFollowUp := err == nil || partial
	next := a.todoTakeQueuedInboundLocked(now, allowFollowUp)
	status := a.todoInboundSyncStatusLocked(now)
	a.todoInboundMu.Unlock()
	payload := map[string]any{"type": "sync.state", "running": status.Running, "queued": status.Queued, "inboundSync": status}
	if err != nil {
		payload["lastError"] = status.LastError
	}
	a.todoEmit(payload)
	return next
}

func (a *Service) todoLaunchInboundRun(listIDs []string, reason string) {
	if len(listIDs) == 0 {
		return
	}
	go func() {
		started := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
		defer cancel()
		result, runErr := a.syncTodoListIDsNow(ctx, listIDs)
		next := a.todoFinishInboundRun(todoInboundResultError(result, runErr), started)
		if len(next) > 0 {
			a.todoLaunchInboundRun(next, "coalesced")
		}
	}()
	_ = reason
}

func (a *Service) todoStartInboundSync(reason string) {
	a.todoStartInboundSyncForLists(a.todoInboundMicrosoftListIDs(), reason)
}

// todoStartInboundSyncForList is intentionally explicit. Cache GET handlers
// call no Graph endpoint; an app open or saved local edit requests a bounded
// background pass through this coordinator instead.
func (a *Service) todoStartInboundSyncForList(listID string) {
	a.todoStartInboundSyncForLists([]string{listID}, "list")
}

func (a *Service) todoStartInboundSyncForLists(listIDs []string, reason string) {
	unique := todoUniqueMicrosoftListIDs(a, listIDs)
	if len(unique) == 0 {
		return
	}
	prepared, started, err := a.todoBeginInboundRun(unique, reason)
	if err != nil || !started {
		return
	}
	a.todoLaunchInboundRun(prepared, reason)
}

// todoRunInboundSync is the authenticated, user-visible Sync now flow. It
// returns an already-running/queued result instead of waiting behind network
// work on the kiosk request path. The current run will make exactly one
// coalesced follow-up when it succeeds or partially succeeds.
func (a *Service) todoRunInboundSync(ctx context.Context) (todoSyncResult, error) {
	return a.todoRunInboundSyncForLists(ctx, a.todoInboundMicrosoftListIDs(), "manual")
}

func (a *Service) todoRunInboundSyncForLists(ctx context.Context, listIDs []string, reason string) (todoSyncResult, error) {
	if !a.todoCloudSyncEnabled() {
		return todoSyncResult{OK: true, Skipped: true, Reason: "Microsoft To Do is not linked", Lists: []todoSyncListResult{}}, nil
	}
	unique := todoUniqueMicrosoftListIDs(a, listIDs)
	if len(unique) == 0 {
		return todoSyncResult{OK: true, Skipped: true, Reason: "no Microsoft list is ready to sync", Lists: []todoSyncListResult{}}, nil
	}
	prepared, started, err := a.todoBeginInboundRun(unique, reason)
	if err != nil {
		return todoSyncResult{OK: false, Lists: []todoSyncListResult{}}, err
	}
	if !started {
		a.todoInboundMu.Lock()
		queued := a.todoInboundQueued
		a.todoInboundMu.Unlock()
		return todoSyncResult{OK: true, AlreadyRunning: true, Queued: queued, Lists: []todoSyncListResult{}}, nil
	}
	startedAt := time.Now()
	result, runErr := a.syncTodoListIDsNow(ctx, prepared)
	next := a.todoFinishInboundRun(todoInboundResultError(result, runErr), startedAt)
	if len(next) > 0 {
		a.todoLaunchInboundRun(next, "coalesced")
	}
	if runErr != nil {
		var throttled *todoThrottleError
		if len(result.Lists) > 0 && !errors.As(runErr, &throttled) {
			result.OK = false
			result.Partial = true
			return result, nil
		}
	}
	return result, runErr
}
