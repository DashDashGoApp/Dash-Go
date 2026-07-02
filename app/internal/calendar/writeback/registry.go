// Package writeback owns the narrow, local-first vdir mutation boundary used
// by Dash-Go calendar edits. A source is writable only when it is explicitly
// registered to a trusted private vdir collection; URL ICS and generated feeds
// never enter this registry and therefore cannot be written by any API path.
package writeback

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
)

const RegistryVersion = 1

var ErrBusy = errors.New("calendar sync is already running")

type Calendar struct {
	Source     string `json:"source"`
	Collection string `json:"collection"`
	Writable   bool   `json:"writable"`
	Enabled    bool   `json:"enabled"`
	Name       string `json:"name,omitempty"`
}
type Registry struct {
	Version    int        `json:"version"`
	Enabled    bool       `json:"enabled"`
	RequirePIN bool       `json:"requirePin"`
	Calendars  []Calendar `json:"calendars"`
}
type Status struct {
	Enabled    bool           `json:"enabled"`
	RequirePIN bool           `json:"requirePin"`
	Calendars  []Calendar     `json:"calendars"`
	Last       map[string]any `json:"last,omitempty"`
}

type Config struct {
	RegistryPath string
	StatusPath   string
	VDirHome     string
	CalendarDir  string
	Now          func() time.Time
}
type Service struct {
	registryPath string
	statusPath   string
	vdirHome     string
	calendarDir  string
	nowFn        func() time.Time
	mu           sync.Mutex
}

func New(cfg Config) *Service {
	return &Service{registryPath: cfg.RegistryPath, statusPath: cfg.StatusPath, vdirHome: cfg.VDirHome, calendarDir: cfg.CalendarDir, nowFn: cfg.Now}
}
func (s *Service) now() time.Time {
	if s != nil && s.nowFn != nil {
		return s.nowFn()
	}
	return time.Now()
}
func (s *Service) RegistryPath() string {
	if s == nil {
		return ""
	}
	return s.registryPath
}

func defaultRegistry() Registry { return Registry{Version: RegistryVersion, Calendars: []Calendar{}} }
func (s *Service) loadLocked() (Registry, error) {
	reg := defaultRegistry()
	b, err := os.ReadFile(s.registryPath)
	if os.IsNotExist(err) {
		return reg, nil
	}
	if err != nil {
		return Registry{}, fmt.Errorf("read calendar writeback registry: %w", err)
	}
	if err := jsonUnmarshalStrict(b, &reg); err != nil {
		return Registry{}, fmt.Errorf("invalid calendar writeback registry: %w", err)
	}
	if reg.Version != RegistryVersion {
		return Registry{}, fmt.Errorf("unsupported calendar writeback registry")
	}
	if reg.Calendars == nil {
		reg.Calendars = []Calendar{}
	}
	if err := s.validateRegistry(reg); err != nil {
		return Registry{}, err
	}
	return reg, nil
}
func (s *Service) writeLocked(reg Registry) error {
	reg.Version = RegistryVersion
	if err := s.validateRegistry(reg); err != nil {
		return err
	}
	return fileio.WriteJSON(s.registryPath, reg)
}

func (s *Service) validateRegistry(reg Registry) error {
	seen := map[string]bool{}
	for _, item := range reg.Calendars {
		source := strings.TrimSpace(item.Source)
		if !validSource(source) {
			return fmt.Errorf("invalid writable calendar source")
		}
		if seen[source] {
			return fmt.Errorf("duplicate writable calendar source")
		}
		seen[source] = true
		if !item.Writable {
			continue
		}
		if err := s.validateCollection(item.Collection); err != nil {
			return err
		}
	}
	return nil
}
func validSource(source string) bool {
	source = filepath.ToSlash(strings.TrimSpace(source))
	if !strings.HasPrefix(source, "calendars/") || strings.Contains(source, "..") {
		return false
	}
	base := strings.TrimPrefix(source, "calendars/")
	return base != "" && !strings.Contains(base, "/") && strings.HasSuffix(strings.ToLower(base), ".ics")
}
func (s *Service) validateCollection(collection string) error {
	collection = filepath.Clean(strings.TrimSpace(collection))
	if collection == "." || !filepath.IsAbs(collection) {
		return fmt.Errorf("writable calendar collection is invalid")
	}
	root := filepath.Join(filepath.Clean(s.vdirHome), "collections")
	rel, err := filepath.Rel(root, collection)
	if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return fmt.Errorf("writable calendar collection is outside the private vdir root")
	}
	return nil
}
func (s *Service) sourcePath(source string) (string, error) {
	if !validSource(source) {
		return "", fmt.Errorf("invalid calendar source")
	}
	return filepath.Join(s.calendarDir, filepath.Base(source)), nil
}

