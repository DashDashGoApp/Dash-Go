package main

import (
	"path/filepath"
	"strings"
	"time"

	controlauth "github.com/DashDashGoApp/Dash-Go/app/internal/auth"
)

// Dashboard Control auth is a bounded service. Core keeps route/CLI adaptation
// only; the service owns the private PIN document, session tokens, one-shot
// tokens, PIN failures, and the single lock protecting that runtime state.
type timeoutOption = controlauth.TimeoutOption

const (
	defaultTimeout = controlauth.DefaultTimeout
	oneShotTTL     = controlauth.OneShotTTL
)

func normalizeTimeout(raw any) string { return controlauth.NormalizeTimeout(raw) }

func (a *app) controlEnvPath() string {
	if strings.TrimSpace(a.home) == "" {
		return ""
	}
	return filepath.Join(a.home, ".dashboard-control.env")
}

func (a *app) authService() *controlauth.Service {
	a.authInitMu.Lock()
	defer a.authInitMu.Unlock()
	if a.auth == nil {
		a.auth = controlauth.NewService(controlauth.ServiceConfig{EnvPath: a.controlEnvPath(), Now: time.Now})
	}
	return a.auth
}

func (a *app) lockConfig() map[string]any { return a.authService().Config().Payload() }
func (a *app) lockConfigAvailable() bool  { return a.authService().Config().Available }
func (a *app) pinStatus(token string) map[string]any {
	return a.authService().PinStatus(token)
}
func (a *app) verifyPin(pin string) bool { return a.authService().VerifyPIN(pin) }
func (a *app) setPin(pin string, timeout any) (map[string]any, error) {
	return a.authService().SetPIN(pin, timeout)
}
func (a *app) setPinTimeout(timeout string) error { return a.authService().SetTimeout(timeout) }
func (a *app) removePin() (map[string]any, error) { return a.authService().RemovePIN() }
func (a *app) issueToken() string                 { return a.authService().IssueToken() }
func (a *app) tokenOK(token string) bool          { return a.authService().TokenOK(token) }
func (a *app) sessionTTL(token string) int        { return a.authService().SessionTTL(token) }
func (a *app) refreshSession(token, timeout string) map[string]any {
	return a.authService().RefreshSession(token, timeout)
}
func (a *app) revoke(token string)             { a.authService().Revoke(token) }
func (a *app) issueOneShot(path string) string { return a.authService().IssueOneShot(path) }
func (a *app) consumeOneShot(token, path string) bool {
	return a.authService().ConsumeOneShot(token, path)
}
func (a *app) pinLockoutRemaining() int { return a.authService().PINLockoutRemaining() }
func (a *app) recordPinFailure() int    { return a.authService().RecordPINFailure() }
func (a *app) clearPinFailures()        { a.authService().ClearPINFailures() }

// Keep pure token and ternary helpers at the core adapter layer for unrelated
// callers that predate the auth-service extraction. They do not own mutable
// authorization state.
func randToken() string { return controlauth.NewToken() }
func tern[T any](condition bool, whenTrue, whenFalse T) T {
	if condition {
		return whenTrue
	}
	return whenFalse
}
