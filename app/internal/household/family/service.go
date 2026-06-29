// Package family owns the private Family Board document, inbox PIN verifiers,
// and short-lived inbox sessions. It has no dependency on the dashboard core
// or sibling domain services; callers supply the narrow People snapshot/name
// callbacks needed to construct active inbox views.
package family

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

const (
	Schema                    = 3
	ArchiveDays               = 90
	MaxDirectMessagesPerInbox = 400
	InboxVisibleLimit         = 160
	InboxSessionTTL           = 2 * time.Minute
)

type ServiceConfig struct {
	Home       string
	StorePath  string
	PinsPath   string
	Now        func() time.Time
	Token      func() string
	People     func() []map[string]any
	PersonName func(map[string]any) string
}

type Service struct {
	storePath  string
	pinsPath   string
	now        func() time.Time
	token      func() string
	people     func() []map[string]any
	personName func(map[string]any) string

	boardMu  sync.Mutex
	pinsMu   sync.Mutex
	inboxMu  sync.Mutex
	sessions map[string]InboxSession
	failures map[string][]time.Time
}

type InboxSession struct {
	PersonID string
	Exp      time.Time
}

func New(cfg ServiceConfig) *Service {
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	token := cfg.Token
	if token == nil {
		token = randomToken
	}
	people := cfg.People
	if people == nil {
		people = func() []map[string]any { return nil }
	}
	personName := cfg.PersonName
	if personName == nil {
		personName = func(person map[string]any) string { return Text(person["name"], 64) }
	}
	storePath := cfg.StorePath
	if storePath == "" {
		storePath = filepath.Join(cfg.Home, ".dashboard-family-board.json")
	}
	pinsPath := cfg.PinsPath
	if pinsPath == "" {
		pinsPath = filepath.Join(cfg.Home, ".dashboard-family-board-inbox-pins.json")
	}
	return &Service{
		storePath: storePath, pinsPath: pinsPath, now: now, token: token,
		people: people, personName: personName,
		sessions: map[string]InboxSession{}, failures: map[string][]time.Time{},
	}
}

func (s *Service) StorePath() string { return s.storePath }
func (s *Service) PinsPath() string  { return s.pinsPath }
func (s *Service) Now() time.Time    { return s.now().In(time.Local) }

// Lock and Unlock are intentionally narrow adapters for the existing HTTP
// handlers. The state mutex remains inside this service; core never receives
// the underlying map, store path, or session state.
func (s *Service) Lock()   { s.boardMu.Lock() }
func (s *Service) Unlock() { s.boardMu.Unlock() }

// Payload returns the normalized document under the service-owned mutex.
func (s *Service) Payload() map[string]any {
	s.Lock()
	defer s.Unlock()
	return s.PayloadLocked()
}

// PayloadLocked is for a caller already inside Lock/Unlock. It deliberately
// does not persist compatibility normalization; Read performs that durable
// refresh on the GET/status path just as the pre-extraction implementation did.
func (s *Service) PayloadLocked() map[string]any {
	return Normalize(jsonutil.Map(readJSONDefault(s.storePath, Default())), s.Now())
}

func (s *Service) Ensure() error {
	s.Lock()
	defer s.Unlock()
	return s.ensureLocked()
}

func (s *Service) ensureLocked() error {
	st, err := os.Stat(s.storePath)
	if err == nil {
		if !st.Mode().IsRegular() {
			return fmt.Errorf("private Family Message Board store is not a regular file")
		}
		return os.Chmod(s.storePath, 0600)
	}
	if !os.IsNotExist(err) {
		return err
	}
	return s.writeLocked(Default())
}

// Read normalizes expiry/archive state and persists canonical changes once.
func (s *Service) Read() (map[string]any, error) {
	s.Lock()
	defer s.Unlock()
	if err := s.ensureLocked(); err != nil {
		return Default(), err
	}
	raw := jsonutil.Map(readJSONDefault(s.storePath, Default()))
	payload := Normalize(raw, s.Now())
	if !Equal(raw, payload, s.Now()) {
		if err := s.writeLocked(payload); err != nil {
			return payload, err
		}
	}
	return payload, nil
}

func (s *Service) Write(payload map[string]any) error {
	s.Lock()
	defer s.Unlock()
	return s.writeLocked(payload)
}

// WriteLocked is for a caller holding Lock. It retains the existing owner-only
// private-store mode and canonical normalization semantics.
func (s *Service) WriteLocked(payload map[string]any) error {
	return s.writeLocked(payload)
}

func (s *Service) writeLocked(payload map[string]any) error {
	payload = Normalize(payload, s.Now())
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return fileio.WriteAtomic(s.storePath, data, 0600)
}

func readJSONDefault(path string, def any) any {
	raw, err := os.ReadFile(path)
	if err != nil {
		return def
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return def
	}
	return decoded
}

func randomToken() string {
	var raw [24]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return fmt.Sprintf("family-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(raw[:])
}
