// Package household owns Dash-Go's canonical local household roster.
//
// The roster is intentionally small, local-first, and independent of any one
// household app. Dependent apps retain their own task/calendar/history state;
// they read this service for current people and use stable IDs plus snapshots
// for historical records. The package never imports package main.
package household

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// Schema is the durable household roster schema.
const Schema = 1

// MaxPeople is the intentional roster ceiling shared by People and Routines.
const MaxPeople = 20

// ServiceConfig contains the narrow runtime inputs owned by core startup.
type ServiceConfig struct {
	ConfigDir string
	Now       func() time.Time
}

// Service owns the roster path and the mutex that serializes People mutations.
type Service struct {
	configDir string
	nowFn     func() time.Time
	mu        sync.Mutex
}

// New constructs a household roster service. It does not create or seed any
// user-owned file; missing or malformed files remain an empty usable roster.
func New(cfg ServiceConfig) *Service {
	return &Service{configDir: cfg.ConfigDir, nowFn: cfg.Now}
}

func (s *Service) now() time.Time {
	if s != nil && s.nowFn != nil {
		return s.nowFn().In(time.Local)
	}
	return time.Now().In(time.Local)
}

// File is the sole durable canonical household roster path.
func (s *Service) File() string {
	if s == nil {
		return ""
	}
	return filepath.Join(s.configDir, "household-people.json")
}

func (s *Service) readJSONDefault(path string, def any) any {
	b, err := os.ReadFile(path)
	if err != nil {
		return def
	}
	var value any
	if err := json.Unmarshal(b, &value); err != nil {
		return def
	}
	return value
}

// Payload returns a normalized roster without writing a missing/default file.
func (s *Service) Payload() map[string]any {
	if s == nil {
		return Default()
	}
	return Normalize(jsonutil.Map(s.readJSONDefault(s.File(), Default())), s.now())
}

// Write persists one normalized roster using the existing atomic JSON helper.
func (s *Service) Write(value map[string]any) error {
	if s == nil {
		return nil
	}
	return fileio.WriteJSON(s.File(), Normalize(value, s.now()))
}

// WithLock preserves the historic People mutation critical section while the
// surrounding app-owned reconciliation callbacks remain in core for now.
func (s *Service) WithLock(fn func() error) error {
	if s == nil {
		if fn == nil {
			return nil
		}
		return fn()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if fn == nil {
		return nil
	}
	return fn()
}

// Ensure merges one-time migration candidates into the canonical roster. Once
// a canonical record exists, candidates cannot rename, revive, or overwrite it.
func (s *Service) Ensure(candidates ...[]any) map[string]any {
	var out map[string]any
	_ = s.WithLock(func() error {
		current := s.Payload()
		next := Merge(current, s.now(), candidates...)
		if !Equal(current, next, s.now()) {
			next["revision"] = max(0, jsonutil.Int(current["revision"], 0)) + 1
			_ = s.Write(next)
		}
		out = next
		return nil
	})
	if out == nil {
		return Default()
	}
	return out
}

// AssignmentLookup returns active or archived people so existing records may
// render a calm Former label. New assignments should use ActiveAssignment.
func (s *Service) AssignmentLookup(id string) (map[string]any, bool) {
	id = ID(id)
	if id == "" {
		return nil, false
	}
	for _, raw := range jsonutil.List(s.Payload()["people"]) {
		person := jsonutil.Map(raw)
		if ID(person["id"]) == id {
			return person, true
		}
	}
	return nil, false
}

// ActiveAssignment returns a currently assignable person only.
func (s *Service) ActiveAssignment(id string) (map[string]any, bool) {
	person, ok := s.AssignmentLookup(id)
	if !ok || person["state"] != "active" {
		return nil, false
	}
	return person, true
}
