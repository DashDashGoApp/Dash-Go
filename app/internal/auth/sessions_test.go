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
	if _, err := s.RemovePIN(); err != nil {
		t.Fatal(err)
	}
	if !s.TokenOK(token) {
		t.Fatal("PIN-off mode should accept a local request")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.sessions) != 0 || s.lockout.Failures != 0 || s.lockout.LockedUntil != 0 {
		t.Fatalf("remove PIN retained runtime state: sessions=%d lockout=%#v", len(s.sessions), s.lockout)
	}
}

func TestServicePINConfigAndEscalatingLockoutPersist(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/dashboard-control.env"
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	serviceConfig := ServiceConfig{EnvPath: path, Now: func() time.Time { return now }}
	s := NewService(serviceConfig)
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
	if !config.Enabled || !config.Available || config.Timeout != "60" || config.Label != "1 minute" {
		t.Fatalf("unexpected PIN config: %#v", config)
	}
	payload := config.Payload()
	for _, secret := range []string{"hash", "salt", "iterations"} {
		if _, found := payload[secret]; found {
			t.Fatalf("browser PIN payload leaked %s", secret)
		}
	}
	for range MaxPINFailures {
		s.RecordPINFailure()
	}
	if remaining := s.PINLockoutRemaining(); remaining != int(InitialPINLockout.Seconds()) {
		t.Fatalf("first lockout remaining=%d", remaining)
	}
	if _, err := os.Stat(s.lockoutPath); err != nil {
		t.Fatalf("lockout was not persisted: %v", err)
	}
	restarted := NewService(serviceConfig)
	if remaining := restarted.PINLockoutRemaining(); remaining != int(InitialPINLockout.Seconds()) {
		t.Fatalf("restart lost lockout: %d", remaining)
	}
	now = now.Add(InitialPINLockout + time.Second)
	if remaining := restarted.RecordPINFailure(); remaining != int((2 * InitialPINLockout).Seconds()) {
		t.Fatalf("second lockout remaining=%d", remaining)
	}
	restarted.ClearPINFailures()
	if remaining := restarted.PINLockoutRemaining(); remaining != 0 {
		t.Fatalf("lockout persisted after clear: %d", remaining)
	}
}

func TestPINConfigReadFailureFailsClosedAndLocalRecoveryClearsPIN(t *testing.T) {
	dir := t.TempDir()
	bad := NewService(ServiceConfig{EnvPath: dir}) // a directory cannot be read as an env file
	if cfg := bad.Config(); cfg.Available || !cfg.Enabled {
		t.Fatalf("unreadable config must fail closed: %#v", cfg)
	}
	if bad.TokenOK("anything") || bad.VerifyPIN("2468") {
		t.Fatal("unreadable PIN configuration allowed authentication")
	}
	if _, err := bad.SetPIN("2468", "60"); err == nil {
		t.Fatal("unreadable PIN configuration accepted a write")
	}

	path := dir + "/dashboard-control.env"
	if _, err := NewService(ServiceConfig{EnvPath: path}).SetPIN("2468", "60"); err != nil {
		t.Fatal(err)
	}
	flag := dir + "/.dashboard-control-pin-reset"
	if err := os.WriteFile(flag, []byte("local recovery\n"), 0600); err != nil {
		t.Fatal(err)
	}
	recovered := NewService(ServiceConfig{EnvPath: path})
	if cfg := recovered.Config(); !cfg.Available || cfg.Enabled {
		t.Fatalf("local recovery did not disable the PIN: %#v", cfg)
	}
	if _, err := os.Stat(flag); !os.IsNotExist(err) {
		t.Fatalf("local recovery flag remained: %v", err)
	}
}

func TestVerifyPINDoesNotTreatDisabledProtectionAsCredential(t *testing.T) {
	s := NewService(ServiceConfig{EnvPath: t.TempDir() + "/dashboard-control.env"})
	if s.VerifyPIN("2468") {
		t.Fatal("disabled PIN protection verified an arbitrary PIN")
	}
}

func TestEveryOpenSessionExpiresServerSideAndHeartbeatRefreshes(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	s := NewService(ServiceConfig{EnvPath: dir + "/dashboard-control.env", Now: func() time.Time { return now }})
	if _, err := s.SetPIN("2468", "every_open"); err != nil {
		t.Fatal(err)
	}
	token := s.IssueToken()
	if token == "" || !s.TokenOK(token) {
		t.Fatal("every-open token was not issued")
	}
	now = now.Add(60 * time.Second)
	if refreshed := s.RefreshSession(token, "every_open"); refreshed["sessionRefreshed"] != true || refreshed["sessionTtl"] != int(EveryOpenActiveTTL.Seconds()) {
		t.Fatalf("heartbeat refresh=%#v", refreshed)
	}
	now = now.Add(EveryOpenActiveTTL + time.Second)
	if s.TokenOK(token) {
		t.Fatal("every-open token survived past its server-side heartbeat expiry")
	}
}
