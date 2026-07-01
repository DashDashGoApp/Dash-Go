package auth

import (
	"fmt"
	"os"
)

// initializeLocalRecovery honors a one-shot flag created from the local shell:
// ~/.dashboard-control-pin-reset followed by a dashboard-control-server restart.
// The flag is deliberately not reachable through HTTP. It is a recovery route
// for a forgotten kiosk PIN, not an alternate browser-side unlock mechanism.
func (s *Service) initializeLocalRecovery() {
	if s.recoveryPath == "" {
		return
	}
	info, err := os.Lstat(s.recoveryPath)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		s.recoveryErr = fmt.Errorf("inspect local PIN recovery flag: %w", err)
		return
	}
	if !info.Mode().IsRegular() {
		s.recoveryErr = fmt.Errorf("local PIN recovery flag is not a regular file")
		return
	}

	s.configMu.Lock()
	defer s.configMu.Unlock()
	if err := writeEnv(s.envPath, map[string]string{
		"DASH_CONTROL_PIN_ENABLED": "0",
		"DASH_CONTROL_PIN_HASH":    "",
		"DASH_CONTROL_PIN_SALT":    "",
	}); err != nil {
		s.recoveryErr = fmt.Errorf("apply local PIN recovery: %w", err)
		return
	}
	if err := os.Remove(s.recoveryPath); err != nil {
		s.recoveryErr = fmt.Errorf("clear local PIN recovery flag: %w", err)
		return
	}
	if s.lockoutPath != "" {
		if err := os.Remove(s.lockoutPath); err != nil && !os.IsNotExist(err) {
			s.recoveryErr = fmt.Errorf("clear PIN recovery lockout: %w", err)
			return
		}
	}
}
