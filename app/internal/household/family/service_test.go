package family

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func testService(home string, now *time.Time) *Service {
	return New(ServiceConfig{
		Home:  home,
		Now:   func() time.Time { return *now },
		Token: func() string { return "family-test-token" },
		People: func() []map[string]any {
			return []map[string]any{
				{"id": "sam", "name": "Sam", "state": "active"},
				{"id": "jordan", "name": "Jordan", "state": "active"},
				{"id": "former", "name": "Former", "state": "archived"},
			}
		},
	})
}

func TestReadCanonicalizesPrivateStoreWithOwnerOnlyMode(t *testing.T) {
	now := time.Date(2026, 6, 27, 10, 0, 0, 0, time.Local)
	svc := testService(t.TempDir(), &now)
	raw := []byte(`{"schema":3,"settings":{"showPinnedOnDashboard":false},"notes":[{"id":"public","text":"Hello","scope":"household","priority":"normal","state":"active","createdAt":"2026-06-27T10:00:00Z","updatedAt":"2026-06-27T10:00:00Z"}]}`)
	if err := os.WriteFile(svc.StorePath(), raw, 0644); err != nil {
		t.Fatalf("seed private store: %v", err)
	}

	payload, err := svc.Read()
	if err != nil {
		t.Fatalf("read private store: %v", err)
	}
	if got := jsonutil.Map(payload["settings"])["showUrgentAlertsOnDashboard"]; got != false {
		t.Fatalf("canonical settings = %#v, want false", got)
	}
	if got := len(jsonutil.List(payload["notes"])); got != 1 {
		t.Fatalf("canonical note count = %d, want 1", got)
	}
	info, err := os.Stat(svc.StorePath())
	if err != nil {
		t.Fatalf("stat private store: %v", err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("private store mode = %o, want 0600", got)
	}
}

func TestInboxPINRemovalRevokesSessions(t *testing.T) {
	now := time.Date(2026, 6, 27, 10, 0, 0, 0, time.Local)
	svc := testService(t.TempDir(), &now)
	if err := svc.SetPIN("sam", "1234"); err != nil {
		t.Fatalf("set PIN: %v", err)
	}
	if !svc.PinConfigured("sam") || !svc.VerifyPIN("sam", "1234") || svc.VerifyPIN("sam", "0000") {
		t.Fatal("PIN verifier did not preserve expected behavior")
	}
	info, err := os.Stat(svc.PinsPath())
	if err != nil {
		t.Fatalf("stat PIN store: %v", err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("PIN store mode = %o, want 0600", got)
	}
	token := svc.IssueSession("sam")
	if !svc.SessionOK(token, "sam") {
		t.Fatal("new inbox session should be valid")
	}
	if err := svc.RemovePIN("sam"); err != nil {
		t.Fatalf("remove PIN: %v", err)
	}
	if svc.PinConfigured("sam") || svc.SessionOK(token, "sam") {
		t.Fatal("PIN removal must remove verifier and revoke active inbox sessions")
	}
}

func TestInboxViewsKeepPrivateMessagesOutOfPublicSummary(t *testing.T) {
	now := time.Date(2026, 6, 27, 10, 0, 0, 0, time.Local)
	svc := testService(t.TempDir(), &now)
	payload := Default()
	payload["notes"] = []any{
		map[string]any{"id": "private", "scope": "direct", "text": "For Jordan", "priority": "urgent", "state": "active", "createdAt": now.Format(time.RFC3339), "updatedAt": now.Format(time.RFC3339), "senderPersonId": "sam", "senderNameSnapshot": "Sam", "recipientPersonId": "jordan", "recipientNameSnapshot": "Jordan"},
		map[string]any{"id": "public", "scope": "household", "text": "For everyone", "priority": "urgent", "state": "active", "createdAt": now.Format(time.RFC3339), "updatedAt": now.Format(time.RFC3339)},
	}
	payload = Normalize(payload, now)
	view, changed := svc.InboxPayload(payload, "jordan", true)
	if !changed {
		t.Fatal("opening an unread direct inbox must mark its message read")
	}
	if got := len(jsonutil.List(view["inbox"])); got != 1 {
		t.Fatalf("direct inbox count = %d, want 1", got)
	}
	if got := PublicPayload(payload); len(jsonutil.List(got["notes"])) != 1 {
		t.Fatalf("public Family Board payload leaked private message: %#v", got)
	}
	if got := jsonutil.Int(Summary(payload)["urgentCount"], 0); got != 1 {
		t.Fatalf("private urgent message changed dashboard summary count to %d", got)
	}
	if err := svc.Write(payload); err != nil {
		t.Fatalf("write private message state: %v", err)
	}
	if !svc.MessageStillCurrent("private", "jordan", true) {
		t.Fatal("active private message should remain available for current recipient dispatch checks")
	}
	if svc.MessageStillCurrent("private", "sam", true) {
		t.Fatal("private message must not authorize a different recipient")
	}
	if filepath.Dir(svc.StorePath()) == "" {
		t.Fatal("private store path must be configured")
	}
}

func TestFamilyBoardMutationsKeepPrivateScopeAndPersistence(t *testing.T) {
	now := time.Date(2026, 6, 27, 10, 0, 0, 0, time.Local)
	svc := testService(t.TempDir(), &now)
	payload, urgent, actionErr := svc.MutateHousehold("add", map[string]any{"text": "Call home", "priority": "urgent", "expiration": map[string]any{"kind": "none"}})
	if actionErr != nil {
		t.Fatalf("add household note: %v", actionErr)
	}
	if urgent == nil || len(jsonutil.List(payload["notes"])) != 1 {
		t.Fatalf("urgent household mutation did not return expected durable payload: %#v", payload)
	}
	now = now.Add(time.Nanosecond)
	view, note, actionErr := svc.SendDirect("sam", map[string]any{"recipientPersonId": "jordan", "text": "Private", "priority": "urgent"})
	if actionErr != nil {
		t.Fatalf("send direct message: %v", actionErr)
	}
	if note == nil || len(jsonutil.List(view["sent"])) != 1 {
		t.Fatalf("direct send did not return sender view: %#v", view)
	}
	if _, actionErr = svc.MutateDirect("sam", String(note["id"]), "archive"); actionErr == nil || actionErr.Status != 403 {
		t.Fatalf("sender archive error = %#v, want recipient-only 403", actionErr)
	}
	if _, actionErr = svc.MutateDirect("jordan", String(note["id"]), "archive"); actionErr != nil {
		t.Fatalf("recipient archive: %v", actionErr)
	}
	if svc.MessageStillCurrent(String(note["id"]), "jordan", true) != true {
		t.Fatal("recipient archive must not invalidate current private delivery freshness")
	}
}
