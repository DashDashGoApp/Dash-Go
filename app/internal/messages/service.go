// Package messages owns Dash-Go's rotating-message catalog, cached feeds,
// provider fetches, temporary/scheduled messages, compliments, birthdays, and
// celebrations. It is local-first: durable files remain in the existing
// config/home locations, and failed network fetches retain the existing local
// fallback behavior.
package messages

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// ServiceConfig contains the narrow core handles Messages needs. The package
// never imports package main; core startup supplies immutable paths and the
// few callbacks that cross the message/calendar/platform boundaries.
type ServiceConfig struct {
	Home             string
	ConfigDir        string
	ConfigLocal      string
	CelebrationsFile string

	GenerateDefaultCalendars func(bool) (map[string]any, error)
	ProviderBackoffActive    func(string) (time.Time, string, int, bool)
	NoteProviderBackoff      func(string, error)
	ClearProviderBackoff     func(string)
	NetworkLikelyAvailable   func() bool
}

// Service owns the former message write mutex along with message-only paths
// and callbacks. All cache/override read-modify-write transactions stay
// serialized inside this bounded context.
type Service struct {
	home             string
	configDir        string
	configLocal      string
	celebrationsFile string

	generateDefaultCalendarsFn func(bool) (map[string]any, error)
	providerBackoffActiveFn    func(string) (time.Time, string, int, bool)
	noteProviderBackoffFn      func(string, error)
	clearProviderBackoffFn     func(string)
	networkLikelyAvailableFn   func() bool

	messageWriteMu sync.Mutex
}

func New(cfg ServiceConfig) *Service {
	return &Service{
		home:                       cfg.Home,
		configDir:                  cfg.ConfigDir,
		configLocal:                cfg.ConfigLocal,
		celebrationsFile:           cfg.CelebrationsFile,
		generateDefaultCalendarsFn: cfg.GenerateDefaultCalendars,
		providerBackoffActiveFn:    cfg.ProviderBackoffActive,
		noteProviderBackoffFn:      cfg.NoteProviderBackoff,
		clearProviderBackoffFn:     cfg.ClearProviderBackoff,
		networkLikelyAvailableFn:   cfg.NetworkLikelyAvailable,
	}
}

func (s *Service) generateDefaultCalendars(refresh bool) (map[string]any, error) {
	if s == nil || s.generateDefaultCalendarsFn == nil {
		return map[string]any{}, nil
	}
	return s.generateDefaultCalendarsFn(refresh)
}

func (s *Service) providerBackoffActive(name string) (time.Time, string, int, bool) {
	if s == nil || s.providerBackoffActiveFn == nil {
		return time.Time{}, "", 0, false
	}
	return s.providerBackoffActiveFn(name)
}

func (s *Service) noteProviderBackoff(name string, err error) {
	if s != nil && s.noteProviderBackoffFn != nil {
		s.noteProviderBackoffFn(name, err)
	}
}

func (s *Service) clearProviderBackoff(name string) {
	if s != nil && s.clearProviderBackoffFn != nil {
		s.clearProviderBackoffFn(name)
	}
}

func (s *Service) networkLikelyAvailable() bool {
	if s == nil || s.networkLikelyAvailableFn == nil {
		return true
	}
	return s.networkLikelyAvailableFn()
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

func (s *Service) json(w http.ResponseWriter, value any, code ...int) {
	status := http.StatusOK
	if len(code) > 0 {
		status = code[0]
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (s *Service) err(w http.ResponseWriter, message string, code int) {
	s.json(w, map[string]any{"error": message}, code)
}

func clamp(value, lower, upper int) int { return min(max(value, lower), upper) }

func compareText(left, right any) int { return strings.Compare(fmt.Sprint(left), fmt.Sprint(right)) }

func strOr(value any, def string) string {
	if text := jsonutil.TextValue(value); text != "" {
		return text
	}
	return def
}

func readEnv(path string) map[string]string {
	values := map[string]string{}
	body, err := os.ReadFile(path)
	if err != nil {
		return values
	}
	for line := range strings.SplitSeq(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		values[strings.TrimSpace(parts[0])] = strings.Trim(strings.TrimSpace(parts[1]), "\"'")
	}
	return values
}

var (
	controlChars       = regexp.MustCompile(`[\x00-\x1f]+`)
	whitespaceRun      = regexp.MustCompile(`\s+`)
	reNSFWPrefix       = regexp.MustCompile(`(?i)^nsfw:\s*`)
	reMonthDay         = regexp.MustCompile(`^\d{2}-\d{2}$`)
	reISODate          = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	reTimeOfDay        = regexp.MustCompile(`^\d{1,2}:\d{2}$`)
	reBirthdayList     = regexp.MustCompile(`(?ms)\bbirthdays\s*:\s*(\[[^\]]*\])`)
	reBirthdayListLine = regexp.MustCompile(`(?ms)^(\s*)birthdays\s*:\s*\[[^\]]*\]\s*,?`)
	reFeedExpiry       = regexp.MustCompile(`^(30|60|120|360|720|1440)m$`)
)
