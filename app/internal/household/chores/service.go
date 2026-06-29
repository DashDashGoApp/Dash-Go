// Package chores owns the durable Chore Wheel model and its deterministic
// server-side projections. It intentionally does not import package main,
// the People service, or calendar persistence. Core supplies the canonical
// roster transaction and writes projected events through its calendar port.
package chores

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

const Schema = 1

type ServiceConfig struct {
	ConfigDir string
	Now       func() time.Time
}

type Service struct {
	file string
	now  func() time.Time
	mu   sync.Mutex
}

func New(cfg ServiceConfig) *Service {
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &Service{file: filepath.Join(cfg.ConfigDir, "chore-wheel.json"), now: now}
}

func (s *Service) File() string { return s.file }

func (s *Service) Now() time.Time { return s.now().In(time.Local) }
func (s *Service) Today() string  { return LocalDateKey(s.Now()) }

func (s *Service) WithLock(fn func() error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return fn()
}

func (s *Service) Payload() map[string]any {
	b, err := os.ReadFile(s.file)
	if err != nil {
		return Default()
	}
	var decoded any
	if err := json.Unmarshal(b, &decoded); err != nil {
		return Default()
	}
	return NormalizeAt(jsonutil.Map(decoded), s.Now())
}

func (s *Service) Write(payload map[string]any) error {
	return fileio.WriteJSON(s.file, NormalizeAt(payload, s.Now()))
}
