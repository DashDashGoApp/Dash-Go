package notify

import (
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func newTestService(t *testing.T, send SendFunc) *Service {
	t.Helper()
	root := t.TempDir()
	return New(ServiceConfig{
		Home:      filepath.Join(root, "home"),
		ConfigDir: filepath.Join(root, "config"),
		NormalizePersonID: func(value any) string {
			return defaultPersonID(value)
		},
		People: func() []Person {
			return []Person{{ID: "alex", Name: "Alex", State: "active"}, {ID: "sam", Name: "Sam", State: "archived"}}
		},
		ActivePerson: func(id string) bool { return id == "alex" },
		MessageStillCurrent: func(Event) bool {
			return true
		},
		Send: send,
	})
}

func TestPrivateRoutesAndPreferencesRetainPermissions(t *testing.T) {
	s := newTestService(t, nil)
	if err := s.SaveRoutes(RouteStore{Schema: RouteSchema, Enabled: true, Routes: map[string][]string{"alex": {"json://example.invalid/token"}}}); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(s.RoutesFile()); err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("route config mode=%v err=%v", info.Mode(), err)
	}
	if err := s.SavePreferences(PreferencesStore{Schema: PreferencesSchema, People: map[string]PersonPreferences{"alex": {PrivateMessages: true}}}); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(s.PreferencesFile()); err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("preferences mode=%v err=%v", info.Mode(), err)
	}
}

func TestDeliveryRecordsSuccessAndRejectsStaleMessage(t *testing.T) {
	calls := 0
	s := newTestService(t, func(routes []string, title, body string, warning bool) error {
		calls++
		return nil
	})
	if err := s.SaveRoutes(RouteStore{Schema: RouteSchema, Enabled: true, Routes: map[string][]string{"alex": {"json://example.invalid/token"}}}); err != nil {
		t.Fatal(err)
	}
	s.Deliver(Event{PersonID: "alex", Title: "Test", Body: "Body"})
	if calls != 1 || s.PersonPreferences("alex").LastState != "delivered" {
		t.Fatalf("delivery calls=%d state=%q", calls, s.PersonPreferences("alex").LastState)
	}
	s.messageStillCurrentFn = func(Event) bool { return false }
	s.Deliver(Event{PersonID: "alex", MessageID: "withdrawn", Title: "Test", Body: "Body"})
	if calls != 1 {
		t.Fatalf("stale message sent %d times", calls)
	}
}

func TestDeliveryRecordsFailure(t *testing.T) {
	s := newTestService(t, func(routes []string, title, body string, warning bool) error { return errors.New("offline") })
	if err := s.SaveRoutes(RouteStore{Schema: RouteSchema, Enabled: true, Routes: map[string][]string{"alex": {"json://example.invalid/token"}}}); err != nil {
		t.Fatal(err)
	}
	s.Deliver(Event{PersonID: "alex", Title: "Test", Body: "Body"})
	if state := s.PersonPreferences("alex").LastState; state != "failed" {
		t.Fatalf("state=%q", state)
	}
}

func TestConfiguredPeopleAndOrphansKeepArchivedPeople(t *testing.T) {
	s := newTestService(t, nil)
	if err := s.SaveRoutes(RouteStore{Schema: RouteSchema, Enabled: true, Routes: map[string][]string{
		"alex":    {"json://example.invalid/token"},
		"sam":     {"json://example.invalid/token"},
		"removed": {"json://example.invalid/token"},
	}}); err != nil {
		t.Fatal(err)
	}
	people := s.ConfiguredPeople()
	if len(people) != 2 || !people[0].Configured || people[0].Name != "Alex" {
		t.Fatalf("people=%#v", people)
	}
	orphans := s.OrphanRouteIDs()
	if len(orphans) != 1 || orphans[0] != "removed" {
		t.Fatalf("orphans=%#v", orphans)
	}
}

func TestStopDrainsAcceptedDeliveryBeforeReturning(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	s := newTestService(t, func(routes []string, title, body string, warning bool) error {
		calls.Add(1)
		close(started)
		<-release
		return nil
	})
	if err := s.SaveRoutes(RouteStore{Schema: RouteSchema, Enabled: true, Routes: map[string][]string{"alex": {"json://example.invalid/token"}}}); err != nil {
		t.Fatal(err)
	}
	s.Start()
	s.Enqueue(Event{PersonID: "alex", Title: "Test", Body: "Body"})
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("delivery did not start")
	}

	stopped := make(chan struct{})
	go func() {
		s.Stop()
		close(stopped)
	}()
	select {
	case <-stopped:
		t.Fatal("Stop returned before the active delivery completed")
	case <-time.After(25 * time.Millisecond):
	}
	close(release)
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("Stop did not finish the drained delivery")
	}
	if state := s.PersonPreferences("alex").LastState; state != "delivered" {
		t.Fatalf("state after Stop=%q", state)
	}
	if _, err := os.Stat(s.PreferencesFile()); err != nil {
		t.Fatalf("delivery state was not persisted before Stop returned: %v", err)
	}

	// A stopped notifier is terminal: later enqueues produce no send and no
	// new preference write, which makes temporary-directory cleanup safe.
	before := s.PersonPreferences("alex")
	s.Enqueue(Event{PersonID: "alex", Title: "Later", Body: "Ignored"})
	if calls.Load() != 1 {
		t.Fatalf("post-stop enqueue sent %d deliveries", calls.Load())
	}
	after := s.PersonPreferences("alex")
	if after.LastState != before.LastState || after.LastAt != before.LastAt {
		t.Fatalf("post-stop enqueue changed delivery state: before=%#v after=%#v", before, after)
	}
}

func TestStartIsIdempotentAndStopIsRepeatable(t *testing.T) {
	var calls atomic.Int32
	delivered := make(chan struct{}, 1)
	s := newTestService(t, func(routes []string, title, body string, warning bool) error {
		calls.Add(1)
		delivered <- struct{}{}
		return nil
	})
	if err := s.SaveRoutes(RouteStore{Schema: RouteSchema, Enabled: true, Routes: map[string][]string{"alex": {"json://example.invalid/token"}}}); err != nil {
		t.Fatal(err)
	}
	s.Start()
	s.Start()
	s.Enqueue(Event{PersonID: "alex", Title: "Test", Body: "Body"})
	select {
	case <-delivered:
	case <-time.After(time.Second):
		t.Fatal("queued delivery did not complete")
	}
	if calls.Load() != 1 {
		t.Fatalf("expected one worker delivery, got %d", calls.Load())
	}
	s.Stop()
	s.Stop()
}
