package notify

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
)

func (s *Service) Dir() string {
	return filepath.Join(s.home, ".config", "dash-go", "apprise")
}

func (s *Service) RoutesFile() string {
	return filepath.Join(s.Dir(), "routes.json")
}

func (s *Service) PreferencesFile() string {
	return filepath.Join(s.configDir, "notification-preferences.json")
}

func defaultRoutes() RouteStore {
	return RouteStore{Schema: RouteSchema, Enabled: true, Routes: map[string][]string{}}
}

func defaultPreferences() PreferencesStore {
	return PreferencesStore{Schema: PreferencesSchema, People: map[string]PersonPreferences{}}
}

func (s *Service) normalizeRoutes(raw RouteStore) RouteStore {
	out := defaultRoutes()
	out.Enabled = raw.Enabled
	if raw.Schema == 0 {
		out.Enabled = true
	}
	for personID, rows := range raw.Routes {
		personID = s.normalizePersonID(personID)
		if personID == "" {
			continue
		}
		seen := map[string]bool{}
		for _, route := range rows {
			route = strings.TrimSpace(route)
			if route == "" || len(route) > MaxRouteLength || seen[route] {
				continue
			}
			seen[route] = true
			out.Routes[personID] = append(out.Routes[personID], route)
			if len(out.Routes[personID]) >= MaxRoutesPerPerson {
				break
			}
		}
	}
	return out
}

func (s *Service) normalizePreferences(raw PreferencesStore) PreferencesStore {
	out := defaultPreferences()
	for personID, pref := range raw.People {
		personID = s.normalizePersonID(personID)
		if personID == "" {
			continue
		}
		pref.LastState = strings.TrimSpace(pref.LastState)
		if pref.LastState != "" && pref.LastState != "ready" && pref.LastState != "delivered" && pref.LastState != "failed" && pref.LastState != "rate-limited" && pref.LastState != "disabled" && pref.LastState != "timeout" {
			pref.LastState = ""
		}
		if pref.LastAt < 0 {
			pref.LastAt = 0
		}
		out.People[personID] = pref
	}
	return out
}

func readPrivateJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func writePrivateJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return fileio.WriteAtomic(path, data, 0600)
}

func (s *Service) Routes() RouteStore {
	var raw RouteStore
	if err := readPrivateJSON(s.RoutesFile(), &raw); err != nil {
		return defaultRoutes()
	}
	return s.normalizeRoutes(raw)
}

func (s *Service) Preferences() PreferencesStore {
	var raw PreferencesStore
	if err := readPrivateJSON(s.PreferencesFile(), &raw); err != nil {
		return defaultPreferences()
	}
	return s.normalizePreferences(raw)
}

func (s *Service) SaveRoutes(value RouteStore) error {
	if err := os.MkdirAll(s.Dir(), 0700); err != nil {
		return err
	}
	if err := os.Chmod(s.Dir(), 0700); err != nil {
		return err
	}
	return writePrivateJSON(s.RoutesFile(), s.normalizeRoutes(value))
}

func (s *Service) SavePreferences(value PreferencesStore) error {
	return writePrivateJSON(s.PreferencesFile(), s.normalizePreferences(value))
}

func (s *Service) RoutesForPerson(personID string) []string {
	routes := s.Routes()
	if !routes.Enabled {
		return nil
	}
	return append([]string{}, routes.Routes[s.normalizePersonID(personID)]...)
}

func (s *Service) ConfiguredForPerson(personID string) bool {
	return len(s.RoutesForPerson(personID)) > 0
}

func (s *Service) PersonPreferences(personID string) PersonPreferences {
	return s.Preferences().People[s.normalizePersonID(personID)]
}

func (s *Service) PersonControlStatus(personID string) map[string]any {
	routes := s.Routes()
	pref := s.PersonPreferences(personID)
	return map[string]any{
		"routeConfigured": len(routes.Routes[s.normalizePersonID(personID)]) > 0,
		"deliveryEnabled": routes.Enabled,
		"urgentHousehold": pref.UrgentHousehold,
		"privateMessages": pref.PrivateMessages,
		"privatePreviews": pref.PrivatePreviews,
		"lastState":       pref.LastState,
		"lastAt":          pref.LastAt,
	}
}

func (s *Service) SetPersonPreferences(personID string, pref PersonPreferences) error {
	personID = s.normalizePersonID(personID)
	store := s.Preferences()
	if !s.ConfiguredForPerson(personID) {
		return os.ErrNotExist
	}
	pref.PrivatePreviews = pref.PrivateMessages && pref.PrivatePreviews
	pref.LastState = store.People[personID].LastState
	pref.LastAt = store.People[personID].LastAt
	store.People[personID] = pref
	return s.SavePreferences(store)
}

func (s *Service) RecordDeliveryState(personID, state string) {
	personID = s.normalizePersonID(personID)
	if personID == "" {
		return
	}
	store := s.Preferences()
	pref := store.People[personID]
	pref.LastState, pref.LastAt = state, time.Now().Unix()
	store.People[personID] = pref
	// Never include a destination or message payload in logs.
	_ = s.SavePreferences(store)
}

func (s *Service) ConfiguredPeople() []PersonStatus {
	routes := s.Routes()
	people := []PersonStatus{}
	for _, person := range s.people() {
		id := s.normalizePersonID(person.ID)
		if id == "" {
			continue
		}
		people = append(people, PersonStatus{
			ID:         id,
			Name:       strings.TrimSpace(person.Name),
			State:      person.State,
			Configured: len(routes.Routes[id]) > 0,
		})
	}
	slices.SortStableFunc(people, func(left, right PersonStatus) int {
		return strings.Compare(strings.ToLower(left.Name), strings.ToLower(right.Name))
	})
	return people
}

// OrphanRouteIDs identifies private routes whose People record was permanently
// removed. Archived People deliberately remain known so a restored person can
// regain their existing route without re-entry.
func (s *Service) OrphanRouteIDs() []string {
	routes := s.Routes()
	known := map[string]bool{}
	for _, person := range s.people() {
		if id := s.normalizePersonID(person.ID); id != "" {
			known[id] = true
		}
	}
	orphans := []string{}
	for id, values := range routes.Routes {
		if len(values) > 0 && !known[id] {
			orphans = append(orphans, id)
		}
	}
	slices.Sort(orphans)
	return orphans
}

func (s *Service) RemoveOrphanRoutes() (int, error) {
	store := s.Routes()
	orphans := s.OrphanRouteIDs()
	if len(orphans) == 0 {
		return 0, nil
	}
	for _, id := range orphans {
		delete(store.Routes, id)
	}
	if err := s.SaveRoutes(store); err != nil {
		return 0, err
	}
	prefs := s.Preferences()
	for _, id := range orphans {
		delete(prefs.People, id)
	}
	if err := s.SavePreferences(prefs); err != nil {
		return 0, err
	}
	return len(orphans), nil
}

// RestoreRoutes restores only the private route document. Orphaned stable-ID
// routes are retained intentionally so a restored/remapped household member
// does not cause secret endpoints to be silently discarded.
func (s *Service) RestoreRoutes(stage string) error {
	data, err := os.ReadFile(filepath.Join(stage, "secrets", "apprise-routes.json"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var raw RouteStore
	if err := json.Unmarshal(data, &raw); err != nil {
		return os.ErrInvalid
	}
	return s.SaveRoutes(s.normalizeRoutes(raw))
}

func (s *Service) RemoveRoutesConfig() error {
	if err := os.Remove(s.RoutesFile()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
