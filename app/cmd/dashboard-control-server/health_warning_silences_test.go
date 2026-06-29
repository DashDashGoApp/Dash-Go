package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func healthFactNamed(h map[string]any, name string) (healthFact, bool) {
	facts, _ := h["facts"].([]healthFact)
	for _, fact := range facts {
		if fact.Name == name {
			return fact, true
		}
	}
	return healthFact{}, false
}

func TestDefaultsOnlyMessagesDoNotCreateFreshnessFact(t *testing.T) {
	a := testApp(t)
	if err := fileio.WriteJSON(filepath.Join(a.configDir, "message-sources.json"), map[string]any{"enabled": []any{}}); err != nil {
		t.Fatal(err)
	}
	if err := fileio.WriteJSON(a.messageCachePath(), map[string]any{"lastSuccessAt": time.Now().Add(-72 * time.Hour).UnixMilli(), "items": []any{}}); err != nil {
		t.Fatal(err)
	}
	if _, ok := healthFactNamed(a.deviceHealth(), "messages"); ok {
		t.Fatal("defaults-only messages must not create a message freshness fact")
	}
}

func TestEnabledMessageSourceCreatesFreshnessFact(t *testing.T) {
	a := testApp(t)
	if err := fileio.WriteJSON(filepath.Join(a.configDir, "message-sources.json"), map[string]any{"enabled": []any{"jokes"}}); err != nil {
		t.Fatal(err)
	}
	if err := fileio.WriteJSON(a.messageCachePath(), map[string]any{"lastSuccessAt": time.Now().Add(-72 * time.Hour).UnixMilli(), "items": []any{}}); err != nil {
		t.Fatal(err)
	}
	fact, ok := healthFactNamed(a.deviceHealth(), "messages")
	if !ok {
		t.Fatal("enabled message source must create a message freshness fact")
	}
	// The 36-hour footer reminder is intentionally earlier than the Go
	// diagnostic escalation window. This test proves that the selected source
	// creates the canonical data fact without conflating those two thresholds.
	if fact.Tier != "data" || fact.Level != "ok" || fact.Timestamp <= 0 {
		t.Fatalf("unexpected message freshness fact: %#v", fact)
	}
}

