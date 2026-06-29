package todo

import (
	"errors"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

const todoInboundSyncMaxBackoff = 15 * time.Minute

type todoThrottleError struct {
	RetryAfter time.Duration
	Err        error
}

func (e *todoThrottleError) Error() string {
	if e == nil || e.Err == nil {
		return "Microsoft To Do asked Dash-Go to slow down"
	}
	return e.Err.Error()
}
func (e *todoThrottleError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type todoInboundBackoffError struct{ Until time.Time }

func (e *todoInboundBackoffError) Error() string {
	if e == nil || e.Until.IsZero() {
		return "Microsoft To Do sync is temporarily paused"
	}
	return "Microsoft To Do sync is temporarily paused until " + e.Until.Local().Format(time.Kitchen)
}

// todoPartialSyncError records an actionable per-list failure without putting
// successful household lists into a global exponential backoff.
type todoPartialSyncError struct{ Message string }

func (e *todoPartialSyncError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "Some Microsoft To Do lists need attention"
	}
	return e.Message
}

func todoInboundSyncMode(_ int) string { return "automatic" }

// todoInboundSyncSeconds is intentionally not user-tunable. Twenty-five seconds
// is the bounded periodic safety net for phone-originated changes while list-open,
// Dash-Go writes, and Sync now remain immediate coordinator requests.
func (a *Service) todoInboundSyncSeconds() int { return todoInboundSyncFixedSeconds }

// todoNormalizeInboundSyncSetting retires the earlier Manual/Fast/Balanced/
// Low-power preference without creating fresh settings on a new local-only
// dashboard. Existing saved values converge to the fixed 25-second cadence
// during startup; the runtime already uses that value even if persistence fails.
func (a *Service) todoNormalizeInboundSyncSetting() error {
	todo := a.todoSettings()
	raw, exists := todo["inboundSyncSeconds"]
	if !exists || jsonutil.Int(raw, -1) == todoInboundSyncFixedSeconds {
		return nil
	}
	_, err := a.writeTodoSettings(func(next map[string]any) {
		next["inboundSyncSeconds"] = todoInboundSyncFixedSeconds
	})
	return err
}

func (a *Service) todoInboundSyncStatus() todoInboundSyncStatus {
	a.todoInboundMu.Lock()
	defer a.todoInboundMu.Unlock()
	return a.todoInboundSyncStatusLocked(time.Now())
}

func (a *Service) todoInboundSyncStatusLocked(now time.Time) todoInboundSyncStatus {
	configured := a.todoInboundSyncSeconds()
	status := todoInboundSyncStatus{
		ConfiguredSeconds: configured,
		EffectiveSeconds:  configured,
		Mode:              todoInboundSyncMode(configured),
		Enabled:           a.todoInboundSyncReady(),
		Running:           a.todoInboundRunning,
		Queued:            a.todoInboundQueued,
		LastSyncAt:        a.todoInboundLastAt,
		LastError:         a.todoInboundStatusLastError(a.todoInboundLastError),
		LastDurationMs:    a.todoInboundLastDuration,
		LastQueueWaitMs:   a.todoInboundLastQueueWait,
		CoalescedRequests: a.todoInboundCoalesced,
	}
	if a.todoInboundQueued && !a.todoInboundQueuedAt.IsZero() {
		status.QueueSeconds = int(now.Sub(a.todoInboundQueuedAt).Seconds())
		if status.QueueSeconds < 1 {
			status.QueueSeconds = 1
		}
	}
	if !a.todoInboundBackoffUntil.IsZero() {
		status.BackoffUntil = a.todoInboundBackoffUntil.UnixMilli()
	}
	if a.todoInboundBackoffUntil.After(now) {
		remaining := int(a.todoInboundBackoffUntil.Sub(now).Seconds())
		if remaining < 1 {
			remaining = 1
		}
		status.BackoffSeconds = remaining
		status.EffectiveSeconds = remaining
	}
	return status
}

func (a *Service) todoNotifyInboundScheduler() {
	if a.todoInboundWake == nil {
		return
	}
	select {
	case a.todoInboundWake <- struct{}{}:
	default:
	}
}

func (a *Service) todoInboundScheduleDelay() time.Duration {
	return time.Duration(todoInboundSyncFixedSeconds) * time.Second
}

func (a *Service) startTodoInboundScheduler() {
	if a.todoInboundWake == nil {
		a.todoInboundWake = make(chan struct{}, 1)
	}
	go func() {
		for {
			timer := time.NewTimer(a.todoInboundScheduleDelay())
			select {
			case <-timer.C:
				if a.todoInboundSyncReady() {
					a.todoStartInboundSync("scheduled")
				}
			case <-a.todoInboundWake:
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
			}
		}
	}()
}

func todoInboundBackoffForFailure(failures int) time.Duration {
	if failures < 1 {
		return 0
	}
	delay := 30 * time.Second
	for i := 1; i < failures && delay < todoInboundSyncMaxBackoff; i++ {
		delay *= 2
	}
	if delay > todoInboundSyncMaxBackoff {
		return todoInboundSyncMaxBackoff
	}
	return delay
}

func todoInboundFinishKind(err error) (partial bool, throttled *todoThrottleError) {
	var partialErr *todoPartialSyncError
	partial = errors.As(err, &partialErr)
	errors.As(err, &throttled)
	return partial, throttled
}
