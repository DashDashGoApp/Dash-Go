package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	settingspkg "github.com/DashDashGoApp/Dash-Go/app/internal/settings"
)

// settingsConfig is the narrow bridge from application-core paths and Radar
// validation into the extracted Settings service. Other domains still retain
// read-only path aliases during the staged refactor, but mutable cache/locking
// and all settings/profile/theme/font logic live in internal/settings.
func (a *app) settingsConfig() settingspkg.Config {
	fontsDir := a.fontsDir
	if fontsDir == "" {
		fontsDir = filepath.Join(a.dash, "fonts")
	}
	return settingspkg.Config{
		SettingsFile:    a.settingsFile,
		ConfigLocal:     a.configLocal,
		CacheDir:        a.cacheDir,
		ThemeCatalog:    filepath.Join(a.dash, "themes.list"),
		FontsDir:        fontsDir,
		BundledFontsDir: filepath.Join(a.dash, "ui", "fonts"),
		ValidateRadar:   validateRadarSettings,
	}
}

func (a *app) settingsService() *settingspkg.Service {
	cfg := a.settingsConfig()
	a.settingsInitMu.Lock()
	defer a.settingsInitMu.Unlock()
	if a.settings == nil || !a.settings.Matches(cfg) {
		a.settings = settingspkg.New(cfg)
	}
	return a.settings
}

func cloneStringAnyMap(src map[string]any) map[string]any { return settingspkg.CloneMap(src) }

type dashboardProfile = settingspkg.Profile

func profileByName(name string) (dashboardProfile, bool) { return settingspkg.ProfileByName(name) }

// validateSettingsShape remains a narrow main-package adapter for Radar's
// consumer-side validator and existing core integration tests.
func validateSettingsShape(values map[string]any) error {
	return settingspkg.ValidateShape(values, validateRadarSettings)
}

func readSettingsObject(path string) (map[string]any, error) {
	return settingspkg.ReadObject(path, validateRadarSettings)
}

func (a *app) invalidateSettingsCache()                  { a.settingsService().Invalidate() }
func (a *app) loadSettings() map[string]any              { return a.settingsService().Load() }
func (a *app) writeSettings(values map[string]any) error { return a.settingsService().Write(values) }
func (a *app) updateSettings(mut func(map[string]any)) (map[string]any, error) {
	return a.settingsService().Update(mut)
}
func (a *app) mutateSettings(mut func(map[string]any) error) (map[string]any, error) {
	return a.settingsService().Mutate(mut)
}
func (a *app) ensureSettingsSafeAtBoot()    { a.settingsService().EnsureSafeAtBoot() }
func (a *app) configRevertFile() string     { return filepath.Join(a.cacheDir, "config-revert.json") }
func (a *app) lastGoodSettingsFile() string { return a.settingsFile + ".last-good" }

func (a *app) profileBaseForSettings(values map[string]any) string {
	return a.settingsService().ProfileBaseForSettings(values)
}
func (a *app) profilePayload() map[string]any { return a.profilePayloadForSettings(a.loadSettings()) }
func (a *app) profilePayloadForSettings(values map[string]any) map[string]any {
	return a.settingsService().ProfilePayloadForSettings(values, a.weatherRefreshPolicyForSettings(values))
}
func (a *app) updateProfileValues(set map[string]any) (map[string]any, error) {
	values, err := a.settingsService().UpdateProfileValues(set)
	if err != nil {
		return nil, err
	}
	return a.profilePayloadForSettings(values), nil
}
func (a *app) applyProfilePreset(name string) (map[string]any, error) {
	values, err := a.settingsService().ApplyProfilePreset(name)
	if err != nil {
		return nil, err
	}
	return a.profilePayloadForSettings(values), nil
}
func (a *app) runApplyProfilePresetCLI(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: dashboard-control-server --apply-profile-preset lite|balanced|enhanced")
		return 2
	}
	payload, err := a.applyProfilePreset(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	body, _ := json.Marshal(payload)
	fmt.Println(string(body))
	return 0
}

func (a *app) configString(key, def string) string { return a.settingsService().ConfigString(key, def) }
func (a *app) currentTheme() string                { return a.settingsService().CurrentTheme() }
func (a *app) themeList() []string                 { return a.settingsService().ThemeList() }
func (a *app) themeNameFromBody(body map[string]any) string {
	return settingspkg.ThemeNameFromBody(body)
}
func (a *app) validTheme(name string) bool   { return a.settingsService().ValidTheme(name) }
func (a *app) writeTheme(theme string) error { return a.settingsService().WriteTheme(theme) }

func (a *app) runSettingsValidateCLI(args []string) int {
	repair := false
	for _, arg := range args {
		if arg == "--repair" {
			repair = true
		}
	}
	message, err := a.settingsService().ValidateCLI(repair)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(message)
	return 0
}
