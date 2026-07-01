package auth

import (
	"encoding/json"
	"os"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
)

const (
	MaxPINFailures           = 8
	InitialPINLockout        = 60 * time.Second
	MaxPINLockout            = 30 * time.Minute
	MaxPersistentPINFailures = 32
	PINFailureWindow         = InitialPINLockout // compatibility name for callers/tests
	pinLockoutStateSchema    = 1
)

type pinLockoutState struct {
	Schema      int   `json:"schema"`
	Failures    int   `json:"failures"`
	LockedUntil int64 `json:"lockedUntil"`
}

func (s *Service) loadPINLockout() {
	if s.lockoutPath == "" {
		return
	}
	body, err := os.ReadFile(s.lockoutPath)
	if err != nil {
		return
	}
	var state pinLockoutState
	if json.Unmarshal(body, &state) != nil || state.Schema != pinLockoutStateSchema || state.Failures < 0 || state.LockedUntil < 0 {
		return
	}
	if state.Failures > MaxPersistentPINFailures {
		state.Failures = MaxPersistentPINFailures
	}
	s.mu.Lock()
	s.lockout = state
	s.mu.Unlock()
}

func (s *Service) savePINLockoutLocked() {
	if s.lockoutPath == "" {
		return
	}
	state := s.lockout
	state.Schema = pinLockoutStateSchema
	body, err := json.Marshal(state)
	if err != nil {
		return
	}
	_ = fileio.WriteAtomic(s.lockoutPath, append(body, '\n'), 0600)
}

func pinLockoutDuration(failures int) time.Duration {
	if failures < MaxPINFailures {
		return 0
	}
	steps := failures - MaxPINFailures
	duration := InitialPINLockout
	for steps > 0 && duration < MaxPINLockout {
		duration *= 2
		steps--
	}
	if duration > MaxPINLockout {
		return MaxPINLockout
	}
	return duration
}

func remainingLockout(lockedUntil time.Time, now time.Time) int {
	if !lockedUntil.After(now) {
		return 0
	}
	remaining := lockedUntil.Sub(now)
	return int((remaining + time.Second - 1) / time.Second)
}

func (s *Service) pinLockoutRemainingLocked(now time.Time) int {
	if s.lockout.LockedUntil == 0 {
		return 0
	}
	return remainingLockout(time.Unix(s.lockout.LockedUntil, 0), now)
}

func (s *Service) PINLockoutRemaining() int {
	now := s.nowTime()
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pinLockoutRemainingLocked(now)
}

// RecordPINFailure persists a bounded total attempt count. After the first
// eight failures, every further wrong PIN receives an exponentially longer
// delay (capped at thirty minutes), including after a service restart.
func (s *Service) RecordPINFailure() int {
	now := s.nowTime()
	s.mu.Lock()
	defer s.mu.Unlock()
	if remaining := s.pinLockoutRemainingLocked(now); remaining > 0 {
		return remaining
	}
	if s.lockout.Failures < MaxPersistentPINFailures {
		s.lockout.Failures++
	}
	if duration := pinLockoutDuration(s.lockout.Failures); duration > 0 {
		s.lockout.LockedUntil = now.Add(duration).Unix()
	} else {
		s.lockout.LockedUntil = 0
	}
	s.savePINLockoutLocked()
	return s.pinLockoutRemainingLocked(now)
}

func (s *Service) ClearPINFailures() {
	s.mu.Lock()
	s.lockout = pinLockoutState{}
	path := s.lockoutPath
	s.mu.Unlock()
	if path != "" {
		_ = os.Remove(path)
	}
}
