package main

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testFontPayload() []byte {
	payload := make([]byte, 4096)
	payload[0], payload[1], payload[2], payload[3] = 0, 1, 0, 0
	for i := 4; i < len(payload); i++ {
		payload[i] = byte(i % 251)
	}
	return payload
}

func testFontSHA256(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func TestRuntimeFontSpecsArePinnedAndChecksummed(t *testing.T) {
	for key, spec := range runtimeFontSpecs {
		for _, asset := range spec.Assets {
			if !validRuntimeFontSHA256(asset.SHA256) {
				t.Fatalf("%s/%s has invalid SHA-256", key, asset.File)
			}
			if strings.Contains(asset.URL, "/main/") || strings.Contains(asset.URL, "/master/") {
				t.Fatalf("%s/%s uses a floating source URL: %s", key, asset.File, asset.URL)
			}
			if !strings.HasPrefix(asset.URL, "https://") {
				t.Fatalf("%s/%s does not use HTTPS", key, asset.File)
			}
		}
	}
}

func TestRuntimeFontDownloadVerifiesBeforeReplacingLiveFile(t *testing.T) {
	a := testApp(t)
	a.fontsDir = filepath.Join(a.dash, "fonts")
	payload := testFontPayload()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
	defer server.Close()
	asset := runtimeFontAsset{File: "fixture.ttf", Family: "Fixture", Weight: "400", URL: server.URL, SHA256: testFontSHA256(payload)}
	spec := runtimeFontSpec{Key: "fixture", Family: "Fixture", Assets: []runtimeFontAsset{asset}}
	if err := a.downloadRuntimeFontWithClient(spec, server.Client()); err != nil {
		t.Fatalf("verified download failed: %v", err)
	}
	live := filepath.Join(a.fontsDir, asset.File)
	if !runtimeFontAssetValid(live, asset) {
		t.Fatal("verified asset was not installed intact")
	}
}

func TestRuntimeFontHashMismatchKeepsExistingFile(t *testing.T) {
	a := testApp(t)
	a.fontsDir = filepath.Join(a.dash, "fonts")
	if err := os.MkdirAll(a.fontsDir, 0755); err != nil {
		t.Fatal(err)
	}
	live := filepath.Join(a.fontsDir, "fixture.ttf")
	original := []byte("keep the known-good local font untouched")
	if err := os.WriteFile(live, original, 0644); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(testFontPayload())
	}))
	defer server.Close()
	asset := runtimeFontAsset{File: "fixture.ttf", Family: "Fixture", Weight: "400", URL: server.URL, SHA256: strings.Repeat("0", 64)}
	spec := runtimeFontSpec{Key: "fixture", Family: "Fixture", Assets: []runtimeFontAsset{asset}}
	if err := a.downloadRuntimeFontWithClient(spec, server.Client()); err == nil {
		t.Fatal("hash mismatch unexpectedly succeeded")
	}
	got, err := os.ReadFile(live)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatal("failed font download replaced the existing file")
	}
}
