package family

import (
	"os"
	"time"

	controlauth "github.com/DashDashGoApp/Dash-Go/app/internal/auth"
)

func (s *Service) IssueSession(personID string) string {
	personID = PersonID(personID)
	token := s.token()
	now := s.Now()
	s.inboxMu.Lock()
	if s.sessions == nil {
		s.sessions = map[string]InboxSession{}
	}
	if s.failures == nil {
		s.failures = map[string]inboxPINLockout{}
	}
	s.sessions[token] = InboxSession{PersonID: personID, Exp: now.Add(InboxSessionTTL)}
	s.pruneLocked(now)
	s.inboxMu.Unlock()
	return token
}

func (s *Service) SessionOK(token, personID string) bool {
	if token == "" || personID == "" {
		return false
	}
	personID = PersonID(personID)
	s.inboxMu.Lock()
	defer s.inboxMu.Unlock()
	now := s.Now()
	s.pruneLocked(now)
	entry, ok := s.sessions[token]
	if !ok || entry.PersonID != personID || !entry.Exp.After(now) {
		delete(s.sessions, token)
		return false
	}
	entry.Exp = now.Add(InboxSessionTTL)
	s.sessions[token] = entry
	return true
}

func (s *Service) Session(token string) (InboxSession, bool) {
	if token == "" {
		return InboxSession{}, false
	}
	s.inboxMu.Lock()
	defer s.inboxMu.Unlock()
	s.pruneLocked(s.Now())
	entry, ok := s.sessions[token]
	return entry, ok
}

func (s *Service) RevokeSession(token string) {
	s.inboxMu.Lock()
	delete(s.sessions, token)
	s.inboxMu.Unlock()
}

func (s *Service) RevokeSessions(personID string) {
	personID = PersonID(personID)
	s.inboxMu.Lock()
	for token, entry := range s.sessions {
		if personID == "" || entry.PersonID == personID {
			delete(s.sessions, token)
		}
	}
	s.inboxMu.Unlock()
}

func (s *Service) pruneLocked(now time.Time) {
	for token, entry := range s.sessions {
		if !entry.Exp.After(now) {
			delete(s.sessions, token)
		}
	}
}

func (s *Service) LockoutRemaining(personID string) int {
	personID = PersonID(personID)
	s.inboxMu.Lock()
	defer s.inboxMu.Unlock()
	return s.inboxLockoutRemainingLocked(personID, s.Now())
}

// RecordPINFailure persists a per-person escalating lockout. A failed inbox
// PIN cannot regain a fresh eight-attempt window by restarting the server.
func (s *Service) RecordPINFailure(personID string) int {
	personID = PersonID(personID)
	now := s.Now()
	s.inboxMu.Lock()
	defer s.inboxMu.Unlock()
	if s.failures == nil {
		s.failures = map[string]inboxPINLockout{}
	}
	if remaining := s.inboxLockoutRemainingLocked(personID, now); remaining > 0 {
		return remaining
	}
	state := s.failures[personID]
	if state.Failures < controlauth.MaxPersistentPINFailures {
		state.Failures++
	}
	if duration := inboxLockoutDuration(state.Failures); duration > 0 {
		state.LockedUntil = now.Add(duration).Unix()
	} else {
		state.LockedUntil = 0
	}
	s.failures[personID] = state
	s.saveInboxLockoutsLocked()
	return s.inboxLockoutRemainingLocked(personID, now)
}

func inboxLockoutDuration(failures int) time.Duration {
	if failures < controlauth.MaxPINFailures {
		return 0
	}
	steps := failures - controlauth.MaxPINFailures
	duration := controlauth.InitialPINLockout
	for steps > 0 && duration < controlauth.MaxPINLockout {
		duration *= 2
		steps--
	}
	if duration > controlauth.MaxPINLockout {
		return controlauth.MaxPINLockout
	}
	return duration
}

func (s *Service) ClearPINFailures(personID string) {
	personID = PersonID(personID)
	s.inboxMu.Lock()
	delete(s.failures, personID)
	path := s.lockoutPath
	empty := len(s.failures) == 0
	if !empty {
		s.saveInboxLockoutsLocked()
	}
	s.inboxMu.Unlock()
	if empty && path != "" {
		_ = os.Remove(path)
	}
}
