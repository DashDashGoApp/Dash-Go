// Package writeback owns the narrow, local-first vdir mutation boundary used
// by Dash-Go calendar edits. A source is writable only when it is explicitly
// registered to a trusted private vdir collection; URL ICS and generated feeds
// never enter this registry and therefore cannot be written by any API path.
package writeback

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
)

const RegistryVersion = 2

var ErrBusy = errors.New("calendar sync is already running")

type Calendar struct {
	Source     string `json:"source"`
	Collection string `json:"collection"`
	Writable   bool   `json:"writable"`
	Enabled    bool   `json:"enabled"`
	Name       string `json:"name,omitempty"`
	// Pair is the exact generated vdirsyncer pair used by this dashboard source.
	// Empty is accepted only for a migrated v1 entry and falls back to a full
	// private-calendar sync until the setup refresh rewrites it.
	Pair       string `json:"pair,omitempty"`
	Provider   string `json:"provider,omitempty"`
	Connection string `json:"connection,omitempty"`
	RemoteID   string `json:"remoteId,omitempty"`
}
type Registry struct {
	Version    int        `json:"version"`
	Enabled    bool       `json:"enabled"`
	RequirePIN bool       `json:"requirePin"`
	Calendars  []Calendar `json:"calendars"`
}
type CalendarState struct {
	UpdatedAt string `json:"updatedAt"`
	State     string `json:"state"`
	Detail    string `json:"detail"`
	// Sync is a safe, provider-neutral machine state copied from the bounded
	// vdirsyncer wrapper result file. It is intentionally optional so existing
	// persisted status envelopes remain valid.
	Sync string `json:"sync,omitempty"`
}
type Status struct {
	Enabled    bool                     `json:"enabled"`
	RequirePIN bool                     `json:"requirePin"`
	Calendars  []Calendar               `json:"calendars"`
	States     map[string]CalendarState `json:"-"`
	Last       map[string]any           `json:"last,omitempty"`
}

