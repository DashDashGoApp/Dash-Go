package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
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
	loopback         bool
	displayDir       string
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

func googleOAuthRedirectURI(_ googleOAuthOptions) string {
	// Dash-Go supports only Google's Desktop-app localhost redirect. The phone
	// route is an explicit paste-back fallback, not a server callback.
	return googleOAuthPastebackRedirectURI
}

func googleOAuthConnectionLabel(tokenFile string) string {
	base := strings.TrimSuffix(filepath.Base(tokenFile), filepath.Ext(tokenFile))
	if base == "" || base == "." {
		return "Google Calendar"
	}
	return base
}
