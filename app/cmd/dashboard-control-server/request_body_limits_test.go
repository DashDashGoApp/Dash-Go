package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func postJSONRequest(path string, body []byte) *http.Request {
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:12345"
	return req
}

func responseError(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("response is not JSON: %v (%s)", err, w.Body.String())
	}
	return payload["error"].(string)
}

func TestReadBodyRejectsAnnouncedAndChunkedOversizeWithoutDecoding(t *testing.T) {
	a := testProfileApp(t)
	announced := postJSONRequest("/api/settings", []byte(strings.Repeat("x", maxJSONRequestBodyBytes+1)))
	if _, err := a.readBody(announced); !errors.Is(err, errRequestBodyTooLarge) {
		t.Fatalf("announced oversized body error=%v", err)
	}

	chunkedPayload := []byte(`{"value":"` + strings.Repeat("x", maxJSONRequestBodyBytes) + `"}`)
	chunked := httptest.NewRequest(http.MethodPost, "/api/settings", nil)
	chunked.Body = io.NopCloser(bytes.NewReader(chunkedPayload))
	chunked.ContentLength = -1
	if _, err := a.readBody(chunked); !errors.Is(err, errRequestBodyTooLarge) {
		t.Fatalf("chunked oversized body error=%v", err)
	}
}

func TestReadBodyAllowsNearLimitNormalPayload(t *testing.T) {
	a := testProfileApp(t)
	body := map[string]any{}
	for i := 0; i < 15; i++ {
		body["field"+string(rune('a'+i))] = strings.Repeat("x", maxJSONRequestStringRunes)
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) > maxJSONRequestBodyBytes {
		t.Fatalf("fixture is not within the request limit: %d", len(encoded))
	}
	decoded, err := a.readBody(postJSONRequest("/api/settings", encoded))
	if err != nil {
		t.Fatalf("near-limit normal payload rejected: %v", err)
	}
	if len(decoded) != len(body) {
		t.Fatalf("decoded fields=%d want %d", len(decoded), len(body))
	}
}

func TestRequestLimitsPreserveLoopbackAndPreventDurableWrite(t *testing.T) {
	a := testProfileApp(t)

	oversized := postJSONRequest("/api/settings", []byte(strings.Repeat("x", maxJSONRequestBodyBytes+1)))
	tooLarge := httptest.NewRecorder()
	a.handle(tooLarge, oversized)
	if tooLarge.Code != http.StatusRequestEntityTooLarge || responseError(t, tooLarge) != "request body too large" {
		t.Fatalf("oversized response=%d %s", tooLarge.Code, tooLarge.Body.String())
	}

	tooMany := map[string]any{}
	for i := 0; i <= maxJSONRequestObjectFields; i++ {
		tooMany["field"+string(rune('a'+i))] = true
	}
	encoded, err := json.Marshal(tooMany)
	if err != nil {
		t.Fatal(err)
	}
	limited := httptest.NewRecorder()
	a.handle(limited, postJSONRequest("/api/settings", encoded))
	if limited.Code != http.StatusBadRequest || responseError(t, limited) != "request fields exceed supported limits" {
		t.Fatalf("field-limited response=%d %s", limited.Code, limited.Body.String())
	}
	if _, err := os.Stat(a.settingsFile); !os.IsNotExist(err) {
		t.Fatalf("rejected request created settings: %v", err)
	}

	nonLoopback := postJSONRequest("/api/settings", []byte(strings.Repeat("x", maxJSONRequestBodyBytes+1)))
	nonLoopback.RemoteAddr = "203.0.113.9:54321"
	blocked := httptest.NewRecorder()
	a.handle(blocked, nonLoopback)
	if blocked.Code != http.StatusForbidden || responseError(t, blocked) != "loopback only" {
		t.Fatalf("non-loopback response=%d %s", blocked.Code, blocked.Body.String())
	}
}

func TestChalkboardPayloadLimitsKeepNormalSaveWorking(t *testing.T) {
	a := testProfileApp(t)
	tooMany := make([]any, maxChalkboardStrokes+1)
	for i := range tooMany {
		tooMany[i] = map[string]any{"id": i + 1, "pts": []any{[]any{0, 0}}}
	}
	encoded, err := json.Marshal(map[string]any{"board": "dark", "strokes": tooMany})
	if err != nil {
		t.Fatal(err)
	}
	failed := httptest.NewRecorder()
	a.handle(failed, postJSONRequest("/api/chalkboard", encoded))
	if failed.Code != http.StatusBadRequest || responseError(t, failed) != "request fields exceed supported limits" {
		t.Fatalf("oversized chalkboard response=%d %s", failed.Code, failed.Body.String())
	}
	chalkboardPath := filepath.Join(a.configDir, "chalkboard.json")
	if _, err := os.Stat(chalkboardPath); !os.IsNotExist(err) {
		t.Fatalf("rejected chalkboard payload persisted: %v", err)
	}

	normal := []byte(`{"board":"dark","strokes":[{"id":1,"tool":"pen","pts":[[1,2],[3,4]]}]}`)
	ok := httptest.NewRecorder()
	a.handle(ok, postJSONRequest("/api/chalkboard", normal))
	if ok.Code != http.StatusOK {
		t.Fatalf("normal chalkboard save=%d %s", ok.Code, ok.Body.String())
	}
	if _, err := os.Stat(chalkboardPath); err != nil {
		t.Fatalf("normal chalkboard save was not persisted: %v", err)
	}
}

func TestProtectedNormalPostStillReturnsLocked(t *testing.T) {
	a := testProfileApp(t)
	if _, err := a.setPin("2468", "60"); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	a.handle(w, postJSONRequest("/api/theme", []byte(`{"theme":"paper"}`)))
	if w.Code != http.StatusUnauthorized || responseError(t, w) != "locked" {
		t.Fatalf("protected normal request response=%d %s", w.Code, w.Body.String())
	}
}
