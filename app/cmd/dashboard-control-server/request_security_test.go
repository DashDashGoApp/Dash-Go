package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func assertLimiterAvailable(t *testing.T, limiter *operationLimiter, path string) {
	t.Helper()
	lease, wait, ok := limiter.acquire(path)
	if !ok {
		t.Fatalf("operation limiter for %s unexpectedly blocked for %v", path, wait)
	}
	lease.Finish()
}

func TestOperationLeaseWithoutStartDoesNotCreateCooldown(t *testing.T) {
	limiter := newOperationLimiter()
	lease, wait, ok := limiter.acquire("/api/backup")
	if !ok {
		t.Fatalf("initial lease blocked for %v", wait)
	}
	lease.Finish()
	assertLimiterAvailable(t, limiter, "/api/backup")
}

func TestStartedOperationLeaseCreatesCooldown(t *testing.T) {
	limiter := newOperationLimiter()
	lease, wait, ok := limiter.acquire("/api/backup")
	if !ok {
		t.Fatalf("initial lease blocked for %v", wait)
	}
	lease.Start()
	lease.Finish()
	if _, wait, ok := limiter.acquire("/api/backup"); ok || wait <= 0 {
		t.Fatalf("started operation did not create a cooldown: ok=%v wait=%v", ok, wait)
	}
}

func TestMalformedRequestDoesNotConsumeOperationCooldown(t *testing.T) {
	a := testProfileApp(t)
	a.operationLimiter = newOperationLimiter()
	request := postJSONRequest("/api/backup", []byte(`{"broken"`))
	response := httptest.NewRecorder()
	a.handle(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("malformed request status=%d body=%s", response.Code, response.Body.String())
	}
	assertLimiterAvailable(t, a.operationLimiter, "/api/backup")
}

func TestUnauthorizedRequestDoesNotConsumeOperationCooldown(t *testing.T) {
	a := testProfileApp(t)
	a.operationLimiter = newOperationLimiter()
	if _, err := a.setPin("2468", "60"); err != nil {
		t.Fatal(err)
	}
	request := postJSONRequest("/api/backup", []byte(`{}`))
	response := httptest.NewRecorder()
	a.handle(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized request status=%d body=%s", response.Code, response.Body.String())
	}
	assertLimiterAvailable(t, a.operationLimiter, "/api/backup")
}

func TestInvalidRestoreDoesNotConsumeOperationCooldown(t *testing.T) {
	a := testProfileApp(t)
	a.operationLimiter = newOperationLimiter()
	if _, err := a.setPin("2468", "60"); err != nil {
		t.Fatal(err)
	}
	token := a.issueToken()
	request := postJSONRequest("/api/backup/restore", []byte(`{"name":"missing-backup"}`))
	request.Header.Set("X-Dashboard-Token", token)
	response := httptest.NewRecorder()
	a.handle(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid restore status=%d body=%s", response.Code, response.Body.String())
	}
	assertLimiterAvailable(t, a.operationLimiter, "/api/backup/restore")
}
