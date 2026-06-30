// Package settings owns Dash-Go's durable settings cache, safety boundary,
// profile model, visual-config helpers, and runtime-font storage. It accepts
// narrow configuration values and callbacks from the core server; it never
// imports package main.
package settings

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
)

// RadarValidator validates the Radar-owned settings subset. The callback keeps
// settings independent from the Radar package while preserving the existing
// safety boundary.
type RadarValidator func(map[string]any) error

// Config contains the stable paths and narrow cross-boundary hook required by
// a settings service instance.
type Config struct {
	SettingsFile    string
	ConfigLocal     string
	CacheDir        string
	ThemeCatalog    string
	FontsDir        string
	BundledFontsDir string
	ValidateRadar   RadarValidator
}

// Service owns mutable settings state. Paths are immutable for a normal server
// lifetime; test-only app facades may construct a new service when a fixture
// supplies different paths.
type Service struct {
	settingsFile    string
	configLocal     string
	cacheDir        string
	themeCatalog    string
	fontsDir        string
	bundledFontsDir string
	validateRadar   RadarValidator

	mu         sync.Mutex
	writeMu    sync.Mutex
	fontMu     sync.Mutex
	fontChecks map[string]runtimeFontVerification
	cache      map[string]any
	cacheMod   time.Time
	cacheSize  int64
	cacheOK    bool
}

func New(cfg Config) *Service {
	return &Service{
		settingsFile:    cfg.SettingsFile,
		configLocal:     cfg.ConfigLocal,
		cacheDir:        cfg.CacheDir,
		themeCatalog:    cfg.ThemeCatalog,
		fontsDir:        cfg.FontsDir,
		bundledFontsDir: cfg.BundledFontsDir,
		validateRadar:   cfg.ValidateRadar,
	}
}

// Matches reports whether the durable path portion of cfg describes this
// service. The core uses it only for isolated test fixtures that mutate paths.
func (s *Service) Matches(cfg Config) bool {
	if s == nil {
		return false
	}
	return s.settingsFile == cfg.SettingsFile &&
		s.configLocal == cfg.ConfigLocal &&
		s.cacheDir == cfg.CacheDir &&
		s.themeCatalog == cfg.ThemeCatalog &&
		s.fontsDir == cfg.FontsDir &&
		s.bundledFontsDir == cfg.BundledFontsDir
}

func (s *Service) ConfigLocal() string { return s.configLocal }
func (s *Service) FontsDir() string    { return s.fontsDir }

// CloneMap is intentionally shallow; callers must not mutate nested maps or
// slices in place. It returns a usable empty map for a nil input.
func CloneMap(src map[string]any) map[string]any {
	out := make(map[string]any, len(src))
	maps.Copy(out, src)
	return out
}

func (s *Service) Invalidate() {
	s.mu.Lock()
	s.cacheOK = false
	s.cache = nil
	s.mu.Unlock()
}

func (s *Service) Load() map[string]any {
	st, err := os.Stat(s.settingsFile)
	if err != nil || st.IsDir() {
		return map[string]any{}
	}
	s.mu.Lock()
	if s.cacheOK && st.ModTime().Equal(s.cacheMod) && st.Size() == s.cacheSize {
		out := CloneMap(s.cache)
		s.mu.Unlock()
		return out
	}
	b, err := os.ReadFile(s.settingsFile)
	if err != nil {
		s.mu.Unlock()
		return map[string]any{}
	}
	values := map[string]any{}
	if err := json.Unmarshal(b, &values); err != nil {
		values = map[string]any{}
	}
	s.cache = CloneMap(values)
	s.cacheMod = st.ModTime()
	s.cacheSize = st.Size()
	s.cacheOK = true
	out := CloneMap(values)
	s.mu.Unlock()
	return out
}

