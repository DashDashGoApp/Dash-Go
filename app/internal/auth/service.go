package auth

import (
	"path/filepath"
	"sync"
	"time"
)

// Service owns the Dashboard Control PIN document and its bounded
// authorization state. Credential-file access is serialized separately from
// token/lockout state so a concurrent timeout/PIN update cannot lose fields.
type Service struct {
	envPath      string
	recoveryPath string
	lockoutPath  string
	now          func() time.Time

	configMu    sync.RWMutex
	recoveryErr error

	mu       sync.Mutex
	sessions map[string]sessionMeta
	oneShots map[string]oneShotMeta
	lockout  pinLockoutState
}

type ServiceConfig struct {
	EnvPath      string
	RecoveryPath string
	LockoutPath  string
	Now          func() time.Time
}

func NewService(cfg ServiceConfig) *Service {
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	recoveryPath := cfg.RecoveryPath
	if recoveryPath == "" && cfg.EnvPath != "" {
		recoveryPath = filepath.Join(filepath.Dir(cfg.EnvPath), ".dashboard-control-pin-reset")
	}
	lockoutPath := cfg.LockoutPath
	if lockoutPath == "" && cfg.EnvPath != "" {
		lockoutPath = filepath.Join(filepath.Dir(cfg.EnvPath), ".dashboard-control-pin-lockout.json")
	}
	s := &Service{
		envPath:      cfg.EnvPath,
		recoveryPath: recoveryPath,
		lockoutPath:  lockoutPath,
		now:          now,
		sessions:     map[string]sessionMeta{},
		oneShots:     map[string]oneShotMeta{},
	}
	s.initializeLocalRecovery()
	s.loadPINLockout()
	return s
}

func (s *Service) nowTime() time.Time { return s.now() }
