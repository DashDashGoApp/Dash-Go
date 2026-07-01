package auth

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
)

// Config is the non-secret-facing projection of the Dashboard Control PIN
// document. Credentials stay inside the auth service and are never serialized
// into browser-facing API payloads.
type Config struct {
	Enabled    bool
	Available  bool
	Hash       string
	Salt       string
	Iterations int
	Timeout    string
	Label      string
	TTL        *int
	Options    []TimeoutOption
}

func readEnv(path string) (map[string]string, error) {
	out := map[string]string{}
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("Dashboard Control PIN path is unavailable")
	}
	body, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return nil, err
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
	return out, nil
}

func writeEnv(path string, updates map[string]string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("Dashboard Control PIN path is unavailable")
	}
	values, err := readEnv(path)
	if err != nil {
		return fmt.Errorf("read Dashboard Control PIN configuration: %w", err)
	}
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
	return fileio.WriteAtomic(path, []byte(body.String()), 0600)
}

func unavailableConfig() Config {
	return Config{Enabled: true, Available: false, Timeout: DefaultTimeout, Label: TimeoutInfo(DefaultTimeout).Label, TTL: TimeoutInfo(DefaultTimeout).Seconds, Options: TimeoutOptions()}
}

func configFromValues(values map[string]string) Config {
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
	enabled := values["DASH_CONTROL_PIN_ENABLED"] == "1"
	if !enabled {
		return Config{Enabled: false, Available: true, Timeout: info.Value, Label: info.Label, TTL: info.Seconds, Options: TimeoutOptions()}
	}

	hash := values["DASH_CONTROL_PIN_HASH"]
	salt := values["DASH_CONTROL_PIN_SALT"]
	iterations := DefaultPINIterations
	if raw := strings.TrimSpace(values["DASH_CONTROL_PIN_ITERATIONS"]); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < MinPINIterations || parsed > MaxPINIterations {
			return unavailableConfig()
		}
		iterations = parsed
	}
	if hash == "" || salt == "" {
		return unavailableConfig()
	}
	return Config{
		Enabled: enabled, Available: true, Hash: hash, Salt: salt, Iterations: iterations,
		Timeout: info.Value, Label: info.Label, TTL: info.Seconds, Options: TimeoutOptions(),
	}
}

func (s *Service) configLocked() Config {
	if s.recoveryErr != nil {
		return unavailableConfig()
	}
	values, err := readEnv(s.envPath)
	if err != nil {
		return unavailableConfig()
	}
	return configFromValues(values)
}

func (s *Service) Config() Config {
	s.configMu.RLock()
	defer s.configMu.RUnlock()
	return s.configLocked()
}

func (c Config) Payload() map[string]any {
	return map[string]any{
		"enabled": c.Enabled, "available": c.Available,
		"timeout": c.Timeout, "timeoutLabel": c.Label, "ttl": c.TTL, "options": c.Options,
	}
}

func (s *Service) PinStatus(token string) map[string]any {
	config := s.Config()
	unlocked := !config.Enabled && config.Available
	if config.Enabled && config.Available {
		unlocked = s.tokenOKEnabled(token)
	}
	payload := map[string]any{
		"enabled": config.Enabled, "available": config.Available,
		"timeout": config.Timeout, "timeoutLabel": config.Label, "ttl": config.TTL,
		"sessionTtl": tern(unlocked, s.SessionTTL(token), 0), "options": config.Options, "unlocked": unlocked,
		"lockoutRemaining": s.PINLockoutRemaining(),
	}
	if !config.Available {
		payload["error"] = "PIN protection configuration is unavailable; use local recovery."
	}
	return payload
}

func tern[T any](condition bool, whenTrue, whenFalse T) T {
	if condition {
		return whenTrue
	}
	return whenFalse
}

// VerifyPIN answers only whether a configured credential matches. Disabled or
// unavailable protection is not a successful identity verification.
func (s *Service) VerifyPIN(pin string) bool {
	config := s.Config()
	if !config.Available || !config.Enabled {
		return false
	}
	return VerifyPIN(pin, config.Salt, config.Hash, config.Iterations)
}

func (s *Service) SetPIN(pin string, timeout any) (map[string]any, error) {
	payload, err := NewPINPayload(pin, timeout)
	if err != nil {
		return nil, err
	}
	s.configMu.Lock()
	defer s.configMu.Unlock()
	if s.recoveryErr != nil {
		return nil, fmt.Errorf("PIN protection configuration is unavailable; use local recovery")
	}
	if err := writeEnv(s.envPath, payload); err != nil {
		return nil, err
	}
	config := s.configLocked()
	if !config.Available || !config.Enabled {
		return nil, fmt.Errorf("PIN protection configuration could not be verified after saving")
	}
	s.ClearPINFailures()
	return config.Payload(), nil
}

func (s *Service) SetTimeout(timeout string) error {
	s.configMu.Lock()
	defer s.configMu.Unlock()
	config := s.configLocked()
	if !config.Available {
		return fmt.Errorf("PIN protection configuration is unavailable; use local recovery")
	}
	if !config.Enabled {
		return fmt.Errorf("PIN lock is not enabled")
	}
	return writeEnv(s.envPath, map[string]string{"DASH_CONTROL_PIN_TIMEOUT": NormalizeTimeout(timeout)})
}

func (s *Service) RemovePIN() (map[string]any, error) {
	s.configMu.Lock()
	defer s.configMu.Unlock()
	config := s.configLocked()
	if !config.Available {
		return nil, fmt.Errorf("PIN protection configuration is unavailable; use local recovery")
	}
	if err := writeEnv(s.envPath, map[string]string{"DASH_CONTROL_PIN_ENABLED": "0", "DASH_CONTROL_PIN_HASH": "", "DASH_CONTROL_PIN_SALT": ""}); err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.sessions = map[string]sessionMeta{}
	s.mu.Unlock()
	s.ClearPINFailures()
	return s.configLocked().Payload(), nil
}
