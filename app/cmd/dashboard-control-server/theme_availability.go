package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// Calendar-aware theme availability intentionally reads the existing local event
// cache. It never fetches a calendar or guesses an observance from a date.
// That keeps optional holiday themes bounded, offline-safe, and aligned with
// the calendar data the dashboard is already rendering.
type themeAvailability struct {
	Available map[string]bool
	Reasons   map[string]string
	Today     string
}

type seasonalThemeMatch struct {
	Theme string
	Title string
}

var optionalThemeReasons = map[string]string{
	"hanukkah": "Hanukkah appears when an enabled Jewish holiday calendar reports Hanukkah today.",
	"kwanzaa":  "Kwanzaa appears when an enabled holiday calendar reports Kwanzaa today.",
}

func themeTextKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer("’", "'", "‘", "'", "–", "-", "—", "-", "_", " ").Replace(value)
	return strings.Join(strings.Fields(value), " ")
}

func themeEventLayer(event map[string]any) string {
	cal := jsonutil.Map(event["cal"])
	source := strings.ToLower(strings.TrimSpace(jsonutil.TextValue(cal["url"])))
	if source == "" {
		source = strings.ToLower(strings.TrimSpace(jsonutil.TextValue(event["calUrl"])))
	}
	switch {
	case strings.Contains(source, "jewish-holidays."):
		return "jewish"
	case strings.Contains(source, "islamic-holidays."):
		return "islamic"
	case strings.Contains(source, "christian-holidays."):
		return "christian"
	case strings.Contains(source, "orthodox-holidays."):
		return "orthodox"
	case strings.Contains(source, "hindu-holidays."):
		return "hindu"
	case strings.Contains(source, "holidays."):
		return "civil"
	case themeTextKey(jsonutil.TextValue(cal["tag"])) == "holiday":
		return "holiday"
	default:
		return ""
	}
}

func themeEventDate(event map[string]any) (time.Time, bool) {
	start, ok := event["start"]
	if !ok {
		return time.Time{}, false
	}
	var millis int64
	switch value := start.(type) {
	case float64:
		millis = int64(value)
	case int64:
		millis = value
	case int:
		millis = int64(value)
	case json.Number:
		parsed, err := value.Int64()
		if err != nil {
			return time.Time{}, false
		}
		millis = parsed
	default:
		return time.Time{}, false
	}
	if millis <= 0 {
		return time.Time{}, false
	}
	return time.UnixMilli(millis).In(time.Local), true
}

func themeEventIsToday(event map[string]any, now time.Time) bool {
	start, ok := themeEventDate(event)
	if !ok {
		return false
	}
	today := now.In(time.Local)
	return start.Year() == today.Year() && start.YearDay() == today.YearDay()
}

func themeTitleHasAny(title string, names ...string) bool {
	// Holiday providers commonly add a parenthetical day marker (for example,
	// "Hanukkah (first day)") or reverse the title ("First Day of Hanukkah").
	// Normalize punctuation into word boundaries so matching remains explicit
	// without depending on one provider's exact title spelling.
	key := " " + strings.NewReplacer("(", " ", ")", " ", ",", " ", ":", " ").Replace(themeTextKey(title)) + " "
	key = " " + strings.Join(strings.Fields(key), " ") + " "
	for _, name := range names {
		needle := " " + themeTextKey(name) + " "
		if strings.Contains(key, needle) {
			return true
		}
	}
	return false
}

func (a *app) cachedThemeEvents() []map[string]any {
	path := filepath.Join(a.cacheDir, "events.cache.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	out := make([]map[string]any, 0)
	for _, item := range jsonutil.List(raw["events"]) {
		if event := jsonutil.Map(item); len(event) > 0 {
			out = append(out, event)
		}
	}
	return out
}

func themeAvailabilityForEvents(events []map[string]any, now time.Time) themeAvailability {
	state := themeAvailability{Available: map[string]bool{}, Reasons: map[string]string{}, Today: now.In(time.Local).Format("2006-01-02")}
	for key, reason := range optionalThemeReasons {
		state.Reasons[key] = reason
	}
	for _, event := range events {
		if !themeEventIsToday(event, now) {
			continue
		}
		title := jsonutil.TextValue(event["title"])
		layer := themeEventLayer(event)
		if layer == "jewish" && themeTitleHasAny(title, "hanukkah", "chanukah") {
			state.Available["hanukkah"] = true
		}
		if layer != "" && themeTitleHasAny(title, "kwanzaa") {
			state.Available["kwanzaa"] = true
		}
	}
	return state
}

func (a *app) themeAvailability() themeAvailability {
	return themeAvailabilityForEvents(a.cachedThemeEvents(), time.Now())
}

func (a *app) availableThemes() ([]string, themeAvailability) {
	availability := a.themeAvailability()
	all := a.themeList()
	available := make([]string, 0, len(all))
	for _, name := range all {
		if _, optional := optionalThemeReasons[name]; optional && !availability.Available[name] {
			continue
		}
		available = append(available, name)
	}
	return available, availability
}

func (a *app) themeIsAvailable(name string) (bool, string) {
	name = strings.TrimSpace(name)
	if !a.validTheme(name) {
		return false, "unknown theme: " + name
	}
	if reason, optional := optionalThemeReasons[name]; optional && !a.themeAvailability().Available[name] {
		return false, reason
	}
	return true, ""
}

func seasonalThemeForEvents(events []map[string]any, now time.Time) seasonalThemeMatch {
	// Dynamic event-backed observances take precedence over the fixed-date shell
	// schedule. The list is deliberately small and exact; ordinary personal
	// calendar events never qualify unless their source is tagged as a holiday.
	rules := []struct {
		theme  string
		layers []string
		names  []string
	}{
		{"hanukkah", []string{"jewish"}, []string{"hanukkah", "chanukah"}},
		{"kwanzaa", []string{"civil", "holiday"}, []string{"kwanzaa"}},
		{"memorialday", []string{"civil", "holiday"}, []string{"memorial day"}},
		{"laborday", []string{"civil", "holiday"}, []string{"labor day", "labour day"}},
		{"veterans", []string{"civil", "holiday"}, []string{"veterans day", "veteran's day"}},
		{"mothersday", []string{"civil", "holiday"}, []string{"mother's day", "mothers day"}},
		{"fathersday", []string{"civil", "holiday"}, []string{"father's day", "fathers day"}},
	}
	for _, rule := range rules {
		for _, event := range events {
			if !themeEventIsToday(event, now) {
				continue
			}
			layer := themeEventLayer(event)
			if !containsString(rule.layers, layer) || !themeTitleHasAny(jsonutil.TextValue(event["title"]), rule.names...) {
				continue
			}
			return seasonalThemeMatch{Theme: rule.theme, Title: jsonutil.TextValue(event["title"])}
		}
	}
	return seasonalThemeMatch{}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func (a *app) seasonalThemeForToday() seasonalThemeMatch {
	return seasonalThemeForEvents(a.cachedThemeEvents(), time.Now())
}

func (a *app) runSeasonalThemeCLI(args []string) int {
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help") {
		fmt.Println("usage: dashboard-control-server --seasonal-theme")
		return 0
	}
	if match := a.seasonalThemeForToday(); match.Theme != "" {
		fmt.Println(match.Theme)
	}
	return 0
}
