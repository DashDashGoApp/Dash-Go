package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPINSetupRouteCannotRotateAnEnabledPIN(t *testing.T) {
	a := testProfileApp(t)
	if _, err := a.setPin("2468", "60"); err != nil {
		t.Fatal(err)
	}
	token := a.issueToken()
	if token == "" || !a.tokenOK(token) {
		t.Fatal("expected an active session for the existing PIN")
	}

	request := postJSONRequest("/api/lock/set", []byte(`{"pin":"9999","timeout":"60"}`))
	request.Header.Set("X-Dashboard-Token", token)
	response := httptest.NewRecorder()
	a.handle(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("credential-rotation setup route status=%d body=%s", response.Code, response.Body.String())
	}
	if !a.verifyPin("2468") || a.verifyPin("9999") {
		t.Fatal("setup route changed an enabled PIN without proving the current PIN")
	}
}

func TestAPIRejectsCrossOriginBrowserRequest(t *testing.T) {
	a := testProfileApp(t)
	request := httptest.NewRequest(http.MethodGet, "http://dashboard.local/api/lock/status", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	request.Header.Set("Origin", "http://other-local-app.invalid")
	request.Header.Set("Sec-Fetch-Site", "cross-site")
	response := httptest.NewRecorder()
	a.handle(response, request)
	if response.Code != http.StatusForbidden || responseError(t, response) != "same-origin API requests only" {
		t.Fatalf("cross-origin response=%d body=%s", response.Code, response.Body.String())
	}
}

func TestAPIAllowsSameOriginBrowserRequest(t *testing.T) {
	a := testProfileApp(t)
	request := httptest.NewRequest(http.MethodGet, "http://dashboard.local/api/lock/status", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	request.Header.Set("Origin", "http://dashboard.local")
	request.Header.Set("Sec-Fetch-Site", "same-origin")
	response := httptest.NewRecorder()
	a.handle(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("same-origin response=%d body=%s", response.Code, response.Body.String())
	}
}
