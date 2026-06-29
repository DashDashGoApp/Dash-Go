package maintenance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

type ServiceConfig struct {
	ConfigDir string
	Now       func() time.Time
}
type Service struct {
	configDir string
	nowFn     func() time.Time
	mu        sync.Mutex
}

func New(cfg ServiceConfig) *Service { return &Service{configDir: cfg.ConfigDir, nowFn: cfg.Now} }
func (s *Service) Now() time.Time {
	if s != nil && s.nowFn != nil {
		return s.nowFn().In(time.Local)
	}
	return time.Now().In(time.Local)
}

func (s *Service) File() string {
	if s == nil {
		return ""
	}
	return filepath.Join(s.configDir, "maintenance-tracker.json")
}
func (s *Service) readDefault(path string, def any) any {
	b, err := os.ReadFile(path)
	if err != nil {
		return def
	}
	var v any
	if json.Unmarshal(b, &v) != nil {
		return def
	}
	return v
}
func (s *Service) Payload() map[string]any {
	if s == nil {
		return Default()
	}
	return Normalize(jsonutil.Map(s.readDefault(s.File(), Default())), s.Now())
}
func (s *Service) Write(payload map[string]any) error {
	if s == nil {
		return nil
	}
	return fileio.WriteJSON(s.File(), Normalize(payload, s.Now()))
}
func (s *Service) WithLock(fn func() error) error {
	if s == nil {
		if fn != nil {
			return fn()
		}
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if fn != nil {
		return fn()
	}
	return nil
}
