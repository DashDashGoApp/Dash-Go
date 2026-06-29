package auth

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
)

// Config is the non-secret-facing projection of the Dashboard Control PIN
// document. Its map representation remains compatible with the established
// HTTP route payloads.
type Config struct {
	Enabled    bool
	Hash       string
	Salt       string
	Iterations int
	Timeout    string
	Label      string
	TTL        *int
	Options    []TimeoutOption
}

func readEnv(path string) map[string]string {
	out := map[string]string{}
	if strings.TrimSpace(path) == "" {
		return out
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		value := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
		out[strings.TrimSpace(parts[0])] = value
	}
	return out
}

func writeEnv(path string, updates map[string]string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("Dashboard Control PIN path is unavailable")
	}
	values := readEnv(path)
	for key, value := range updates {
		values[key] = value
	}
	var body strings.Builder
	body.WriteString("# saved by install.sh / control panel — optional dashboard control PIN lock\n")
	order := []string{"DASH_CONTROL_PIN_ENABLED", "DASH_CONTROL_PIN_TIMEOUT", "DASH_CONTROL_PIN_ITERATIONS", "DASH_CONTROL_PIN_SALT", "DASH_CONTROL_PIN_HASH"}
	seen := map[string]bool{}
	for _, key := range order {
		if value, ok := values[key]; ok {
			fmt.Fprintf(&body, "%s=%s\n", key, value)
			seen[key] = true
		}
	}
	keys := []string{}
	for key := range values {
		if !seen[key] {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	for _, key := range keys {
		fmt.Fprintf(&body, "%s=%s\n", key, values[key])
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(body.String()), 0600); err != nil {
		return err
	}
	_ = os.Chmod(tmp, 0600)
	return os.Rename(tmp, path)
}

func (s *Service) Config() Config {
	values := readEnv(s.envPath)
	enabled := values["DASH_CONTROL_PIN_ENABLED"] == "1" && values["DASH_CONTROL_PIN_HASH"] != "" && values["DASH_CONTROL_PIN_SALT"] != ""
	timeout := values["DASH_CONTROL_PIN_TIMEOUT"]
	if timeout == "" {
		timeout = values["DASH_CONTROL_PIN_MODE"]
	}
	if timeout == "" {
		timeout = values["DASH_CONTROL_PIN_TTL_SECONDS"]
	}
	if timeout == "" {
		timeout = DefaultTimeout
	}
	info := TimeoutInfo(timeout)
	iterations, _ := strconv.Atoi(values["DASH_CONTROL_PIN_ITERATIONS"])
	if iterations <= 0 {
		iterations = 200000
	}
	return Config{
		Enabled: enabled, Hash: values["DASH_CONTROL_PIN_HASH"], Salt: values["DASH_CONTROL_PIN_SALT"],
		Iterations: iterations, Timeout: info.Value, Label: info.Label, TTL: info.Seconds, Options: TimeoutOptions(),
	}
}

func (c Config) Payload() map[string]any {
	return map[string]any{
		"enabled": c.Enabled, "hash": c.Hash, "salt": c.Salt, "iterations": c.Iterations,
		"timeout": c.Timeout, "timeoutLabel": c.Label, "ttl": c.TTL, "options": c.Options,
	}
}

func (s *Service) PinStatus(token string) map[string]any {
	config := s.Config()
	unlocked := true
	if config.Enabled {
		unlocked = s.tokenOKEnabled(token)
	}
	return map[string]any{
		"enabled": config.Enabled, "timeout": config.Timeout, "timeoutLabel": config.Label, "ttl": config.TTL,
		"sessionTtl": tern(unlocked, s.SessionTTL(token), 0), "options": config.Options, "unlocked": unlocked,
		"lockoutRemaining": s.PINLockoutRemaining(),
	}
}

func tern[T any](condition bool, whenTrue, whenFalse T) T {
	if condition {
		return whenTrue
	}
	return whenFalse
}

func (s *Service) VerifyPIN(pin string) bool {
	config := s.Config()
	if !config.Enabled {
		return true
	}
	return VerifyPIN(pin, config.Salt, config.Hash, config.Iterations)
}

func (s *Service) SetPIN(pin string, timeout any) (map[string]any, error) {
	payload, err := NewPINPayload(pin, timeout)
	if err != nil {
		return nil, err
	}
	if err := writeEnv(s.envPath, payload); err != nil {
		return nil, err
	}
	s.ClearPINFailures()
	return s.Config().Payload(), nil
}

func (s *Service) SetTimeout(timeout string) error {
	return writeEnv(s.envPath, map[string]string{"DASH_CONTROL_PIN_TIMEOUT": NormalizeTimeout(timeout)})
}

func (s *Service) RemovePIN() map[string]any {
	_ = writeEnv(s.envPath, map[string]string{"DASH_CONTROL_PIN_ENABLED": "0", "DASH_CONTROL_PIN_HASH": "", "DASH_CONTROL_PIN_SALT": ""})
	s.mu.Lock()
	s.sessions = map[string]sessionMeta{}
	s.failTimes = nil
	s.mu.Unlock()
	return s.Config().Payload()
}
