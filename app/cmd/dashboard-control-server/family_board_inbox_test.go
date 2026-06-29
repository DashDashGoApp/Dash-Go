package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func TestFamilyBoardPrivateUrgentDoesNotLeakIntoPublicBoardOrAlert(t *testing.T) {
	now := time.Date(2026, 6, 30, 9, 0, 0, 0, time.Local)
	withFamilyBoardClock(t, now)
	payload := normalizeFamilyBoardPayload(map[string]any{"settings": map[string]any{"showUrgentAlertsOnDashboard": true}, "notes": []any{
		map[string]any{"id": "house", "text": "Call everyone", "scope": "household", "priority": "urgent", "state": "active"},
		map[string]any{"id": "private", "text": "Private urgent detail", "scope": "direct", "priority": "urgent", "senderPersonId": "alex", "senderNameSnapshot": "Alex", "recipientPersonId": "sam", "recipientNameSnapshot": "Sam"},
	}})
	public := familyBoardPublicPayload(payload)
	if got := len(jsonutil.List(public["notes"])); got != 1 {
		t.Fatalf("public board leaked private message: %#v", public)
	}
	if text := strings.Contains(strings.ToLower(fmtSprint(public)), "private urgent detail"); text {
		t.Fatalf("public board contains private text: %#v", public)
	}
	summary := familyBoardSummary(payload)
	if got := jsonutil.Int(summary["urgentCount"], 0); got != 1 {
		t.Fatalf("private urgent message changed household alert count: %#v", summary)
	}
}

func TestFamilyBoardInboxPINVerifierIsPrivateAndRemovable(t *testing.T) {
	a := testProfileApp(t)
	if err := a.setFamilyBoardInboxPIN("sam", "1234"); err != nil {
		t.Fatal(err)
	}
	if !a.verifyFamilyBoardInboxPIN("sam", "1234") || a.verifyFamilyBoardInboxPIN("sam", "9999") {
		t.Fatal("personal inbox verifier did not validate correctly")
	}
	path := a.familyBoardInboxPinsFile()
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("personal inbox verifier mode=%v err=%v", info.Mode(), err)
	}
	data, err := os.ReadFile(path)
	if err != nil || strings.Contains(string(data), "1234") {
		t.Fatalf("personal PIN leaked into verifier file: %q err=%v", data, err)
	}
	if err := a.removeFamilyBoardInboxPIN("sam"); err != nil {
		t.Fatal(err)
	}
	if !a.verifyFamilyBoardInboxPIN("sam", "anything") {
		t.Fatal("removed PIN must leave inbox available until a new PIN is set")
	}
}

func fmtSprint(v any) string {
	return strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(fmtAny(v)), "\n", " "), "\t", " "))
}

func fmtAny(v any) string { return fmt.Sprint(v) }

func TestFamilyBoardUsesOnlyPrivateOwnerOnlyStore(t *testing.T) {
	a := testProfileApp(t)
	retired := filepath.Join(a.configDir, "family-board.json")
	if err := os.WriteFile(retired, []byte(`{"schema":1,"notes":[{"id":"retired","text":"Do not import this message","priority":"normal"}]}`), 0644); err != nil {
		t.Fatal(err)
	}
	payload, err := a.familyBoardReadPayload()
	if err != nil {
		t.Fatal(err)
	}
	if got := len(jsonutil.List(payload["notes"])); got != 0 {
		t.Fatalf("retired config-tree Board store was read: %#v", payload)
	}
	info, err := os.Stat(a.familyBoardFile())
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("private board mode=%v err=%v", info.Mode(), err)
	}
	if _, err := os.Stat(retired); err != nil {
		t.Fatalf("retired config-tree file should remain untouched and denied by HTTP: %v", err)
	}
}

