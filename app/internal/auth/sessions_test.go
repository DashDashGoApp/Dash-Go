package auth

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestSessionCapEvictsOldestActiveToken(t *testing.T) {
	s := NewService(ServiceConfig{})
	now := time.Now().Add(-time.Hour)
	for i := 0; i < MaxSessionTokens+1; i++ {
		token := fmt.Sprintf("session-%02d", i)
		s.sessions[token] = sessionMeta{IssuedAt: now.Add(time.Duration(i) * time.Second), Timeout: DefaultTimeout}
	}
	s.mu.Lock()
	s.capSessionsLocked()
	s.mu.Unlock()
	if got := len(s.sessions); got != MaxSessionTokens {
		t.Fatalf("sessions=%d want %d", got, MaxSessionTokens)
	}
	if _, kept := s.sessions["session-00"]; kept {
		t.Fatal("oldest active session was retained")
	}
	if _, kept := s.sessions[fmt.Sprintf("session-%02d", MaxSessionTokens)]; !kept {
		t.Fatal("newest active session was evicted")
	}
}

func TestOneShotCapEvictsOldestUnconsumedToken(t *testing.T) {
	s := NewService(ServiceConfig{})
	now := time.Now().Add(-time.Minute)
	for i := 0; i < MaxOneShotTokens+1; i++ {
		token := fmt.Sprintf("oneshot-%02d", i)
		s.oneShots[token] = oneShotMeta{Path: "/api/test", Exp: time.Now().Add(time.Hour), IssuedAt: now.Add(time.Duration(i) * time.Second)}
	}
	s.mu.Lock()
	s.capOneShotsLocked()
	s.mu.Unlock()
	if got := len(s.oneShots); got != MaxOneShotTokens {
		t.Fatalf("one-shots=%d want %d", got, MaxOneShotTokens)
	}
	if _, kept := s.oneShots["oneshot-00"]; kept {
		t.Fatal("oldest one-shot was retained")
	}
	if _, kept := s.oneShots[fmt.Sprintf("oneshot-%02d", MaxOneShotTokens)]; !kept {
		t.Fatal("newest one-shot was evicted")
	}
}

func TestCredentialCapsPruneExpiredBeforeEviction(t *testing.T) {
	s := NewService(ServiceConfig{})
	now := time.Now()
	expired := now.Add(-time.Second)
	s.sessions["expired"] = sessionMeta{IssuedAt: now.Add(-time.Hour), Exp: &expired}
	s.oneShots["expired"] = oneShotMeta{Path: "/api/test", IssuedAt: now.Add(-time.Hour), Exp: expired}
	for i := 0; i < MaxSessionTokens; i++ {
		s.sessions[fmt.Sprintf("session-%02d", i)] = sessionMeta{IssuedAt: now.Add(time.Duration(i) * time.Second)}
	}
	for i := 0; i < MaxOneShotTokens; i++ {
		s.oneShots[fmt.Sprintf("oneshot-%02d", i)] = oneShotMeta{Path: "/api/test", Exp: now.Add(time.Hour), IssuedAt: now.Add(time.Duration(i) * time.Second)}
	}
	s.mu.Lock()
	s.pruneLocked(now)
	s.capSessionsLocked()
	s.capOneShotsLocked()
	s.mu.Unlock()
	if _, found := s.sessions["expired"]; found {
		t.Fatal("expired session was not pruned")
	}
	if _, found := s.oneShots["expired"]; found {
		t.Fatal("expired one-shot was not pruned")
	}
	if got := len(s.sessions); got != MaxSessionTokens {
		t.Fatalf("sessions=%d want %d", got, MaxSessionTokens)
	}
	if got := len(s.oneShots); got != MaxOneShotTokens {
		t.Fatalf("one-shots=%d want %d", got, MaxOneShotTokens)
	}
}

func TestRemovePINClearsSessionsAndFailures(t *testing.T) {
	dir := t.TempDir()
	s := NewService(ServiceConfig{EnvPath: dir + "/dashboard-control.env"})
	if _, err := s.SetPIN("2468", "60"); err != nil {
		t.Fatal(err)
	}
	token := s.IssueToken()
	s.RecordPINFailure()
	s.RemovePIN()
	if !s.TokenOK(token) {
		t.Fatal("PIN-off mode should accept a local request")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.sessions) != 0 || len(s.failTimes) != 0 {
		t.Fatalf("remove PIN retained runtime state: sessions=%d failures=%d", len(s.sessions), len(s.failTimes))
	}
}

func TestServicePINConfigAndLockoutRemainBounded(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/dashboard-control.env"
	s := NewService(ServiceConfig{EnvPath: path})
	if _, err := s.SetPIN("2468", "60"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("PIN env mode=%#o want 0600", info.Mode().Perm())
	}
	config := s.Config()
	if !config.Enabled || config.Timeout != "60" || config.Label != "1 minute" {
		t.Fatalf("unexpected PIN config: %#v", config)
	}
	for range MaxPINFailures {
		s.RecordPINFailure()
	}
	if remaining := s.PINLockoutRemaining(); remaining <= 0 || remaining > int(PINFailureWindow.Seconds()) {
		t.Fatalf("lockout remaining=%d", remaining)
	}
	s.ClearPINFailures()
	if remaining := s.PINLockoutRemaining(); remaining != 0 {
		t.Fatalf("lockout persisted after clear: %d", remaining)
	}
}