func TestHealthWarningSilenceValidationAndExpiry(t *testing.T) {
	a := testApp(t)
	now := time.Date(2030, 3, 4, 5, 6, 7, 0, time.UTC)
	for _, minutes := range []int{15, 60, 240, 720, 1440} {
		active, err := a.setHealthWarningSilence("messages", minutes, now)
		if err != nil {
			t.Fatalf("messages/%d: %v", minutes, err)
		}
		record := jsonutil.Map(active["messages"])
		if got, want := int64(jsonutil.Int(record["until"], 0)), now.Add(time.Duration(minutes)*time.Minute).UnixMilli(); got != want {
			t.Fatalf("messages/%d until=%d want %d", minutes, got, want)
		}
	}
	if err := fileio.WriteJSON(filepath.Join(a.cacheDir, "storage-wear-state.json"), map[string]any{
		"level": "warn", "reason": "current boot kernel log contains storage I/O or filesystem errors", "updated": now.Unix(),
		"readOnly": false, "canary": "ok", "freeKB": 1024 * 1024, "kernelErrorsCurrentBoot": 3,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.setHealthWarningSilence("storage", 15, now); err != nil {
		t.Fatalf("eligible degraded storage notice should be silenceable: %v", err)
	}
	if err := fileio.WriteJSON(filepath.Join(a.cacheDir, "storage-wear-state.json"), map[string]any{"level": "failing", "readOnly": true, "canary": "ok"}); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []struct {
		key     string
		minutes int
	}{{"device", 15}, {"messages", 0}, {"messages", 2}, {"messages", 1441}, {"messages", -1}, {"storage", 15}} {
		if _, err := a.setHealthWarningSilence(invalid.key, invalid.minutes, now); err == nil {
			t.Fatalf("expected %q/%d to be rejected", invalid.key, invalid.minutes)
		}
	}
	if err := fileio.WriteJSON(a.healthWarningSilencesPath(), healthWarningSilenceState{Schema: 1, UpdatedAt: now.Add(-time.Hour).UnixMilli(), Data: map[string]healthWarningSilence{"messages": {Until: now.Add(-time.Minute).UnixMilli()}}}); err != nil {
		t.Fatal(err)
	}
	if active := a.healthWarningSilences(now); len(active) != 0 {
		t.Fatalf("expired silence leaked into health payload: %#v", active)
	}
}

func TestHealthWarningSilenceDoesNotChangeHealthFacts(t *testing.T) {
	a := testApp(t)
	if err := fileio.WriteJSON(filepath.Join(a.configDir, "message-sources.json"), map[string]any{"enabled": []any{"quotes"}}); err != nil {
		t.Fatal(err)
	}
	if err := fileio.WriteJSON(a.messageCachePath(), map[string]any{"lastSuccessAt": time.Now().Add(-145 * time.Hour).UnixMilli(), "items": []any{}}); err != nil {
		t.Fatal(err)
	}
	before, ok := healthFactNamed(a.deviceHealth(), "messages")
	if !ok || before.Level == "ok" {
		t.Fatalf("expected stale messages fact before silence: %#v", before)
	}
	if _, err := a.setHealthWarningSilence("messages", 240, time.Now()); err != nil {
		t.Fatal(err)
	}
	afterHealth := a.deviceHealth()
	after, ok := healthFactNamed(afterHealth, "messages")
	if !ok || after.Level != before.Level {
		t.Fatalf("silence changed message diagnostic fact: before=%#v after=%#v", before, after)
	}
	if silences := jsonutil.Map(afterHealth["warningSilences"]); jsonutil.Map(silences["messages"])["until"] == nil {
		t.Fatalf("active message silence missing from health payload: %#v", afterHealth)
	}
}

func TestHealthWarningSilencePublicEndpointIsNarrow(t *testing.T) {
	a := testApp(t)
	// The silence route must remain usable even when the Control PIN is enabled;
	// verify the comparison against a genuinely protected ordinary mutation.
	if _, err := a.setPin("2468", "60"); err != nil {
		t.Fatal(err)
	}
	if err := fileio.WriteJSON(filepath.Join(a.cacheDir, "storage-wear-state.json"), map[string]any{
		"level": "warn", "reason": "current boot kernel log contains storage I/O or filesystem errors", "updated": time.Now().Unix(),
		"readOnly": false, "canary": "ok", "freeKB": 1024 * 1024, "kernelErrorsCurrentBoot": 3,
	}); err != nil {
		t.Fatal(err)
	}
	post := func(path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "127.0.0.1:12345"
		res := httptest.NewRecorder()
		a.handle(res, req)
		return res
	}

	ok := post("/api/health/warnings/silence", `{"key":"messages","minutes":240}`)
	if ok.Code != http.StatusOK {
		t.Fatalf("valid public silence status=%d body=%s", ok.Code, ok.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(ok.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	until := int64(jsonutil.Int(jsonutil.Map(jsonutil.Map(payload["warningSilences"])["messages"])["until"], 0))
	if until <= time.Now().Add(3*time.Hour).UnixMilli() || until > time.Now().Add(5*time.Hour).UnixMilli() {
		t.Fatalf("server did not own bounded expiry: %d", until)
	}
	legacy := post("/api/health/warnings/silence", `{"key":"storage","hours":1}`)
	if legacy.Code != http.StatusOK {
		t.Fatalf("legacy whole-hour silence status=%d body=%s", legacy.Code, legacy.Body.String())
	}
	bad := post("/api/health/warnings/silence", `{"key":"device","minutes":15}`)
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("generic device silence status=%d want 400", bad.Code)
	}
	locked := post("/api/settings", `{}`)
	if locked.Code != http.StatusUnauthorized {
		t.Fatalf("ordinary privileged endpoint became public: status=%d body=%s", locked.Code, locked.Body.String())
	}
}

func TestMessageProviderBackoffStaysDiagnostic(t *testing.T) {
	a := testApp(t)
	if err := fileio.WriteJSON(filepath.Join(a.configDir, "message-sources.json"), map[string]any{"enabled": []any{"quotes"}}); err != nil {
		t.Fatal(err)
	}
	if err := fileio.WriteJSON(a.messageCachePath(), map[string]any{"lastSuccessAt": time.Now().UnixMilli(), "items": []any{}}); err != nil {
		t.Fatal(err)
	}
	a.noteProviderBackoff("message-quotable", errors.New("lookup api.quotable.io: server misbehaving"))
	health := a.deviceHealth()
	fact, ok := healthFactNamed(health, "provider:message-quotable")
	if !ok || fact.Tier != "diagnostic" || fact.Level != "degraded" {
		t.Fatalf("quote provider failure must remain a diagnostic fact: %#v", health)
	}
	if health["device"] != "ok" || health["data"] != "ok" || health["statusLine"] != "All systems normal" {
		t.Fatalf("quote provider failure leaked into actionable health: %#v", health)
	}
}