// Mutate serializes one complete read-modify-write transaction. It is the
// cross-domain transaction seam for callers that need mutation errors without
// reintroducing an app-owned settings write mutex.
func (s *Service) Mutate(mut func(map[string]any) error) (map[string]any, error) {
	if mut == nil {
		return nil, fmt.Errorf("settings mutation is required")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	values := s.Load()
	if err := mut(values); err != nil {
		return nil, err
	}
	if err := s.Write(values); err != nil {
		return nil, err
	}
	return values, nil
}

func (s *Service) Update(mut func(map[string]any)) (map[string]any, error) {
	if mut == nil {
		return nil, fmt.Errorf("settings mutation is required")
	}
	return s.Mutate(func(values map[string]any) error {
		mut(values)
		return nil
	})
}

func (s *Service) lastGoodFile() string { return s.settingsFile + ".last-good" }
func (s *Service) revertFile() string   { return filepath.Join(s.cacheDir, "config-revert.json") }

func (s *Service) readObject(path string) (map[string]any, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var values map[string]any
	if err := json.Unmarshal(b, &values); err != nil {
		return nil, err
	}
	if err := ValidateShape(values, s.validateRadar); err != nil {
		return nil, err
	}
	return values, nil
}

// ReadObject validates a settings JSON object with the supplied Radar seam.
// It supports core CLI/fixture adapters without exposing Service internals.
func ReadObject(path string, validateRadar RadarValidator) (map[string]any, error) {
	return New(Config{SettingsFile: path, ValidateRadar: validateRadar}).readObject(path)
}

// Write is the only settings-specific write path. It validates before replacing
// the live file, atomically writes it, and snapshots valid content as last-good
// only after the live write succeeds.
func (s *Service) Write(values map[string]any) error {
	if err := ValidateShape(values, s.validateRadar); err != nil {
		return err
	}
	if err := fileio.WriteJSON(s.settingsFile, values); err != nil {
		return err
	}
	if err := fileio.WriteJSON(s.lastGoodFile(), values); err != nil {
		return err
	}
	_ = os.Remove(s.revertFile())
	s.Invalidate()
	return nil
}

// EnsureSafeAtBoot runs only during process start. A malformed live settings
// file falls back to a previously validated last-good snapshot when possible.
func (s *Service) EnsureSafeAtBoot() {
	if _, err := os.Stat(s.settingsFile); os.IsNotExist(err) {
		return
	}
	values, err := s.readObject(s.settingsFile)
	if err == nil {
		if _, lastErr := s.readObject(s.lastGoodFile()); lastErr != nil {
			_ = fileio.WriteJSON(s.lastGoodFile(), values)
		}
		return
	}
	last, lastErr := s.readObject(s.lastGoodFile())
	marker := map[string]any{
		"updated": time.Now().Format(time.RFC3339),
		"reason":  strings.TrimSpace(err.Error()),
		"state":   "invalid-no-last-good",
	}
	if lastErr == nil {
		if writeErr := fileio.WriteJSON(s.settingsFile, last); writeErr == nil {
			s.Invalidate()
			marker["state"] = "reverted"
			marker["lastGood"] = filepath.Base(s.lastGoodFile())
		} else {
			marker["restoreError"] = writeErr.Error()
		}
	} else {
		marker["lastGoodError"] = strings.TrimSpace(lastErr.Error())
	}
	_ = fileio.WriteJSON(s.revertFile(), marker)
}

// ValidateCLI preserves the settings validation/repair behavior used by the
// core command adapter.
func (s *Service) ValidateCLI(repair bool) (string, error) {
	if _, err := s.readObject(s.settingsFile); err == nil {
		return "settings valid", nil
	} else if !repair {
		return "", fmt.Errorf("settings invalid: %w", err)
	}
	s.EnsureSafeAtBoot()
	if _, err := s.readObject(s.settingsFile); err != nil {
		return "", fmt.Errorf("settings repair unavailable: %w", err)
	}
	return "settings restored from last-good", nil
}
