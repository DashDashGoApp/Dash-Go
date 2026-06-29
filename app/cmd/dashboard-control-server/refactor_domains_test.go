package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// These focused regression checks protect the domain seams introduced when the
// former monolithic main.go was split into auth and HTTP files.
func TestAuthDomainPinSessionLifecycle(t *testing.T) {
	a := testProfileApp(t)
	if _, err := a.setPin("2468", "60"); err != nil {
		t.Fatalf("setPin: %v", err)
	}
	if !a.verifyPin("2468") {
		t.Fatal("correct PIN was rejected")
	}
	if a.verifyPin("0000") {
		t.Fatal("incorrect PIN was accepted")
	}

	token := a.issueToken()
	if token == "" || !a.tokenOK(token) {
		t.Fatal("issued session token was not accepted")
	}
	if ttl := a.sessionTTL(token); ttl <= 0 || ttl > 60 {
		t.Fatalf("unexpected token TTL: %d", ttl)
	}
	if refreshed := a.refreshSession(token, "300"); refreshed["sessionRefreshed"] != true {
		t.Fatalf("session did not refresh: %#v", refreshed)
	}

	a.revoke(token)
	if a.tokenOK(token) {
		t.Fatal("revoked token remained accepted")
	}
	a.removePin()
	if !a.tokenOK("") {
		t.Fatal("PIN-off mode should allow the local control request")
	}
}

func TestHTTPDomainRejectsNonLoopbackRequests(t *testing.T) {
	a := testProfileApp(t)
	req := httptest.NewRequest(http.MethodGet, "http://dashboard.local/api/status", nil)
	req.RemoteAddr = "203.0.113.9:4567"
	res := httptest.NewRecorder()

	a.handle(res, req)
	if res.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
}

func sourceLineCount(t *testing.T, path string) int {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.Count(body, []byte("\n")) + 1
}

func TestControlServerSourceStructureStaysNavigable(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate test source")
	}
	serverRoot := filepath.Dir(thisFile)
	projectRoot := filepath.Clean(filepath.Join(serverRoot, "..", ".."))

	goRoots := []string{serverRoot, filepath.Join(projectRoot, "internal", "auth"), filepath.Join(projectRoot, "internal", "jsonutil"), filepath.Join(projectRoot, "internal", "fileio"), filepath.Join(projectRoot, "internal", "settings"), filepath.Join(projectRoot, "internal", "weather"), filepath.Join(projectRoot, "internal", "calendar", "events"), filepath.Join(projectRoot, "internal", "maps"), filepath.Join(projectRoot, "internal", "messages"), filepath.Join(projectRoot, "internal", "notify"), filepath.Join(projectRoot, "internal", "household"), filepath.Join(projectRoot, "internal", "household", "family")}
	for _, sourceRoot := range goRoots {
		err := filepath.WalkDir(sourceRoot, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			lines := sourceLineCount(t, path)
			name := filepath.Base(path)
			if name == "main.go" && lines > 300 {
				t.Fatalf("main.go grew to %d lines; keep it as the entry point and app-state file", lines)
			}
			if lines > 400 {
				t.Fatalf("%s grew to %d lines; split it by runtime responsibility before it becomes a monolith", path, lines)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	generated := map[string]bool{
		filepath.Join(projectRoot, "ui", "dashboard.css"):               true,
		filepath.Join(projectRoot, "ui", "control-layout.css"):          true,
		filepath.Join(projectRoot, "ui", "js", "app.bundle.js"):         true,
		filepath.Join(projectRoot, "ui", "js", "app.control.bundle.js"): true,
	}
	for path := range generated {
		body, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			// Source-only handoffs intentionally omit derived browser assets.
			// The local release builder creates and verifies them after this
			// source test has established that split modules remain navigable.
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(body, []byte("GENERATED")) {
			t.Fatalf("%s must remain an explicitly generated browser output", path)
		}
	}

	uiRoot := filepath.Join(projectRoot, "ui")
	err := filepath.WalkDir(uiRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || generated[path] {
			return nil
		}
		ext := filepath.Ext(path)
		if ext != ".js" && ext != ".css" {
			return nil
		}
		limit := 500
		if ext == ".js" {
			limit = 400
		}
		if lines := sourceLineCount(t, path); lines > limit {
			t.Fatalf("%s grew to %d lines; split this %s source module by feature", path, lines, ext)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