func (s *Service) Status() (Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	reg, err := s.loadLocked()
	if err != nil {
		return Status{}, err
	}
	rows := append([]Calendar(nil), reg.Calendars...)
	slices.SortFunc(rows, func(a, b Calendar) int { return strings.Compare(strings.ToLower(a.Source), strings.ToLower(b.Source)) })
	out := Status{Enabled: reg.Enabled, RequirePIN: reg.RequirePIN, Calendars: rows}
	if b, err := os.ReadFile(s.statusPath); err == nil {
		var last map[string]any
		if jsonUnmarshalStrict(b, &last) == nil {
			out.Last = last
		}
	}
	return out, nil
}

// RegisteredSource reports a trusted vdir mirror regardless of whether the
// global writeback switch is currently on. It protects Calendar Manager from
// trashing a derived mirror while keeping the remote collection untouched.
func (s *Service) RegisteredSource(source string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	reg, err := s.loadLocked()
	if err != nil {
		return false
	}
	source = filepath.ToSlash(source)
	for _, item := range reg.Calendars {
		if item.Source == source && item.Writable {
			return true
		}
	}
	return false
}

func (s *Service) SourceWritable(source string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	reg, err := s.loadLocked()
	if err != nil || !reg.Enabled {
		return false
	}
	source = filepath.ToSlash(source)
	for _, item := range reg.Calendars {
		if item.Source == source && item.Writable && item.Enabled {
			return true
		}
	}
	return false
}
func (s *Service) RequirePIN() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	reg, err := s.loadLocked()
	return err == nil && reg.Enabled && reg.RequirePIN
}
func (s *Service) Resolve(source string) (Calendar, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	reg, err := s.loadLocked()
	if err != nil {
		return Calendar{}, err
	}
	if !reg.Enabled {
		return Calendar{}, errors.New("calendar editing is not enabled")
	}
	source = filepath.ToSlash(source)
	for _, item := range reg.Calendars {
		if item.Source == source && item.Writable && item.Enabled {
			return item, nil
		}
	}
	return Calendar{}, errors.New("calendar is read-only on this dashboard")
}
func (s *Service) Configure(enabled, requirePIN bool, requested map[string]bool) (Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	reg, err := s.loadLocked()
	if err != nil {
		return Status{}, err
	}
	reg.Enabled, reg.RequirePIN = enabled, requirePIN
	for i := range reg.Calendars {
		if requested != nil {
			if value, ok := requested[reg.Calendars[i].Source]; ok {
				reg.Calendars[i].Enabled = value
			}
		}
	}
	if err := s.writeLocked(reg); err != nil {
		return Status{}, err
	}
	return s.statusLocked(reg), nil
}
func (s *Service) statusLocked(reg Registry) Status {
	rows := append([]Calendar(nil), reg.Calendars...)
	return Status{Enabled: reg.Enabled, RequirePIN: reg.RequirePIN, Calendars: rows}
}
func (s *Service) Record(source, state, detail string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	payload := map[string]any{"updatedAt": s.now().UTC().Format(time.RFC3339), "source": source, "state": state, "detail": detail}
	_ = fileio.WriteJSON(s.statusPath, payload)
}

// WithSyncLock shares sync-vdir.sh's directory lock. It never waits behind a
// cron/manual sync; callers receive ErrBusy and leave data unchanged.
func (s *Service) WithSyncLock(fn func() error) error {
	lock := filepath.Join(s.vdirHome, "sync.lock")
	if err := os.MkdirAll(s.vdirHome, 0700); err != nil {
		return err
	}
	if err := os.Mkdir(lock, 0700); err != nil {
		if !os.IsExist(err) {
			return err
		}
		if staleLock(lock) {
			_ = os.RemoveAll(lock)
			if err := os.Mkdir(lock, 0700); err != nil {
				return ErrBusy
			}
		} else {
			return ErrBusy
		}
	}
	_ = fileio.WriteAtomic(filepath.Join(lock, "pid"), []byte(fmt.Sprintf("%d\n", os.Getpid())), 0600)
	defer os.RemoveAll(lock)
	return fn()
}
func staleLock(lock string) bool {
	b, err := os.ReadFile(filepath.Join(lock, "pid"))
	if err != nil {
		return false
	}
	var pid int
	if _, err = fmt.Sscanf(strings.TrimSpace(string(b)), "%d", &pid); err != nil || pid <= 0 {
		return false
	}
	return processGone(pid)
}
