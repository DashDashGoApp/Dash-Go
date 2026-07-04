package main

import (
	"bufio"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	googleOAuthAuthorizationEndpoint = "https://accounts.google.com/o/oauth2/v2/auth"
	googleOAuthTokenEndpoint         = "https://oauth2.googleapis.com/token"
	googleCalendarOAuthScope         = "https://www.googleapis.com/auth/calendar"
	// A Desktop OAuth client may use a loopback redirect URI. The headless
	// setup path asks the administrator to paste the complete callback URL
	// after approval, so no browser, SSH tunnel, or Python OAuth helper runs
	// on the Dash-Go device.
	googleOAuthPastebackRedirectURI = "http://127.0.0.1:8433/"
)

type googleOAuthToken map[string]any

func (a *app) runGoogleOAuthCLI(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: dashboard-control-server --google-oauth {authorize|check} [flags]")
		return 1
	}
	mode := args[0]
	fs := flag.NewFlagSet("google-oauth", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var options googleOAuthOptions
	fs.StringVar(&options.clientID, "client-id", "", "OAuth client ID")
	fs.StringVar(&options.clientSecretFile, "client-secret-file", "", "owner-only OAuth client-secret file")
	fs.StringVar(&options.tokenFile, "token-file", "", "owner-only vdirsyncer Google token file")
	fs.BoolVar(&options.qr, "qr", false, "render the authorization URL as a QR code when qrencode is installed")
	fs.StringVar(&options.relayDir, "relay-dir", "", "owner-only OAuth relay spool directory for an automatic web callback")
	fs.StringVar(&options.redirectURI, "redirect-uri", "", "exact registered HTTPS callback URI for a web OAuth client")
	fs.BoolVar(&options.noDisplay, "no-display", false, "do not show the authorization QR code on the dashboard display")
	if err := fs.Parse(args[1:]); err != nil {
		return 1
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "google-oauth does not accept additional positional arguments")
		return 1
	}
	if strings.TrimSpace(options.clientID) == "" || strings.TrimSpace(options.clientSecretFile) == "" || strings.TrimSpace(options.tokenFile) == "" {
		fmt.Fprintln(os.Stderr, "client-id, client-secret-file, and token-file are required")
		return 1
	}
	switch mode {
	case "authorize":
		if err := googleOAuthValidateRelayOptions(options); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return googleOAuthAuthorize(options)
	case "check":
		return googleOAuthCheck(options)
	default:
		fmt.Fprintf(os.Stderr, "unknown google-oauth mode %q\n", mode)
		return 1
	}
}

func googleOAuthMaybeQR(authorizationURL string, stderr io.Writer) {
	qrencode, err := exec.LookPath("qrencode")
	if err != nil {
		return
	}
	encoded, err := exec.Command(qrencode, "-t", "ANSIUTF8", authorizationURL).Output()
	if err != nil || len(encoded) == 0 {
		return
	}
	fmt.Fprintf(stderr, "\nOr scan this with your phone:\n\n%s\n", encoded)
	if rows := googleOAuthTerminalRows(); rows > 0 && strings.Count(string(encoded), "\n") > rows {
		fmt.Fprintln(stderr, "The terminal QR is taller than this window; enlarge it or use the printed link instead.")
	}
}

func googleOAuthTerminalRows() int {
	output, err := exec.Command("stty", "size").Output()
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(output))
	if len(fields) != 2 {
		return 0
	}
	rows, err := strconv.Atoi(fields[0])
	if err != nil || rows <= 0 {
		return 0
	}
	return rows
}

