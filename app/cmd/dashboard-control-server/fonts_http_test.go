package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestFontLooksValidRejectsHTMLAndAcceptsTTFHeader(t *testing.T) {
	d := t.TempDir()
	good := filepath.Join(d, "good.ttf")
	bad := filepath.Join(d, "bad.ttf")
	b := make([]byte, 4096)
	b[0] = 0
	b[1] = 1
	b[2] = 0
	b[3] = 0
	if err := os.WriteFile(good, b, 0644); err != nil {
		t.Fatal(err)
	}
	if !fontLooksValid(good) {
		t.Fatal("valid ttf header rejected")
	}
	if err := os.WriteFile(bad, []byte("<!doctype html>"), 0644); err != nil {
		t.Fatal(err)
	}
	if fontLooksValid(bad) {
		t.Fatal("html accepted as font")
	}
}
func TestRuntimeFontStatesAreBounded(t *testing.T) {
	a := doctorDataTestApp(t)
	a.fontsDir = filepath.Join(a.dash, "fonts")
	if got := a.runtimeFontState("system"); got != "system" {
		t.Fatalf("system=%s", got)
	}
	if got := a.runtimeFontState("rounded"); got != "missing" {
		t.Fatalf("rounded=%s", got)
	}
}

func TestFontDownloadRejectsUnknownKeyBeforeNetworkWork(t *testing.T) {
	a := testApp(t)
	request := httptest.NewRequest(http.MethodPost, "http://dashboard.local/api/fonts/download", nil)
	response := httptest.NewRecorder()
	if !a.handleFontPost(response, request, "/api/fonts/download", map[string]any{"key": "not-a-font"}) {
		t.Fatal("font route was not handled")
	}
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestFontFaceCSSRevalidatesWithETag(t *testing.T) {
	a := testApp(t)
	first := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "http://dashboard.local/api/fonts/face.css", nil)
	if !a.handleFontGet(first, request, "/api/fonts/face.css") {
		t.Fatal("font stylesheet route was not handled")
	}
	if got := first.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("cache-control=%q, want no-cache", got)
	}
	tag := first.Header().Get("ETag")
	if tag == "" {
		t.Fatal("font stylesheet response omitted ETag")
	}
	second := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "http://dashboard.local/api/fonts/face.css", nil)
	request.Header.Set("If-None-Match", tag)
	if !a.handleFontGet(second, request, "/api/fonts/face.css") {
		t.Fatal("conditional font stylesheet route was not handled")
	}
	if second.Code != http.StatusNotModified {
		t.Fatalf("conditional status=%d, want %d", second.Code, http.StatusNotModified)
	}
	if second.Body.Len() != 0 {
		t.Fatalf("conditional font stylesheet returned a body: %q", second.Body.String())
	}
}
