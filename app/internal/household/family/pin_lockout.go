package family

import (
	"encoding/json"
	"os"
	"time"

	controlauth "github.com/DashDashGoApp/Dash-Go/app/internal/auth"
	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
)

const inboxLockoutStateSchema = 1

type inboxPINLockout struct {
	Failures    int   `json:"failures"`
	LockedUntil int64 `json:"lockedUntil"`
}

type inboxLockoutDocument struct {
	Schema   int                        `json:"schema"`
	Lockouts map[string]inboxPINLockout `json:"lockouts"`
}

func (s *Service) loadInboxLockouts() {
	if s.lockoutPath == "" {
		return
	}
	body, err := os.ReadFile(s.lockoutPath)
	if err != nil {
		return
	}
	var doc inboxLockoutDocument
	if json.Unmarshal(body, &doc) != nil || doc.Schema != inboxLockoutStateSchema {
		return
	}
	loaded := map[string]inboxPINLockout{}
	for rawID, state := range doc.Lockouts {
		personID := PersonID(rawID)
		if personID == "" || state.Failures < 0 || state.LockedUntil < 0 {
			continue
		}
		if state.Failures > controlauth.MaxPersistentPINFailures {
			state.Failures = controlauth.MaxPersistentPINFailures
		}
		loaded[personID] = state
	}
	s.inboxMu.Lock()
	s.failures = loaded
	s.inboxMu.Unlock()
}

func (s *Service) saveInboxLockoutsLocked() {
	if s.lockoutPath == "" {
		return
	}
	doc := inboxLockoutDocument{Schema: inboxLockoutStateSchema, Lockouts: s.failures}
	body, err := json.Marshal(doc)
	if err != nil {
		return
	}
	_ = fileio.WriteAtomic(s.lockoutPath, append(body, '\n'), 0600)
}

func inboxRemainingLockout(lockedUntil int64, now time.Time) int {
	if lockedUntil == 0 {
		return 0
	}
	target := time.Unix(lockedUntil, 0)
	if !target.After(now) {
		return 0
	}
	return int((target.Sub(now) + time.Second - 1) / time.Second)
}

func (s *Service) inboxLockoutRemainingLocked(personID string, now time.Time) int {
	return inboxRemainingLockout(s.failures[personID].LockedUntil, now)
}
