// Package todo owns Dash-Go's local-first Microsoft To Do integration and
// Grocery Memory catalog. It holds durable token/list/task/archive state,
// Graph transport, bounded queueing and inbound reconciliation, per-list
// synchronization locks, and the runtime-only scheduler/auth state. Core
// supplies only narrow Settings, People, and event-delivery callbacks.
package todo

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type ServiceConfig struct {
	TodoDir          string
	TokenFile        string
	LoadSettings     func() map[string]any
	UpdateSettings   func(func(map[string]any)) (map[string]any, error)
	MutateSettings   func(func(map[string]any) error) (map[string]any, error)
	PeoplePayload    func() map[string]any
	AssignmentLookup func(string) (map[string]any, bool)
	ActiveAssignment func(string) (map[string]any, bool)
	PersonName       func(map[string]any) string
	Emit             func(map[string]any)
	Now              func() time.Time
}

type Service struct {
	todoDir                  string
	todoTokenFile            string
	loadSettingsFn           func() map[string]any
	updateSettingsFn         func(func(map[string]any)) (map[string]any, error)
	mutateSettingsFn         func(func(map[string]any) error) (map[string]any, error)
	peoplePayloadFn          func() map[string]any
	assignmentLookupFn       func(string) (map[string]any, bool)
	activeAssignmentFn       func(string) (map[string]any, bool)
	personNameFn             func(map[string]any) string
	emitFn                   func(map[string]any)
	nowFn                    func() time.Time
	todoMu                   sync.Mutex
	todoCloudMu              sync.Mutex
	todoInboundMu            sync.Mutex
	todoListLocksMu          sync.Mutex
	todoMigrationMu          sync.Mutex
	todoArchiveMu            sync.Mutex
	todoDraining             map[string]bool
	todoListLocks            map[string]*sync.Mutex
	todoInboundRunning       bool
	todoInboundQueued        bool
	todoInboundQueuedLists   map[string]bool
	todoInboundQueuedAt      time.Time
	todoInboundCoalesced     int
	todoInboundFailures      int
	todoInboundBackoffUntil  time.Time
	todoInboundLastAt        int64
	todoInboundLastError     string
	todoInboundLastDuration  int64
	todoInboundLastQueueWait int64
	todoInboundWake          chan struct{}
	todoManualSyncUntil      map[string]time.Time
	todoAuthCancel           func()
	todoAuthState            todoAuthPending
}

func New(config ServiceConfig) *Service {
	if config.LoadSettings == nil {
		config.LoadSettings = func() map[string]any { return map[string]any{} }
	}
	if config.UpdateSettings == nil {
		config.UpdateSettings = func(func(map[string]any)) (map[string]any, error) { return map[string]any{}, nil }
	}
	if config.MutateSettings == nil {
		config.MutateSettings = func(func(map[string]any) error) (map[string]any, error) { return map[string]any{}, nil }
	}
	if config.PeoplePayload == nil {
		config.PeoplePayload = func() map[string]any { return map[string]any{} }
	}
	if config.AssignmentLookup == nil {
		config.AssignmentLookup = func(string) (map[string]any, bool) { return nil, false }
	}
	if config.ActiveAssignment == nil {
		config.ActiveAssignment = func(string) (map[string]any, bool) { return nil, false }
	}
	if config.PersonName == nil {
		config.PersonName = func(map[string]any) string { return "" }
	}
	if config.Emit == nil {
		config.Emit = func(map[string]any) {}
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	return &Service{
		todoDir: config.TodoDir, todoTokenFile: config.TokenFile,
		loadSettingsFn: config.LoadSettings, updateSettingsFn: config.UpdateSettings, mutateSettingsFn: config.MutateSettings,
		peoplePayloadFn: config.PeoplePayload, assignmentLookupFn: config.AssignmentLookup, activeAssignmentFn: config.ActiveAssignment,
		personNameFn: config.PersonName, emitFn: config.Emit, nowFn: config.Now,
		todoDraining: map[string]bool{}, todoListLocks: map[string]*sync.Mutex{}, todoInboundQueuedLists: map[string]bool{},
		todoInboundWake: make(chan struct{}, 1), todoManualSyncUntil: map[string]time.Time{},
	}
}

func (a *Service) loadSettings() map[string]any { return a.loadSettingsFn() }
func (a *Service) updateSettings(mut func(map[string]any)) (map[string]any, error) {
	return a.updateSettingsFn(mut)
}
func (a *Service) mutateSettings(mut func(map[string]any) error) (map[string]any, error) {
	return a.mutateSettingsFn(mut)
}
func (a *Service) householdPeoplePayload() map[string]any { return a.peoplePayloadFn() }
func (a *Service) householdPeopleAssignmentLookup(id string) (map[string]any, bool) {
	return a.assignmentLookupFn(id)
}
func (a *Service) householdPeopleActiveAssignment(id string) (map[string]any, bool) {
	return a.activeAssignmentFn(id)
}
func (a *Service) householdPersonAssignmentName(person map[string]any) string {
	return a.personNameFn(person)
}
func (a *Service) todoEmit(payload map[string]any) { a.emitFn(payload) }
func clamp(value, lower, upper int) int {
	if value < lower {
		return lower
	}
	if value > upper {
		return upper
	}
	return value
}
func todoText(value any, limit int) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	runes := []rune(text)
	if len(runes) > limit {
		return string(runes[:limit])
	}
	return text
}
func todoID(value any) string         { return strings.ReplaceAll(todoText(value, 96), " ", "-") }
func compareText(left, right any) int { return strings.Compare(fmt.Sprint(left), fmt.Sprint(right)) }
func compareFoldedText(left, right any) int {
	return strings.Compare(strings.ToLower(fmt.Sprint(left)), strings.ToLower(fmt.Sprint(right)))
}
func compareIntsDescending(left, right int) int {
	if left == right {
		return 0
	}
	if left > right {
		return -1
	}
	return 1
}
func compareInt64sDescending(left, right int64) int {
	if left == right {
		return 0
	}
	if left > right {
		return -1
	}
	return 1
}
func compareBoolTrueFirst(left, right bool) int {
	if left == right {
		return 0
	}
	if left {
		return -1
	}
	return 1
}
func compareBoolFalseFirst(left, right bool) int {
	if left == right {
		return 0
	}
	if !left {
		return -1
	}
	return 1
}
