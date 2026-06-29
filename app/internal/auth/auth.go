// Package auth contains the command-independent pieces of Dash-Go control
// authentication: PIN hashing, timeout normalization, and random tokens.
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"hash"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultTimeout     = "1800"
	EveryOpenActiveTTL = 365 * 24 * time.Hour
	RefreshGrace       = 60 * time.Second
	OneShotTTL         = 2 * time.Minute
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
	ok, _ := regexp.MatchString(`^\d{4,8}$`, pin)
	return ok
}

func encode(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func decode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "="))
}

func pbkdf2Key(password, salt []byte, iter, keyLen int, h func() hash.Hash) []byte {
	prf := hmac.New(h, password)
	hashLen := prf.Size()
	numBlocks := (keyLen + hashLen - 1) / hashLen
	var derived []byte
	blockValue := make([]byte, hashLen)
	for block := 1; block <= numBlocks; block++ {
		prf.Reset()
		_, _ = prf.Write(salt)
		_, _ = prf.Write([]byte{byte(block >> 24), byte(block >> 16), byte(block >> 8), byte(block)})
		blockValue = prf.Sum(blockValue[:0])
		accumulated := append([]byte(nil), blockValue...)
		for i := 1; i < iter; i++ {
			prf.Reset()
			_, _ = prf.Write(blockValue)
			blockValue = prf.Sum(blockValue[:0])
			for index := range accumulated {
				accumulated[index] ^= blockValue[index]
			}
		}
		derived = append(derived, accumulated...)
	}
	return derived[:keyLen]
}

func NewPINPayload(pin string, timeout any) (map[string]string, error) {
	if !ValidPIN(pin) {
		return nil, errors.New("PIN must be 4–8 digits")
	}
	salt := make([]byte, 16)
	_, _ = rand.Read(salt)
	iterations := 200000
	digest := pbkdf2Key([]byte(pin), salt, iterations, 32, sha256.New)
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
	if saltErr != nil || hashErr != nil || len(want) == 0 || iterations <= 0 {
		return false
	}
	got := pbkdf2Key([]byte(pin), salt, iterations, len(want), sha256.New)
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
