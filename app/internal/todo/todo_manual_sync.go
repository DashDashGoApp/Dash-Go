package todo

import (
	"strings"
	"time"
)

// todoManualListSyncCooldown bounds a family-facing app Sync now request to one
// request per active Microsoft list every 25 seconds. It is runtime-only state:
// a restart never creates a durable user preference or retry history.
const todoManualListSyncCooldown = 25 * time.Second

// todoManualListSyncStatus is safe for the browser. It intentionally contains
// no task data, credentials, Graph URL, or Graph request diagnostics.
type todoManualListSyncStatus struct {
	Available       bool   `json:"available"`
	Running         bool   `json:"running,omitempty"`
	Queued          bool   `json:"queued,omitempty"`
	CooldownUntil   int64  `json:"cooldownUntil,omitempty"`
	CooldownSeconds int    `json:"cooldownSeconds,omitempty"`
	BackoffUntil    int64  `json:"backoffUntil,omitempty"`
	BackoffSeconds  int    `json:"backoffSeconds,omitempty"`
	Reason          string `json:"reason,omitempty"`
}

// todoManualListSyncResult distinguishes an accepted direct app request from a
// safe no-op caused by a cooldown, an in-progress pass, or provider backoff.
type todoManualListSyncResult struct {
	Accepted   bool                     `json:"accepted"`
	Started    bool                     `json:"started,omitempty"`
	Queued     bool                     `json:"queued,omitempty"`
	Reason     string                   `json:"reason,omitempty"`
	ManualSync todoManualListSyncStatus `json:"manualSync"`
}

func todoSecondsUntil(now, until time.Time) int {
	if !until.After(now) {
		return 0
	}
	seconds := int(until.Sub(now).Seconds())
	if seconds < 1 {
		return 1
	}
	return seconds
}

func (a *Service) todoManualListSyncStatus(listID string) todoManualListSyncStatus {
	listID = strings.TrimSpace(listID)
	cloudEnabled := listID != "" && a.todoListCloudSyncEnabled(listID)
	a.todoInboundMu.Lock()
	defer a.todoInboundMu.Unlock()
	return a.todoManualListSyncStatusLocked(listID, cloudEnabled, time.Now())
}

func (a *Service) todoManualListSyncStatusLocked(listID string, cloudEnabled bool, now time.Time) todoManualListSyncStatus {
	if !cloudEnabled {
		return todoManualListSyncStatus{Reason: "Microsoft sync is not active for this list."}
	}
	status := todoManualListSyncStatus{
		Available: true,
		Running:   a.todoInboundRunning,
		Queued:    a.todoInboundQueued,
	}
	if a.todoInboundBackoffUntil.After(now) {
		status.Available = false
		status.BackoffUntil = a.todoInboundBackoffUntil.UnixMilli()
		status.BackoffSeconds = todoSecondsUntil(now, a.todoInboundBackoffUntil)
		status.Reason = "Microsoft asked Dash-Go to wait before the next check."
		return status
	}
	if a.todoManualSyncUntil == nil {
		a.todoManualSyncUntil = map[string]time.Time{}
	}
	until := a.todoManualSyncUntil[listID]
	if !until.After(now) {
		delete(a.todoManualSyncUntil, listID)
		until = time.Time{}
	}
	if !until.IsZero() {
		status.Available = false
		status.CooldownUntil = until.UnixMilli()
		status.CooldownSeconds = todoSecondsUntil(now, until)
		if status.Running {
			status.Reason = "Checking Microsoft changes."
		} else if status.Queued {
			status.Reason = "Microsoft sync is queued."
		} else {
			status.Reason = "Sync was requested recently."
		}
		return status
	}
	if status.Running {
		status.Available = false
		status.Reason = "Checking Microsoft changes."
		return status
	}
	if status.Queued {
		status.Available = false
		status.Reason = "Microsoft sync is queued."
		return status
	}
	status.Reason = "Ready to check Microsoft now."
	return status
}

// todoManualListSyncStatuses returns only eligible household pull targets.
// It keeps normal status responses bounded and avoids advertising a Sync now
// button for a local list or an untouched discovered Microsoft list.
func (a *Service) todoManualListSyncStatuses() map[string]todoManualListSyncStatus {
	out := map[string]todoManualListSyncStatus{}
	for _, listID := range a.todoInboundMicrosoftListIDs() {
		out[listID] = a.todoManualListSyncStatus(listID)
	}
	return out
}

// todoRequestManualListSync accepts at most one direct request per list per
// cooldown window. The existing coordinator remains the only code that starts
// Graph work, so this action never creates a parallel cloud path.
func (a *Service) todoRequestManualListSync(listID string) todoManualListSyncResult {
	listID = strings.TrimSpace(listID)
	cloudEnabled := listID != "" && a.todoListCloudSyncEnabled(listID)
	if !cloudEnabled {
		return todoManualListSyncResult{
			Reason:     "Microsoft sync is not active for this list.",
			ManualSync: a.todoManualListSyncStatus(listID),
		}
	}

	now := time.Now()
	a.todoInboundMu.Lock()
	if a.todoInboundBackoffUntil.After(now) {
		status := a.todoManualListSyncStatusLocked(listID, true, now)
		a.todoInboundMu.Unlock()
		return todoManualListSyncResult{Reason: status.Reason, ManualSync: status, Queued: status.Queued}
	}
	if a.todoManualSyncUntil == nil {
		a.todoManualSyncUntil = map[string]time.Time{}
	}
	if until := a.todoManualSyncUntil[listID]; until.After(now) {
		status := a.todoManualListSyncStatusLocked(listID, true, now)
		a.todoInboundMu.Unlock()
		return todoManualListSyncResult{Reason: status.Reason, ManualSync: status, Queued: status.Queued}
	}
	until := now.Add(todoManualListSyncCooldown)
	a.todoManualSyncUntil[listID] = until
	a.todoInboundMu.Unlock()

	prepared, started, err := a.todoBeginInboundRun([]string{listID}, "app-sync-now")
	if err != nil {
		// A run finishing between the reservation and coordinator admission can
		// set provider backoff. Do not consume the family-facing cooldown when
		// no manual request was admitted.
		a.todoInboundMu.Lock()
		if a.todoManualSyncUntil != nil && a.todoManualSyncUntil[listID].Equal(until) {
			delete(a.todoManualSyncUntil, listID)
		}
		status := a.todoManualListSyncStatusLocked(listID, true, time.Now())
		a.todoInboundMu.Unlock()
		return todoManualListSyncResult{Reason: status.Reason, ManualSync: status, Queued: status.Queued}
	}
	if started {
		a.todoLaunchInboundRun(prepared, "app-sync-now")
	}
	status := a.todoManualListSyncStatus(listID)
	return todoManualListSyncResult{
		Accepted:   true,
		Started:    started,
		Queued:     !started,
		Reason:     status.Reason,
		ManualSync: status,
	}
}
