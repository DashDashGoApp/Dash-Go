package auth

import (
	"sync"
	"time"
)

// Service owns the Dashboard Control PIN document and its bounded, in-memory
// authorization state. It never calls into a consumer while the runtime lock
// is held; callers receive stable payloads through the methods below.
type Service struct {
	envPath string
	now     func() time.Time

	mu        sync.Mutex
	sessions  map[string]sessionMeta
	oneShots  map[string]oneShotMeta
	failTimes []time.Time
}

type ServiceConfig struct {
	EnvPath string
	Now     func() time.Time
}

func NewService(cfg ServiceConfig) *Service {
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &Service{
		envPath:  cfg.EnvPath,
		now:      now,
		sessions: map[string]sessionMeta{},
		oneShots: map[string]oneShotMeta{},
	}
}

func (s *Service) nowTime() time.Time { return s.now() }
