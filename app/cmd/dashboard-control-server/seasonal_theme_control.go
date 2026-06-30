package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (a *app) seasonalThemeFlag() string { return filepath.Join(a.home, ".dashboard-seasonal-themes") }

func (a *app) seasonalThemesEnabled() bool {
	if _, err := os.Stat(a.seasonalThemeFlag()); err == nil {
		return true
	}
	// Compatibility with an existing pre-beta.3 seasonal cron. The first normal
	// apply after update writes the explicit flag, but control state remains
	// truthful before that daily run happens.
	out, err := exec.Command("crontab", "-l").Output()
	return err == nil && strings.Contains(string(out), "seasonal-themes.sh apply")
}

func (a *app) setSeasonalThemesEnabled(enabled bool) error {
	script := filepath.Join(a.binDir, "seasonal-themes.sh")
	if _, err := os.Stat(script); err != nil {
		return fmt.Errorf("seasonal theme helper is unavailable")
	}
	command := "uninstall"
	if enabled {
		command = "install"
	}
	output, err := exec.Command(script, command).CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			return err
		}
		return fmt.Errorf("%s", message)
	}
	return nil
}