type statusEnvelope struct {
	Version   int                      `json:"version"`
	Calendars map[string]CalendarState `json:"calendars"`
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
	// Beta.6 used v1. Its records contain all local safety information, so
	// migrate in memory and preserve the source until setup refresh assigns a
	// targeted vdirsyncer pair and provider metadata.
	if reg.Version == 1 {
		reg.Version = RegistryVersion
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
		if item.Pair != "" && !validPair(item.Pair) {
			return fmt.Errorf("invalid private calendar sync pair")
		}
		if item.Provider != "" && item.Provider != "caldav" && item.Provider != "google" {
			return fmt.Errorf("invalid private calendar provider")
		}
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
func validPair(value string) bool {
	if value == "" || len(value) > 96 {
		return false
	}
	for _, r := range value {
		if !(r == '-' || r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
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

func (s *Service) loadStatesLocked() (map[string]CalendarState, map[string]any) {
	states := map[string]CalendarState{}
	b, err := os.ReadFile(s.statusPath)
	if err != nil {
		return states, nil
	}
	var envelope statusEnvelope
	if jsonUnmarshalStrict(b, &envelope) == nil && envelope.Version == 1 && envelope.Calendars != nil {
		for source, state := range envelope.Calendars {
			if validSource(source) {
				states[source] = state
			}
		}
		return states, nil
	}
	// Beta.6 stored one last-write record. Keep it visible for its original
	// source while moving all new records into the per-calendar envelope.
	var legacy map[string]any
	if jsonUnmarshalStrict(b, &legacy) == nil {
		source, _ := legacy["source"].(string)
		if validSource(source) {
			states[source] = CalendarState{UpdatedAt: fmt.Sprint(legacy["updatedAt"]), State: fmt.Sprint(legacy["state"]), Detail: fmt.Sprint(legacy["detail"])}
		}
		return states, legacy
	}
	return states, nil
}

const (
	maxSyncResultBytes = 64 * 1024
	maxSyncResultRows  = 256
)

type syncOutcome struct {
	State string
	At    time.Time
}

func knownSyncOutcome(value string) bool {
	switch value {
	case "synced", "conflict", "attention-empty", "attention-undiscovered", "attention-auth", "failed", "skipped":
		return true
	default:
		return false
	}
}

// loadSyncOutcomesLocked reads the wrapper's cron-safe summary rather than its
// raw log. The file is treated as untrusted input: its size, row count, pair
// grammar, state vocabulary, and epoch are all bounded before it can affect a
// Dashboard Control row. Reading status never writes or repairs the file.
func (s *Service) loadSyncOutcomesLocked(reg Registry) map[string]syncOutcome {
	byPair := map[string]string{}
	for _, calendar := range reg.Calendars {
		if calendar.Writable && validPair(calendar.Pair) && validSource(calendar.Source) {
			byPair[calendar.Pair] = calendar.Source
		}
	}
	if len(byPair) == 0 {
		return map[string]syncOutcome{}
	}
	file, err := os.Open(filepath.Join(s.vdirHome, "last-sync-results"))
	if err != nil {
		return map[string]syncOutcome{}
	}
	defer file.Close()
	bytes, err := io.ReadAll(io.LimitReader(file, maxSyncResultBytes+1))
	if err != nil || len(bytes) > maxSyncResultBytes {
		return map[string]syncOutcome{}
	}
	out := map[string]syncOutcome{}
	rows := 0
	for _, line := range strings.Split(string(bytes), "\n") {
		if rows >= maxSyncResultRows {
			break
		}
		rows++
		parts := strings.Split(strings.TrimSpace(line), "|")
		if len(parts) != 3 {
			continue
		}
		pair, state := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if !validPair(pair) || !knownSyncOutcome(state) {
			continue
		}
		source, ok := byPair[pair]
		if !ok {
			continue
		}
		epoch, err := strconv.ParseInt(strings.TrimSpace(parts[2]), 10, 64)
		if err != nil || epoch <= 0 {
			continue
		}
		at := time.Unix(epoch, 0).UTC()
		if previous, exists := out[source]; !exists || at.After(previous.At) {
			out[source] = syncOutcome{State: state, At: at}
		}
	}
	return out
}

func calendarStateTime(value CalendarState) time.Time {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, strings.TrimSpace(value.UpdatedAt)); err == nil {
			return parsed.UTC()
		}
	}
	return time.Time{}
}

func outcomePresentation(outcome string, provider string) (state, detail, sync string) {
	sync = outcome
	providerName := "The calendar provider"
	if provider == "google" {
		providerName = "Google"
	} else if provider == "caldav" {
		providerName = "The calendar server"
	}
	switch outcome {
	case "synced":
		return "synced", "The last scheduled private-calendar sync completed.", sync
	case "conflict":
		return "conflict", "Dash-Go and the remote calendar changed before the last sync. Nothing was overwritten. Resolve this calendar before syncing normally again.", sync
	case "attention-undiscovered":
		return "attention", "This selected calendar needs one deliberate connection repair before it can sync.", sync
	case "attention-empty":
		return "attention", "This local calendar became unexpectedly empty. Sync is paused to protect remote events.", sync
	case "attention-auth":
		return "attention", providerName + " rejected this calendar's authorization. Reconnect it through private-calendar setup, then sync again.", sync
	case "skipped":
		return "attention", providerName + " authorization is required before this calendar can sync.", sync
	case "failed":
		return "waiting", "The last scheduled private-calendar sync failed safely. Existing dashboard events were kept.", sync
	default:
		return "", "", ""
	}
}

func (s *Service) mergeSyncOutcomesLocked(reg Registry, states map[string]CalendarState) {
	providerBySource := map[string]string{}
	for _, calendar := range reg.Calendars {
		providerBySource[calendar.Source] = calendar.Provider
	}
	for source, outcome := range s.loadSyncOutcomesLocked(reg) {
		if existing, ok := states[source]; ok && !outcome.At.After(calendarStateTime(existing)) {
			continue
		}
		state, detail, sync := outcomePresentation(outcome.State, providerBySource[source])
		if state == "" {
			continue
		}
		states[source] = CalendarState{UpdatedAt: outcome.At.Format(time.RFC3339), State: state, Detail: detail, Sync: sync}
	}
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
	states, last := s.loadStatesLocked()
	s.mergeSyncOutcomesLocked(reg, states)
	return Status{Enabled: reg.Enabled, RequirePIN: reg.RequirePIN, Calendars: rows, States: states, Last: last}, nil
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

// RegisteredCalendar resolves a trusted exact private source without requiring
// the global Dashboard edit switch. Recovery paths such as conflict resolution
// and connection repair must remain available even when ordinary event editing
// has been temporarily turned off.
func (s *Service) RegisteredCalendar(source string) (Calendar, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	reg, err := s.loadLocked()
	if err != nil {
		return Calendar{}, err
	}
	source = filepath.ToSlash(source)
	for _, item := range reg.Calendars {
		if item.Source == source && item.Writable && validPair(item.Pair) {
			return item, nil
		}
	}
	return Calendar{}, errors.New("selected private calendar is no longer registered")
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
	states, last := s.loadStatesLocked()
	s.mergeSyncOutcomesLocked(reg, states)
	return Status{Enabled: reg.Enabled, RequirePIN: reg.RequirePIN, Calendars: append([]Calendar(nil), reg.Calendars...), States: states, Last: last}, nil
}
func (s *Service) Record(source, state, detail string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	states, _ := s.loadStatesLocked()
	if !validSource(source) {
		return
	}
	states[source] = CalendarState{UpdatedAt: s.now().UTC().Format(time.RFC3339), State: strings.TrimSpace(state), Detail: strings.TrimSpace(detail)}
	_ = fileio.WriteJSON(s.statusPath, statusEnvelope{Version: 1, Calendars: states})
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