// googleOAuthParsePasteback accepts a full callback URL, a callback URL pasted
// without its scheme, or a bare authorization code. Full URLs verify state;
// a bare code still remains bound to this one-time PKCE verifier, but the
// interactive prompt deliberately asks for the full URL whenever possible.
func googleOAuthParsePasteback(pasted, expectedState string) (string, error) {
	pasted = strings.TrimSpace(pasted)
	if pasted == "" {
		return "", errors.New("empty response")
	}
	if strings.Contains(pasted, "://") || strings.Contains(pasted, "?") || strings.HasPrefix(pasted, "127.0.0.1") {
		if !strings.Contains(pasted, "://") {
			pasted = "http://" + pasted
		}
		callback, err := url.Parse(pasted)
		if err != nil {
			return "", err
		}
		query := callback.Query()
		if googleError := strings.TrimSpace(query.Get("error")); googleError != "" {
			return "", fmt.Errorf("Google returned an error: %s", googleError)
		}
		if subtle.ConstantTimeCompare([]byte(query.Get("state")), []byte(expectedState)) != 1 {
			return "", errors.New("the pasted link is from a different or stale sign-in attempt (state mismatch); start again and paste the link from this attempt")
		}
		if code := strings.TrimSpace(query.Get("code")); code != "" {
			return code, nil
		}
		return "", errors.New("no code parameter is present in the pasted address")
	}
	return pasted, nil
}

func googleOAuthPromptPasteback(input io.Reader, stderr io.Writer, state string) (string, error) {
	fmt.Fprintln(stderr, "\nAfter you approve access, the browser may show a connection error at an address starting with http://127.0.0.1:8433/ — that is expected for this headless paste-back step.")
	fmt.Fprintln(stderr, "Copy the complete address from the browser address bar and paste it here. A full address verifies this sign-in attempt.")
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 1024), 128*1024)
	for attempt := 0; attempt < 3; attempt++ {
		fmt.Fprint(stderr, "\nPaste the address (or just the code), blank to cancel: ")
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return "", err
			}
			return "", errors.New("cancelled")
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			return "", errors.New("cancelled")
		}
		code, err := googleOAuthParsePasteback(line, state)
		if err == nil {
			return code, nil
		}
		fmt.Fprintln(stderr, err)
	}
	return "", errors.New("too many invalid authorization responses")
}

func googleOAuthReadSecret(path string) (string, error) {
	secret, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(string(secret))
	if value == "" || strings.ContainsAny(value, "\r\n") {
		return "", errors.New("client secret is empty or is not a single line")
	}
	return value, nil
}

func googleOAuthTokenRequest(form url.Values) (googleOAuthToken, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	response, err := client.PostForm(googleOAuthTokenURL(), form)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint returned %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	var token googleOAuthToken
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, err
	}
	if strings.TrimSpace(stringValue(token["access_token"])) == "" {
		return nil, errors.New("token response did not include an access token")
	}
	return token, nil
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func googleOAuthExpirySeconds(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, typed >= 0
	case float32:
		return float64(typed), typed >= 0
	case int:
		return float64(typed), typed >= 0
	case int64:
		return float64(typed), typed >= 0
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil && parsed >= 0
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed, err == nil && parsed >= 0
	default:
		return 0, false
	}
}

