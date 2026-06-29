package family

import "time"

func (s *Service) IssueSession(personID string) string {
	personID = PersonID(personID)
	token := s.token()
	now := s.Now()
	s.inboxMu.Lock()
	if s.sessions == nil {
		s.sessions = map[string]InboxSession{}
	}
	if s.failures == nil {
		s.failures = map[string][]time.Time{}
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
	for personID, attempts := range s.failures {
		kept := attempts[:0]
		for _, when := range attempts {
			if now.Sub(when) <= time.Minute {
				kept = append(kept, when)
			}
		}
		if len(kept) == 0 {
			delete(s.failures, personID)
		} else {
			s.failures[personID] = kept
		}
	}
}

func (s *Service) LockoutRemaining(personID string) int {
	personID = PersonID(personID)
	s.inboxMu.Lock()
	defer s.inboxMu.Unlock()
	now := s.Now()
	s.pruneLocked(now)
	attempts := s.failures[personID]
	if len(attempts) < 8 {
		return 0
	}
	remaining := int((time.Minute - now.Sub(attempts[0])).Seconds())
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (s *Service) RecordPINFailure(personID string) {
	personID = PersonID(personID)
	s.inboxMu.Lock()
	if s.failures == nil {
		s.failures = map[string][]time.Time{}
	}
	s.failures[personID] = append(s.failures[personID], s.Now())
	s.inboxMu.Unlock()
}

func (s *Service) ClearPINFailures(personID string) {
	s.inboxMu.Lock()
	delete(s.failures, PersonID(personID))
	s.inboxMu.Unlock()
}
