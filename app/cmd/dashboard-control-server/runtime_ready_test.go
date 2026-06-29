package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRuntimeReadyIsPublicAndVersionBound(t *testing.T) {
	a := &app{releaseVersion: "1.4.1-beta.19"}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/ready", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	a.handle(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", w.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["goServer"] != true || got["version"] != "1.4.1-beta.19" {
		t.Fatalf("unexpected readiness payload: %#v", got)
	}
}

func TestRuntimeReadyDoesNotRequireControlToken(t *testing.T) {
	a := &app{releaseVersion: "1.4.1-beta.19"}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/ready", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	a.handle(w, req)
	if w.Code == http.StatusUnauthorized {
		t.Fatal("runtime readiness endpoint must stay available before kiosk authentication")
	}
}
