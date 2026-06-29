package auth

import "time"

const (
	MaxSessionTokens = 32
	MaxOneShotTokens = 64
	MaxPINFailures   = 8
	PINFailureWindow = 60 * time.Second
)

type sessionMeta struct {
	Exp      *time.Time
	IssuedAt time.Time
	Timeout  string
}

type oneShotMeta struct {
	Path     string
	Exp      time.Time
	IssuedAt time.Time
}

func (s *Service) sessionMeta(timeout string, minTTL time.Duration, now time.Time) sessionMeta {
	expires := SessionExpiry(timeout, now)
	if expires != nil && minTTL > 0 && expires.Before(now.Add(minTTL)) {
		next := now.Add(minTTL)
		expires = &next
	}
	return sessionMeta{Exp: expires, IssuedAt: now, Timeout: NormalizeTimeout(timeout)}
}

func (s *Service) IssueToken() string {
	config := s.Config()
	token := NewToken()
	now := s.nowTime()
	s.mu.Lock()
	s.pruneLocked(now)
	s.sessions[token] = s.sessionMeta(config.Timeout, 0, now)
	s.capSessionsLocked()
	s.mu.Unlock()
	return token
}

func (s *Service) TokenOK(token string) bool {
	if !s.Config().Enabled {
		return true
	}
	return s.tokenOKEnabled(token)
}

func (s *Service) tokenOKEnabled(token string) bool {
	if token == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	meta, ok := s.sessions[token]
	if !ok {
		return false
	}
	if meta.Exp == nil || meta.Exp.After(s.nowTime()) {
		return true
	}
	delete(s.sessions, token)
	return false
}

func (s *Service) SessionTTL(token string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	meta, ok := s.sessions[token]
	if !ok || meta.Exp == nil {
		return 0
	}
	seconds := int(time.Until(*meta.Exp).Seconds())
	if seconds < 0 {
		return 0
	}
	return seconds
}

func (s *Service) RefreshSession(token, timeout string) map[string]any {
	now := s.nowTime()
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sessions[token]; !ok {
		return map[string]any{"sessionRefreshed": false, "sessionTtl": 0}
	}
	s.sessions[token] = s.sessionMeta(timeout, RefreshGrace, now)
	seconds := 0
	if exp := s.sessions[token].Exp; exp != nil {
		seconds = int(time.Until(*exp).Seconds())
		if seconds < 0 {
			seconds = 0
		}
	}
	return map[string]any{"sessionRefreshed": true, "sessionTtl": seconds}
}

func (s *Service) Revoke(token string) {
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
}

func (s *Service) IssueOneShot(path string) string {
	token := NewToken()
	now := s.nowTime()
	s.mu.Lock()
	s.pruneLocked(now)
	s.oneShots[token] = oneShotMeta{Path: path, Exp: now.Add(OneShotTTL), IssuedAt: now}
	s.capOneShotsLocked()
	s.mu.Unlock()
	return token
}

func (s *Service) ConsumeOneShot(token, path string) bool {
	if !s.Config().Enabled {
		return true
	}
	if token == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	meta, ok := s.oneShots[token]
	if !ok {
		return false
	}
	delete(s.oneShots, token)
	return meta.Path == path && s.nowTime().Before(meta.Exp)
}

func (s *Service) PINLockoutRemaining() int {
	now := s.nowTime()
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := s.failTimes[:0]
	for _, when := range s.failTimes {
		if now.Sub(when) <= PINFailureWindow {
			kept = append(kept, when)
		}
	}
	s.failTimes = kept
	if len(s.failTimes) < MaxPINFailures {
		return 0
	}
	remaining := int((PINFailureWindow - now.Sub(s.failTimes[0])).Seconds())
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (s *Service) RecordPINFailure() {
	s.mu.Lock()
	s.failTimes = append(s.failTimes, s.nowTime())
	s.mu.Unlock()
}

func (s *Service) ClearPINFailures() {
	s.mu.Lock()
	s.failTimes = nil
	s.mu.Unlock()
}

func (s *Service) pruneLocked(now time.Time) {
	for token, meta := range s.sessions {
		if meta.Exp != nil && meta.Exp.Before(now) {
			delete(s.sessions, token)
		}
	}
	for token, meta := range s.oneShots {
		if meta.Exp.Before(now) {
			delete(s.oneShots, token)
		}
	}
}

func (s *Service) capSessionsLocked() {
	for len(s.sessions) > MaxSessionTokens {
		oldestToken := ""
		var oldest time.Time
		for token, meta := range s.sessions {
			if oldestToken == "" || meta.IssuedAt.Before(oldest) || (meta.IssuedAt.Equal(oldest) && token < oldestToken) {
				oldestToken, oldest = token, meta.IssuedAt
			}
		}
		delete(s.sessions, oldestToken)
	}
}

func (s *Service) capOneShotsLocked() {
	for len(s.oneShots) > MaxOneShotTokens {
		oldestToken := ""
		var oldest time.Time
		for token, meta := range s.oneShots {
			if oldestToken == "" || meta.IssuedAt.Before(oldest) || (meta.IssuedAt.Equal(oldest) && token < oldestToken) {
				oldestToken, oldest = token, meta.IssuedAt
			}
		}
		delete(s.oneShots, oldestToken)
	}
}
