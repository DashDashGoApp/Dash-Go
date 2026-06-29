package settings

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

var (
	reThemeSetting = regexp.MustCompile(`(?m)(\b(?:"theme"|theme)\s*:\s*)(?:"[^"]*"|'[^']*'|true|false|-?\d+(?:\.\d+)?)(\s*,?)`)
)

// ConfigString preserves the existing lightweight config.local.js extraction
// contract. It intentionally does not attempt to parse arbitrary JavaScript.
func (s *Service) ConfigString(key, def string) string {
	b, err := os.ReadFile(s.configLocal)
	if err != nil {
		return def
	}
	re := regexp.MustCompile(`(?m)(?:"` + regexp.QuoteMeta(key) + `"|` + regexp.QuoteMeta(key) + `)\s*:\s*["']([^"']*)["']`)
	match := re.FindStringSubmatch(string(b))
	if len(match) > 1 {
		return match[1]
	}
	return def
}

func (s *Service) CurrentTheme() string { return s.ConfigString("theme", "basic") }

func (s *Service) ThemeList() []string {
	b, err := os.ReadFile(s.themeCatalog)
	if err != nil {
		return []string{"basic"}
	}
	seen := map[string]bool{}
	themes := make([]string, 0)
	for _, line := range strings.Split(string(b), "\n") {
		name := strings.TrimSpace(strings.SplitN(line, "#", 2)[0])
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		themes = append(themes, name)
	}
	if len(themes) == 0 {
		return []string{"basic"}
	}
	return themes
}

func ThemeNameFromBody(body map[string]any) string {
	for _, key := range []string{"name", "theme"} {
		if value := jsonutil.BodyString(body, key); value != "" {
			return value
		}
	}
	return ""
}

func (s *Service) ValidTheme(name string) bool {
	for _, theme := range s.ThemeList() {
		if theme == name {
			return true
		}
	}
	return false
}

func (s *Service) WriteTheme(theme string) error {
	_ = os.MkdirAll(filepath.Dir(s.configLocal), 0755)
	b, err := os.ReadFile(s.configLocal)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		return os.WriteFile(s.configLocal, []byte(fmt.Sprintf("window.DASHBOARD_LOCAL = { theme: %q };\n", theme)), 0644)
	}
	jsValue := strconv.Quote(theme)
	text := string(b)
	if reThemeSetting.MatchString(text) {
		text = reThemeSetting.ReplaceAllString(text, "${1}"+jsValue+"${2}")
	} else if index := strings.Index(text, "{"); index >= 0 {
		insertAt := index + 1
		text = text[:insertAt] + "\n  theme: " + jsValue + "," + text[insertAt:]
	} else {
		text = fmt.Sprintf("window.DASHBOARD_LOCAL = { theme: %q };\n", theme)
	}
	return os.WriteFile(s.configLocal, []byte(text), 0644)
}
