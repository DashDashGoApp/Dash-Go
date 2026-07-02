// Package auth contains the command-independent pieces of Dash-Go control
// authentication: PIN hashing, timeout normalization, and random tokens.
package auth

import (
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultTimeout       = "1800"
	EveryOpenActiveTTL   = 90 * time.Second
	RefreshGrace         = 60 * time.Second
	OneShotTTL           = 2 * time.Minute
	DefaultPINIterations = 200000
	MinPINIterations     = 100000
	MaxPINIterations     = 1000000
)

type TimeoutOption struct {
	Value   string `json:"value"`
	Label   string `json:"label"`
	Seconds *int   `json:"seconds"`
}

func intPtr(n int) *int { return &n }

func TimeoutOptions() []TimeoutOption {
	return []TimeoutOption{
		{"every_open", "Every control open", intPtr(0)},
		{"60", "1 minute", intPtr(60)},
		{"300", "5 minutes", intPtr(300)},
		{"900", "15 minutes", intPtr(900)},
		{"1800", "30 minutes", intPtr(1800)},
		{"3600", "1 hour", intPtr(3600)},
		{"until_reboot", "Until reboot", nil},
	}
}

func NormalizeTimeout(raw any) string {
	s := strings.TrimSpace(fmt.Sprint(raw))
	aliases := map[string]string{
		"": "1800", "0": "every_open", "always": "every_open", "every": "every_open",
		"every-open": "every_open", "every_open": "every_open", "reboot": "until_reboot",
		"until-reboot": "until_reboot", "until_reboot": "until_reboot",
	}
	if v, ok := aliases[s]; ok {
		s = v
	}
	for _, option := range TimeoutOptions() {
		if s == option.Value {
			return s
		}
	}
	return DefaultTimeout
}

func TimeoutInfo(raw string) TimeoutOption {
	value := NormalizeTimeout(raw)
	for _, option := range TimeoutOptions() {
		if option.Value == value {
			return option
		}
	}
	return TimeoutOptions()[4]
}

func ValidPIN(pin string) bool {
	// Four-to-eight digit PINs remain accepted for compatibility with existing
	// kiosk installations. Persistent escalating lockout protects the lower end
	// of that range without silently invalidating an installed household PIN.
	if len(pin) < 4 || len(pin) > 8 {
		return false
	}
	for i := 0; i < len(pin); i++ {
		if pin[i] < '0' || pin[i] > '9' {
			return false
		}
	}
	return true
}

func encode(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func decode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "="))
}

// pbkdf2Key derives a key via the standard library's PBKDF2 (Go 1.24+,
// part of the validated crypto module). The stdlib implementation only
// returns an error for FIPS-mode parameter violations; PIN hashing uses
// SHA-256 with well-formed lengths, so a failure here is unreachable and
// treated as an empty (never-matching) key.
func pbkdf2Key(password, salt []byte, iter, keyLen int) []byte {
	derived, err := pbkdf2.Key(sha256.New, string(password), salt, iter, keyLen)
	if err != nil {
		return nil
	}
	return derived
}

func NewPINPayload(pin string, timeout any) (map[string]string, error) {
	if !ValidPIN(pin) {
		return nil, errors.New("PIN must be 4–8 digits")
	}
	salt := make([]byte, 16)
	_, _ = rand.Read(salt)
	iterations := DefaultPINIterations
	digest := pbkdf2Key([]byte(pin), salt, iterations, 32)
	payload := map[string]string{
		"DASH_CONTROL_PIN_ENABLED":    "1",
		"DASH_CONTROL_PIN_ITERATIONS": strconv.Itoa(iterations),
		"DASH_CONTROL_PIN_SALT":       encode(salt),
		"DASH_CONTROL_PIN_HASH":       encode(digest),
	}
	if timeout != nil {
		payload["DASH_CONTROL_PIN_TIMEOUT"] = NormalizeTimeout(timeout)
	}
	return payload, nil
}

func VerifyPIN(pin, saltEncoded, hashEncoded string, iterations int) bool {
	if !ValidPIN(pin) {
		return false
	}
	salt, saltErr := decode(saltEncoded)
	want, hashErr := decode(hashEncoded)
	if saltErr != nil || hashErr != nil || len(want) == 0 || iterations < MinPINIterations || iterations > MaxPINIterations {
		return false
	}
	got := pbkdf2Key([]byte(pin), salt, iterations, len(want))
	return hmac.Equal(got, want)
}

func NewToken() string {
	bytes := make([]byte, 24)
	_, _ = rand.Read(bytes)
	return encode(bytes)
}

func SessionExpiry(timeout string, now time.Time) *time.Time {
	timeout = NormalizeTimeout(timeout)
	if timeout == "until_reboot" {
		return nil
	}
	if timeout == "every_open" {
		expires := now.Add(EveryOpenActiveTTL)
		return &expires
	}
	if seconds, err := strconv.Atoi(timeout); err == nil {
		expires := now.Add(time.Duration(seconds) * time.Second)
		return &expires
	}
	expires := now.Add(30 * time.Minute)
	return &expires
}
