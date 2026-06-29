package main

import (
	"os"
	"testing"
	"time"
)

type capturedAppriseDelivery struct {
	title   string
	body    string
	warning bool
}

func newAppriseTestApp(t *testing.T) *app {
	t.Helper()
	return testApp(t)
}

func captureAppriseDeliveries(t *testing.T, a *app) <-chan capturedAppriseDelivery {
	t.Helper()
	deliveries := make(chan capturedAppriseDelivery, 2)
	a.notifyInitMu.Lock()
	if a.notify != nil {
		a.notifyInitMu.Unlock()
		t.Fatal("test notifier must be installed before lazy service construction")
	}
	a.notify = a.newNotifyService(func(routes []string, title, body string, warning bool) error {
		deliveries <- capturedAppriseDelivery{title: title, body: body, warning: warning}
		return nil
	})
	service := a.notify
	a.notifyInitMu.Unlock()
	service.Start()
	// testApp registered TempDir cleanup earlier. Cleanup functions run LIFO,
	// so Stop drains the worker and completes its preference write first.
	t.Cleanup(service.Stop)
	return deliveries
}

func TestApprisePrivateMessageDefaultsToNoPreview(t *testing.T) {
	a := newAppriseTestApp(t)
	deliveries := captureAppriseDeliveries(t, a)
	writePeopleAssignmentFixture(t, a,
		map[string]any{"id": "alex", "name": "Alex", "state": "active"},
		map[string]any{"id": "sam", "name": "Sam", "state": "active"},
	)
	if err := a.saveAppriseRoutes(appriseRouteStore{Schema: appriseRouteSchema, Enabled: true, Routes: map[string][]string{"sam": {"json://example.invalid/token"}}}); err != nil {
		t.Fatal(err)
	}
	if err := a.setApprisePersonPreferences("sam", apprisePersonPreferences{PrivateMessages: true}); err != nil {
		t.Fatal(err)
	}
	a.notifyPrivateFamilyMessage(map[string]any{"scope": "direct", "recipientPersonId": "sam", "senderNameSnapshot": "Alex", "text": "Private detail", "priority": "urgent"})
	select {
	case delivery := <-deliveries:
		if delivery.body != "You have an urgent private message in Dash-Go." || delivery.title != "Dash-Go urgent private message" || !delivery.warning {
			t.Fatalf("private notification leaked content or priority: %#v", delivery)
		}
	case <-time.After(time.Second):
		t.Fatal("private notification was not delivered")
	}
}

func TestApprisePrivatePreviewIsOptIn(t *testing.T) {
	a := newAppriseTestApp(t)
	deliveries := captureAppriseDeliveries(t, a)
	writePeopleAssignmentFixture(t, a,
		map[string]any{"id": "alex", "name": "Alex", "state": "active"},
		map[string]any{"id": "sam", "name": "Sam", "state": "active"},
	)
	if err := a.saveAppriseRoutes(appriseRouteStore{Schema: appriseRouteSchema, Enabled: true, Routes: map[string][]string{"sam": {"json://example.invalid/token"}}}); err != nil {
		t.Fatal(err)
	}
	if err := a.setApprisePersonPreferences("sam", apprisePersonPreferences{PrivateMessages: true, PrivatePreviews: true}); err != nil {
		t.Fatal(err)
	}
	a.notifyPrivateFamilyMessage(map[string]any{"scope": "direct", "recipientPersonId": "sam", "senderNameSnapshot": "Alex", "text": "Bring milk", "priority": "normal"})
	select {
	case delivery := <-deliveries:
		if delivery.body != "From Alex: Bring milk" {
			t.Fatalf("preview=%q", delivery.body)
		}
	case <-time.After(time.Second):
		t.Fatal("private notification was not delivered")
	}
}

func TestAppriseRouteAndPreferencesStayPrivate(t *testing.T) {
	a := newAppriseTestApp(t)
	if err := a.saveAppriseRoutes(appriseRouteStore{Schema: appriseRouteSchema, Enabled: true, Routes: map[string][]string{"sam": {"json://example.invalid/token"}}}); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(a.appriseRoutesFile()); err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("route config mode=%v err=%v", info.Mode(), err)
	}
	if err := a.saveApprisePreferences(apprisePreferencesStore{Schema: apprisePreferencesSchema, People: map[string]apprisePersonPreferences{"sam": {PrivateMessages: true}}}); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(a.apprisePreferencesFile()); err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("preferences mode=%v err=%v", info.Mode(), err)
	}
}
