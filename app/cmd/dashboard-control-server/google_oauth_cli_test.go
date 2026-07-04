package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGoogleOAuthParsePasteback(t *testing.T) {
	state := "expected-state"
	for _, test := range []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{name: "full callback URL", input: "http://127.0.0.1:8433/?code=code-1&state=expected-state", want: "code-1"},
		{name: "schemeless callback URL", input: "127.0.0.1:8433/?code=code-2&state=expected-state", want: "code-2"},
		{name: "bare code", input: "4/one-time-code", want: "4/one-time-code"},
		{name: "wrong state", input: "http://127.0.0.1:8433/?code=code-3&state=other", wantErr: "state mismatch"},
		{name: "Google error", input: "http://127.0.0.1:8433/?error=access_denied&state=expected-state", wantErr: "Google returned an error"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := googleOAuthParsePasteback(test.input, state)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("googleOAuthParsePasteback(%q) error = %v, want %q", test.input, err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("googleOAuthParsePasteback(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func TestGoogleOAuthWriteTokenAtomic(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private", "google-tokens")
	path := filepath.Join(dir, "family.json")
	before := time.Now().Unix()
	token := googleOAuthToken{
		"access_token":  "access",
		"expires_in":    float64(3600),
		"refresh_token": "refresh",
		"scope":         googleCalendarOAuthScope,
		"token_type":    "Bearer",
	}
	if err := googleOAuthWriteTokenAtomic(path, token); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("token mode = %o, want 600", got)
	}
	var saved googleOAuthToken
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	if saved["refresh_token"] != "refresh" || saved["scope"] != googleCalendarOAuthScope {
		t.Fatalf("saved token lost vdirsyncer fields: %#v", saved)
	}
	expiresAt, ok := saved["expires_at"].(float64)
	if !ok || expiresAt < float64(before+3500) || expiresAt > float64(time.Now().Unix()+3700) {
		t.Fatalf("expires_at = %#v, want roughly one hour from now", saved["expires_at"])
	}
	leftovers, err := filepath.Glob(filepath.Join(dir, ".google-token-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(leftovers) != 0 {
		t.Fatalf("temporary token files remain: %v", leftovers)
	}
}

func TestGoogleOAuthAuthorizeWritesRefreshableToken(t *testing.T) {
	var received url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		received = r.PostForm
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"access","refresh_token":"refresh","expires_in":3600,"scope":"https://www.googleapis.com/auth/calendar","token_type":"Bearer"}`))
	}))
	defer server.Close()
	t.Setenv("DASH_OAUTH_TOKEN_ENDPOINT", server.URL)
	t.Setenv("DASH_OAUTH_AUTH_ENDPOINT", "https://oauth.example/authorize")
	dir := t.TempDir()
	secretFile := filepath.Join(dir, "secret")
	if err := os.WriteFile(secretFile, []byte("client-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tokenFile := filepath.Join(dir, "google-tokens", "family.json")
	options := googleOAuthOptions{clientID: "client-id", clientSecretFile: secretFile, tokenFile: tokenFile}
	var stdout, stderr bytes.Buffer
	input := strings.NewReader("http://127.0.0.1:8433/?code=authorization-code&state=test-state\n")
	if got := googleOAuthAuthorizeWith(options, input, &stdout, &stderr, "test-state", "test-verifier"); got != 0 {
		t.Fatalf("authorize exit = %d, stderr=%s", got, stderr.String())
	}
	if received.Get("client_id") != "client-id" || received.Get("client_secret") != "client-secret" || received.Get("code") != "authorization-code" || received.Get("code_verifier") != "test-verifier" || received.Get("grant_type") != "authorization_code" || received.Get("redirect_uri") != googleOAuthPastebackRedirectURI {
		t.Fatalf("unexpected token form: %v", received)
	}
	authorizationURL, err := url.Parse(strings.TrimSpace(stdout.String()))
	if err != nil {
		t.Fatal(err)
	}
	query := authorizationURL.Query()
	if query.Get("scope") != googleCalendarOAuthScope || query.Get("state") != "test-state" || query.Get("code_challenge_method") != "S256" || query.Get("redirect_uri") != googleOAuthPastebackRedirectURI {
		t.Fatalf("authorization URL is incomplete: %s", authorizationURL)
	}
	var saved googleOAuthToken
	raw, err := os.ReadFile(tokenFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	if saved["refresh_token"] != "refresh" || saved["expires_at"] == nil {
		t.Fatalf("authorization token lacks unattended-refresh data: %#v", saved)
	}
}

func TestGoogleOAuthAuthorizeRejectsTokenWithoutRefresh(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"access","expires_in":3600,"token_type":"Bearer"}`))
	}))
	defer server.Close()
	t.Setenv("DASH_OAUTH_TOKEN_ENDPOINT", server.URL)
	dir := t.TempDir()
	secretFile := filepath.Join(dir, "secret")
	if err := os.WriteFile(secretFile, []byte("client-secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	tokenFile := filepath.Join(dir, "token.json")
	options := googleOAuthOptions{clientID: "client-id", clientSecretFile: secretFile, tokenFile: tokenFile}
	input := strings.NewReader("http://127.0.0.1:8433/?code=authorization-code&state=test-state\n")
	if got := googleOAuthAuthorizeWith(options, input, &bytes.Buffer{}, &bytes.Buffer{}, "test-state", "test-verifier"); got != 3 {
		t.Fatalf("authorize exit = %d, want 3", got)
	}
	if _, err := os.Stat(tokenFile); !os.IsNotExist(err) {
		t.Fatalf("token file exists after missing refresh token: %v", err)
	}
}

func TestGoogleOAuthCheckPreservesRefreshToken(t *testing.T) {
	var received url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		received = r.PostForm
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"fresh-access","expires_in":3600,"token_type":"Bearer"}`))
	}))
	defer server.Close()
	t.Setenv("DASH_OAUTH_TOKEN_ENDPOINT", server.URL)
	dir := t.TempDir()
	secretFile := filepath.Join(dir, "secret")
	if err := os.WriteFile(secretFile, []byte("client-secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	tokenFile := filepath.Join(dir, "token.json")
	if err := googleOAuthWriteTokenAtomic(tokenFile, googleOAuthToken{"access_token": "old-access", "refresh_token": "kept-refresh", "expires_in": float64(10)}); err != nil {
		t.Fatal(err)
	}
	options := googleOAuthOptions{clientID: "client-id", clientSecretFile: secretFile, tokenFile: tokenFile}
	if got := googleOAuthCheck(options); got != 0 {
		t.Fatalf("check exit = %d", got)
	}
	if received.Get("grant_type") != "refresh_token" || received.Get("refresh_token") != "kept-refresh" {
		t.Fatalf("unexpected refresh form: %v", received)
	}
	var refreshed googleOAuthToken
	raw, err := os.ReadFile(tokenFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &refreshed); err != nil {
		t.Fatal(err)
	}
	if refreshed["access_token"] != "fresh-access" || refreshed["refresh_token"] != "kept-refresh" {
		t.Fatalf("refresh token was not preserved: %#v", refreshed)
	}
}
