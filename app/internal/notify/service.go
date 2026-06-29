// Package notify owns Dash-Go's Apprise-Go route configuration, person-level
// delivery preferences, bounded queueing, and external delivery dispatch. It
// is local-first: route URLs remain in an owner-only file and this package
// receives small callbacks for household identity and Family Board freshness
// instead of importing the application core.
package notify

import (
	"strings"
	"sync"
	"time"
)

const (
	RouteSchema        = 1
	PreferencesSchema  = 1
	MaxRoutesPerPerson = 8
	MaxRouteLength     = 2048
	QueueLimit         = 16

	recipientAttemptsMinute = 3
	allAttemptsMinute       = 10
	deliveryDeadline        = 10 * time.Second
)

// RouteStore is private server-side state. Route URLs must never be included
// in Dashboard Control payloads or normal browser APIs.
type RouteStore struct {
	Schema  int                 `json:"schema"`
	Enabled bool                `json:"enabled"`
	Routes  map[string][]string `json:"routes"`
}

// PersonPreferences is non-secret, person-scoped notification behavior. Route
// destinations remain in RouteStore and are deliberately separate.
type PersonPreferences struct {
	UrgentHousehold bool   `json:"urgentHousehold"`
	PrivateMessages bool   `json:"privateMessages"`
	PrivatePreviews bool   `json:"privatePreviews"`
	LastState       string `json:"lastState,omitempty"`
	LastAt          int64  `json:"lastAt,omitempty"`
}

type PreferencesStore struct {
	Schema int                          `json:"schema"`
	People map[string]PersonPreferences `json:"people"`
}

// Event is the already-composed delivery payload. The Family Board remains the
// source of truth for message text and lifecycle; MessageID/Private let core
// reject a queued board message that was withdrawn before dispatch.
type Event struct {
	PersonID  string
	MessageID string
	Private   bool
	Title     string
	Body      string
	Warning   bool
}

// Person is the narrow household snapshot Notifications needs for CLI status,
// preference visibility, and orphaned-route cleanup.
type Person struct {
	ID    string
	Name  string
	State string
}

type PersonStatus struct {
	ID         string
	Name       string
	State      string
	Configured bool
}

// SendFunc sends through the configured Apprise routes. It must not log raw
// routes, message content, or provider response details.
type SendFunc func(routes []string, title, body string, warning bool) error

// ServiceConfig contains the narrow core handles Notifications needs. The
// package never imports package main or another bounded service.
type ServiceConfig struct {
	Home      string
	ConfigDir string

	NormalizePersonID   func(any) string
	People              func() []Person
	ActivePerson        func(string) bool
	MessageStillCurrent func(Event) bool
	Send                SendFunc
}

// Service owns Apprise-only files and delivery state. The queue and its
// rate-limit/single-flight fields are deliberately not kept on the core app.
type Service struct {
	home      string
	configDir string

	normalizePersonIDFn   func(any) string
	peopleFn              func() []Person
	activePersonFn        func(string) bool
	messageStillCurrentFn func(Event) bool
	sendFn                SendFunc

	mu              sync.Mutex
	queue           chan Event
	recent          []time.Time
	recipientRecent map[string][]time.Time
	inFlight        bool

	// lifecycleMu guards the queue worker lifecycle separately from delivery
	// rate state. Stop is terminal for a process-lifetime service: it rejects
	// new enqueues, drains accepted work, and waits for its worker before
	// returning. The queue intentionally stays open so concurrent callers can
	// never panic by sending on a closed channel.
	lifecycleMu sync.Mutex
	stopCh      chan struct{}
	workerDone  chan struct{}
	started     bool
	stopping    bool
}

func New(cfg ServiceConfig) *Service {
	s := &Service{
		home:                  cfg.Home,
		configDir:             cfg.ConfigDir,
		normalizePersonIDFn:   cfg.NormalizePersonID,
		peopleFn:              cfg.People,
		activePersonFn:        cfg.ActivePerson,
		messageStillCurrentFn: cfg.MessageStillCurrent,
		sendFn:                cfg.Send,
		queue:                 make(chan Event, QueueLimit),
		recipientRecent:       map[string][]time.Time{},
		stopCh:                make(chan struct{}),
	}
	if s.normalizePersonIDFn == nil {
		s.normalizePersonIDFn = defaultPersonID
	}
	if s.sendFn == nil {
		s.sendFn = DefaultSend
	}
	return s
}

func defaultPersonID(value any) string {
	text, _ := value.(string)
	text = strings.TrimSpace(text)
	if len([]rune(text)) > 96 {
		text = string([]rune(text)[:96])
	}
	return strings.ReplaceAll(text, " ", "-")
}

func (s *Service) normalizePersonID(value any) string {
	if s == nil || s.normalizePersonIDFn == nil {
		return defaultPersonID(value)
	}
	return s.normalizePersonIDFn(value)
}

func (s *Service) people() []Person {
	if s == nil || s.peopleFn == nil {
		return []Person{}
	}
	return s.peopleFn()
}

func (s *Service) activePerson(personID string) bool {
	return s != nil && s.activePersonFn != nil && s.activePersonFn(personID)
}

func (s *Service) messageStillCurrent(event Event) bool {
	return s == nil || s.messageStillCurrentFn == nil || s.messageStillCurrentFn(event)
}