func TestFamilyBoardPrivateArchiveAndDeleteStayRecipientScoped(t *testing.T) {
	now := time.Date(2026, 6, 30, 9, 0, 0, 0, time.Local)
	withFamilyBoardClock(t, now)
	a := testProfileApp(t)
	payload := normalizeFamilyBoardPayload(map[string]any{"notes": []any{
		map[string]any{
			"id": "private-1", "text": "Please call home", "scope": "direct", "priority": "normal",
			"senderPersonId": "alex", "senderNameSnapshot": "Alex",
			"recipientPersonId": "sam", "recipientNameSnapshot": "Sam",
		},
	}})

	active, _ := a.familyBoardInboxPayload(payload, "sam", false)
	if got := len(jsonutil.List(active["inbox"])); got != 1 {
		t.Fatalf("recipient inbox rows=%d want 1: %#v", got, active)
	}

	note := jsonutil.Map(jsonutil.List(payload["notes"])[0])
	note["recipientArchivedAt"] = now.Format(time.RFC3339)
	note["updatedAt"] = now.Format(time.RFC3339)
	payload["notes"] = []any{note}
	archived, _ := a.familyBoardInboxPayload(payload, "sam", false)
	if got := len(jsonutil.List(archived["inbox"])); got != 0 {
		t.Fatalf("archived recipient message remained in inbox: %#v", archived)
	}
	if got := len(jsonutil.List(archived["archive"])); got != 1 {
		t.Fatalf("recipient archive rows=%d want 1: %#v", got, archived)
	}
	if got := jsonutil.Map(jsonutil.List(archived["archive"])[0])["archived"]; got != true {
		t.Fatalf("archive row missing archive marker: %#v", archived)
	}
	sentView, _ := a.familyBoardInboxPayload(payload, "alex", false)
	if got := len(jsonutil.List(sentView["sent"])); got != 1 {
		t.Fatalf("sender history vanished after recipient archive: %#v", payload)
	}
	if _, leaked := jsonutil.Map(jsonutil.List(sentView["sent"])[0])["recipientArchivedAt"]; leaked {
		t.Fatalf("sender response leaked recipient archive metadata: %#v", sentView)
	}

	note["recipientDeletedAt"] = now.Add(time.Minute).Format(time.RFC3339)
	payload["notes"] = []any{note}
	deleted, _ := a.familyBoardInboxPayload(payload, "sam", false)
	if got := len(jsonutil.List(deleted["inbox"])) + len(jsonutil.List(deleted["archive"])); got != 0 {
		t.Fatalf("recipient-deleted message remained visible: %#v", deleted)
	}
	sentView, _ = a.familyBoardInboxPayload(payload, "alex", false)
	if got := len(jsonutil.List(sentView["sent"])); got != 1 {
		t.Fatalf("recipient deletion erased sender history: %#v", payload)
	}
	if got := familyBoardDirectCount(payload, "sam"); got != 0 {
		t.Fatalf("recipient-deleted message still consumed inbox limit: %d", got)
	}
}

func TestFamilyBoardPrivateArchiveRestoreAndDeleteEndpoints(t *testing.T) {
	now := time.Date(2026, 6, 30, 9, 0, 0, 0, time.Local)
	withFamilyBoardClock(t, now)
	a := testProfileApp(t)
	if err := fileio.WriteJSON(a.householdPeopleFile(), map[string]any{"people": []any{
		map[string]any{"id": "alex", "name": "Alex", "state": "active"},
		map[string]any{"id": "sam", "name": "Sam", "state": "active"},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := fileio.WriteJSON(a.familyBoardFile(), map[string]any{"notes": []any{
		map[string]any{
			"id": "private-1", "text": "Please call home", "scope": "direct", "priority": "normal",
			"senderPersonId": "alex", "senderNameSnapshot": "Alex",
			"recipientPersonId": "sam", "recipientNameSnapshot": "Sam",
		},
	}}); err != nil {
		t.Fatal(err)
	}
	samsToken := a.issueFamilyBoardInboxSession("sam")
	alexToken := a.issueFamilyBoardInboxSession("alex")
	mutate := func(token, action string, want int) map[string]any {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/api/family-board/messages/private-1/"+action, nil)
		req.Header.Set("X-DashGo-Inbox-Token", token)
		rr := httptest.NewRecorder()
		if !a.familyBoardDirectMutate(rr, req, req.URL.Path, map[string]any{}) {
			t.Fatalf("%s was not handled", action)
		}
		if rr.Code != want {
			t.Fatalf("%s status=%d want %d body=%s", action, rr.Code, want, rr.Body.String())
		}
		out := map[string]any{}
		if rr.Code == http.StatusOK {
			if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
				t.Fatalf("decode %s: %v", action, err)
			}
		}
		return out
	}

	archive := mutate(samsToken, "archive", http.StatusOK)
	if got := len(jsonutil.List(archive["archive"])); got != 1 {
		t.Fatalf("archive response missing private archive row: %#v", archive)
	}
	if got := len(jsonutil.List(archive["inbox"])); got != 0 {
		t.Fatalf("archive response retained active inbox row: %#v", archive)
	}
	mutate(alexToken, "delete", http.StatusForbidden)

	restored := mutate(samsToken, "restore", http.StatusOK)
	if got := len(jsonutil.List(restored["inbox"])); got != 1 {
		t.Fatalf("restore response missing inbox row: %#v", restored)
	}
	if got := len(jsonutil.List(restored["archive"])); got != 0 {
		t.Fatalf("restore response retained archive row: %#v", restored)
	}

	mutate(samsToken, "archive", http.StatusOK)
	deleted := mutate(samsToken, "delete", http.StatusOK)
	if got := len(jsonutil.List(deleted["archive"])); got != 0 {
		t.Fatalf("deleted message remained in recipient archive: %#v", deleted)
	}
	payload, err := a.familyBoardReadPayload()
	if err != nil {
		t.Fatal(err)
	}
	_, note := familyBoardFind(payload, "private-1")
	if note == nil || familyBoardStamp(note["recipientDeletedAt"]) == "" {
		t.Fatalf("recipient-scoped deletion did not persist: %#v", note)
	}
	senderView, _ := a.familyBoardInboxPayload(payload, "alex", false)
	if got := len(jsonutil.List(senderView["sent"])); got != 1 {
		t.Fatalf("recipient delete removed sender Sent history: %#v", senderView)
	}
	if _, leaked := jsonutil.Map(jsonutil.List(senderView["sent"])[0])["recipientDeletedAt"]; leaked {
		t.Fatalf("sender response leaked recipient deletion metadata: %#v", senderView)
	}
}
