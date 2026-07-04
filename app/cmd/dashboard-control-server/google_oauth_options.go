package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type googleOAuthOptions struct {
	clientID         string
	clientSecretFile string
	tokenFile        string
	qr               bool
	relayDir         string
	redirectURI      string
	noDisplay        bool
}

func googleOAuthTokenURL() string {
	if endpoint := strings.TrimSpace(os.Getenv("DASH_OAUTH_TOKEN_ENDPOINT")); endpoint != "" {
		return endpoint
	}
	return googleOAuthTokenEndpoint
}

func googleOAuthAuthorizeURL() string {
	if endpoint := strings.TrimSpace(os.Getenv("DASH_OAUTH_AUTH_ENDPOINT")); endpoint != "" {
		return endpoint
	}
	return googleOAuthAuthorizationEndpoint
}

func googleOAuthRandomToken(bytes int) string {
	value := make([]byte, bytes)
	if _, err := rand.Read(value); err != nil {
		panic("secure random source is unavailable: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(value)
}

func googleOAuthPKCE() (verifier string, challenge string) {
	verifier = googleOAuthRandomToken(48)
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:])
}

func googleOAuthBuildAuthorizationURL(options googleOAuthOptions, state, challenge, redirectURI string) string {
	query := url.Values{
		"access_type":           {"offline"},
		"client_id":             {options.clientID},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"prompt":                {"consent"},
		"redirect_uri":          {redirectURI},
		"response_type":         {"code"},
		"scope":                 {googleCalendarOAuthScope},
		"state":                 {state},
	}
	return googleOAuthAuthorizeURL() + "?" + query.Encode()
}

func googleOAuthAuthorizationURLFor(options googleOAuthOptions, state, verifier, redirectURI string) string {
	sum := sha256.Sum256([]byte(verifier))
	return googleOAuthBuildAuthorizationURL(options, state, base64.RawURLEncoding.EncodeToString(sum[:]), redirectURI)
}

func googleOAuthRedirectURI(options googleOAuthOptions) string {
	if strings.TrimSpace(options.relayDir) != "" && strings.TrimSpace(options.redirectURI) != "" {
		return strings.TrimSpace(options.redirectURI)
	}
	return googleOAuthPastebackRedirectURI
}

// Google permits HTTP redirect URIs only for a local machine. A phone-to-kiosk
// callback must therefore use an exact HTTPS URI published by an administrator
// (normally a local reverse proxy); Dash-Go keeps its control server loopback
// only and accepts the proxied callback through this narrow, armed route.
func googleOAuthValidateRelayOptions(options googleOAuthOptions) error {
	relayDir := strings.TrimSpace(options.relayDir)
	redirectURI := strings.TrimSpace(options.redirectURI)
	if (relayDir == "") != (redirectURI == "") {
		return errors.New("relay-dir and redirect-uri must be supplied together for automatic web authorization")
	}
	if relayDir == "" {
		return nil
	}
	callback, err := url.Parse(redirectURI)
	if err != nil || callback.Scheme != "https" || callback.Host == "" || callback.User != nil || callback.RawQuery != "" || callback.Fragment != "" || callback.Path != "/oauth/google/callback" {
		return errors.New("web authorization needs an exact HTTPS redirect URI ending in /oauth/google/callback; use Desktop paste-back when no HTTPS callback is configured")
	}
	return nil
}

func googleOAuthConnectionLabel(tokenFile string) string {
	base := strings.TrimSuffix(filepath.Base(tokenFile), filepath.Ext(tokenFile))
	if base == "" || base == "." {
		return "Google Calendar"
	}
	return base
}