// googleOAuthWriteTokenAtomic saves the standard oauthlib token mapping used
// by vdirsyncer's google_calendar storage. Google supplies expires_in; oauthlib
// also accepts the corresponding absolute expires_at timestamp that lets the
// isolated vdirsyncer environment refresh the token without another consent.
func googleOAuthWriteTokenAtomic(path string, token googleOAuthToken) error {
	if _, hasExpiry := token["expires_at"]; !hasExpiry {
		if seconds, ok := googleOAuthExpirySeconds(token["expires_in"]); ok {
			token["expires_at"] = float64(time.Now().Unix()) + seconds
		}
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	encoded, err := json.Marshal(token)
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	temporary, err := os.CreateTemp(dir, ".google-token-*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(encoded); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryName, path)
}

func googleOAuthAuthorize(options googleOAuthOptions) int {
	state := googleOAuthRandomToken(16)
	verifier, _ := googleOAuthPKCE()
	return googleOAuthAuthorizeWith(options, os.Stdin, os.Stdout, os.Stderr, state, verifier)
}

func googleOAuthAuthorizeWith(options googleOAuthOptions, input io.Reader, stdout, stderr io.Writer, state, verifier string) int {
	secret, err := googleOAuthReadSecret(options.clientSecretFile)
	if err != nil {
		fmt.Fprintf(stderr, "cannot read OAuth client secret: %v\n", err)
		return 1
	}
	if state == "" {
		state = googleOAuthRandomToken(16)
	}
	if verifier == "" {
		verifier, _ = googleOAuthPKCE()
	}
	redirectURI := googleOAuthRedirectURI(options)
	authorizationURL := googleOAuthAuthorizationURLFor(options, state, verifier, redirectURI)
	fmt.Fprintln(stderr, "\nOpen this Google sign-in link on any device — a phone or laptop is fine:")
	fmt.Fprintln(stdout, authorizationURL)
	if options.qr {
		googleOAuthMaybeQR(authorizationURL, stderr)
	}

	var code string
	if strings.TrimSpace(options.relayDir) != "" {
		if !options.noDisplay {
			if _, qrErr := exec.LookPath("qrencode"); qrErr == nil {
				fmt.Fprintln(stderr, "The QR is now showing on the dashboard display. Scan it with your phone to finish Google sign-in.")
			}
		}
		code, err = googleOAuthRelayWait(options.relayDir, state, googleOAuthConnectionLabel(options.tokenFile), authorizationURL, !options.noDisplay)
		if errors.Is(err, errGoogleOAuthInterrupted) {
			fmt.Fprintln(stderr, "Authorization cancelled. This calendar remains skipped until it is authorized; re-run setup-vdirsyncer.sh --authorize.")
			return 2
		}
		if err == nil && code != "" {
			fmt.Fprintln(stderr, "Google sign-in was received from the dashboard callback.")
		} else if err != nil {
			fmt.Fprintf(stderr, "Automatic web callback did not complete: %v\nFalling back to paste-back.\n", err)
		}
	}
	if code == "" {
		code, err = googleOAuthPromptPasteback(input, stderr, state)
		if err != nil {
			fmt.Fprintln(stderr, "Authorization cancelled. This calendar remains skipped until it is authorized; re-run setup-vdirsyncer.sh --authorize.")
			return 2
		}
	}
	token, err := googleOAuthTokenRequest(url.Values{
		"client_id":     {options.clientID},
		"client_secret": {secret},
		"code":          {code},
		"code_verifier": {verifier},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {redirectURI},
	})
	if err != nil {
		fmt.Fprintf(stderr, "Google rejected the authorization code exchange: %v\n", err)
		if strings.Contains(strings.ToLower(err.Error()), "invalid_grant") {
			fmt.Fprintln(stderr, "Authorization codes expire quickly and can be used only once. Start authorization again and paste the new callback promptly.")
		}
		return 3
	}
	if strings.TrimSpace(stringValue(token["refresh_token"])) == "" {
		fmt.Fprintln(stderr, "Google did not return a refresh token. Remove Dash-Go access in Google Account security settings, then authorize again.")
		return 3
	}
	if err := googleOAuthWriteTokenAtomic(options.tokenFile, token); err != nil {
		fmt.Fprintf(stderr, "could not write the Google token file: %v\n", err)
		return 1
	}
	fmt.Fprintln(stderr, "Google authorization saved.")
	return 0
}

func googleOAuthCheck(options googleOAuthOptions) int {
	raw, err := os.ReadFile(options.tokenFile)
	if err != nil {
		return 1
	}
	var token googleOAuthToken
	if err := json.Unmarshal(raw, &token); err != nil {
		return 1
	}
	refreshToken := strings.TrimSpace(stringValue(token["refresh_token"]))
	if refreshToken == "" {
		return 1
	}
	secret, err := googleOAuthReadSecret(options.clientSecretFile)
	if err != nil {
		return 1
	}
	refreshed, err := googleOAuthTokenRequest(url.Values{
		"client_id":     {options.clientID},
		"client_secret": {secret},
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	})
	if err != nil {
		return 1
	}
	if strings.TrimSpace(stringValue(refreshed["refresh_token"])) == "" {
		refreshed["refresh_token"] = refreshToken
	}
	if err := googleOAuthWriteTokenAtomic(options.tokenFile, refreshed); err != nil {
		return 1
	}
	return 0
}
