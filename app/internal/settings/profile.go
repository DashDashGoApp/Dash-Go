package settings

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// Profile is a named, user-facing performance preset.
type Profile struct {
	Name   string
	Label  string
	Detail string
	Values map[string]any
}

var profileOwnedKeys = []string{
	"showSeconds", "weeksAbove", "weeksBelow", "rowHeight", "sidebarWidth",
	"showInteractiveMaps", "weatherAlerts",
}

var profileOwnedKeySet = func() map[string]struct{} {
	out := make(map[string]struct{}, len(profileOwnedKeys))
	for _, key := range profileOwnedKeys {
		out[key] = struct{}{}
	}
	return out
}()

var profileValueRanges = map[string][2]float64{
	"weeksAbove": {0, 6}, "weeksBelow": {2, 16}, "rowHeight": {150, 280}, "sidebarWidth": {300, 520},
}

func NormalizeProfileName(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "lite", "zero2", "low", "low-power":
		return "lite"
	case "enhanced", "maximum", "x86", "x86_64", "amd64":
		return "enhanced"
	case "balanced":
		return "balanced"
	default:
		return ""
	}
}

func Profiles() []Profile {
	return []Profile{
		{Name: "lite", Label: "Lite", Detail: "Low-memory Pi / kiosk safe", Values: map[string]any{
			"showSeconds": true, "weeksAbove": 1, "weeksBelow": 8, "rowHeight": 205, "sidebarWidth": 370,
			"showInteractiveMaps": false, "weatherAlerts": map[string]any{"enabled": true, "refreshMinutes": 5, "minSeverity": "moderate"},
		}},
		{Name: "balanced", Label: "Balanced", Detail: "Default home kiosk profile", Values: map[string]any{
			"showSeconds": true, "weeksAbove": 2, "weeksBelow": 10, "rowHeight": 210, "sidebarWidth": 380,
			"showInteractiveMaps": false, "weatherAlerts": map[string]any{"enabled": true, "refreshMinutes": 5, "minSeverity": "moderate"},
		}},
		{Name: "enhanced", Label: "Enhanced", Detail: "More capable devices", Values: map[string]any{
			"showSeconds": true, "weeksAbove": 3, "weeksBelow": 12, "rowHeight": 218, "sidebarWidth": 400,
			"showInteractiveMaps": true, "weatherAlerts": map[string]any{"enabled": true, "refreshMinutes": 5, "minSeverity": "moderate"},
		}},
	}
}

func ProfileByName(name string) (Profile, bool) {
	name = NormalizeProfileName(name)
	for _, profile := range Profiles() {
		if profile.Name == name {
			return profile, true
		}
	}
	return Profile{}, false
}

func ProfileMeta(name string) (string, string) {
	if profile, ok := ProfileByName(name); ok {
		return profile.Label, profile.Detail
	}
	return "Balanced", "Default home kiosk profile"
}

func profileLabel(name string) string {
	label, _ := ProfileMeta(name)
	return label
}

func cloneProfileValues(src map[string]any) map[string]any {
	out := make(map[string]any, len(src))
	for key, value := range src {
		if nested, ok := value.(map[string]any); ok {
			out[key] = cloneProfileValues(nested)
		} else {
			out[key] = value
		}
	}
	return out
}

func profileOptionsPayload() []map[string]any {
	out := make([]map[string]any, 0, 3)
	for _, profile := range Profiles() {
		out = append(out, map[string]any{"name": profile.Name, "label": profile.Label, "detail": profile.Detail, "values": cloneProfileValues(profile.Values)})
	}
	return out
}

// ProfileValuesFor exposes only values reset by a selected profile. Older
// tuning keys remain readable but cannot change automatic defaults.
func ProfileValuesFor(name string, values map[string]any) map[string]any {
	profile, ok := ProfileByName(name)
	if !ok {
		profile, _ = ProfileByName("balanced")
	}
	out := cloneProfileValues(profile.Values)
	for key := range out {
		if value, ok := values[key]; ok {
			out[key] = value
		}
	}
	if alerts, ok := out["weatherAlerts"].(map[string]any); ok {
		alerts["refreshMinutes"] = 5
		out["weatherAlerts"] = alerts
	}
	return out
}

func (s *Service) ProfileBaseForSettings(values map[string]any) string {
	base := NormalizeProfileName(fmt.Sprint(values["profile"]))
	if base == "" {
		base = NormalizeProfileName(s.ConfigString("profile", "balanced"))
	}
	if base == "" {
		base = "balanced"
	}
	return base
}

func profileValuesEqual(left, right any) bool {
	if leftNumber, ok := NumberValue(left); ok {
		if rightNumber, ok := NumberValue(right); ok {
			return leftNumber == rightNumber
		}
	}
	leftMap, leftOK := left.(map[string]any)
	rightMap, rightOK := right.(map[string]any)
	if leftOK || rightOK {
		if !leftOK || !rightOK || len(leftMap) != len(rightMap) {
			return false
		}
		for key, leftValue := range leftMap {
			rightValue, ok := rightMap[key]
			if !ok || !profileValuesEqual(leftValue, rightValue) {
				return false
			}
		}
		return true
	}
	return fmt.Sprint(left) == fmt.Sprint(right)
}

func profileDivergedKeys(base string, values map[string]any) []string {
	profile, ok := ProfileByName(base)
	if !ok {
		profile, _ = ProfileByName("balanced")
	}
	out := make([]string, 0, len(profileOwnedKeys))
	for _, key := range profileOwnedKeys {
		preset, hasPreset := profile.Values[key]
		current, present := values[key]
		if hasPreset && present && !profileValuesEqual(current, preset) {
			out = append(out, key)
		}
	}
	return out
}

func cloneProfileValue(value any) any {
	if nested, ok := value.(map[string]any); ok {
		return cloneProfileValues(nested)
	}
	return value
}

func profileChangedSettings(base string, values map[string]any) []map[string]any {
	profile, ok := ProfileByName(base)
	if !ok {
		profile, _ = ProfileByName("balanced")
	}
	effective := ProfileValuesFor(base, values)
	keys := profileDivergedKeys(base, values)
	out := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		out = append(out, map[string]any{"key": key, "default": cloneProfileValue(profile.Values[key]), "current": cloneProfileValue(effective[key])})
	}
	return out
}

func nearestProfile(base string, values map[string]any) string {
	current := ProfileValuesFor(base, values)
	best, bestMiss := "balanced", math.MaxInt
	for _, profile := range Profiles() {
		misses := 0
		for _, key := range profileOwnedKeys {
			if !profileValuesEqual(current[key], profile.Values[key]) {
				misses++
			}
		}
		if misses < bestMiss || (misses == bestMiss && profile.Name == base) {
			best, bestMiss = profile.Name, misses
		}
	}
	return best
}

// ProfilePayloadForSettings returns the stable Control contract. The weather
// policy arrives as data from the Weather boundary, avoiding a package edge.
func (s *Service) ProfilePayloadForSettings(values map[string]any, weatherRefresh map[string]any) map[string]any {
	base := s.ProfileBaseForSettings(values)
	diverged := profileDivergedKeys(base, values)
	changed := profileChangedSettings(base, values)
	custom := len(changed) > 0
	current := base
	label, detail := ProfileMeta(base)
	if custom {
		current = "custom"
		label = "Custom"
		detail = "Based on " + profileLabel(base) + " · adjusted " + fmt.Sprintf("%d", len(diverged)) + " setting(s)"
	}
	return map[string]any{
		"current": current, "base": base, "custom": custom, "diverged": diverged, "changedSettings": changed,
		"nearest": nearestProfile(base, values), "label": label, "detail": detail,
		"values": ProfileValuesFor(base, values), "settings": values,
		"options": profileOptionsPayload(), "weatherRefresh": weatherRefresh,
	}
}

func profileKeyAllowed(key string) bool {
	_, ok := profileOwnedKeySet[key]
	return ok
}

func profileBooleanValue(key string, raw any) (bool, error) {
	if value, ok := raw.(bool); ok {
		return value, nil
	}
	if text, ok := raw.(string); ok {
		switch strings.ToLower(strings.TrimSpace(text)) {
		case "1", "true", "0", "false":
			return jsonutil.Truthy(text), nil
		}
	}
	return false, fmt.Errorf("profile.%s must be true or false", key)
}

// ClampProfileWeatherAlerts keeps alert visibility user-controlled while
// preserving the safe five-minute automatic background cadence.
func ClampProfileWeatherAlerts(raw any) (map[string]any, error) {
	values, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("profile.weatherAlerts must be an object")
	}
	for key := range values {
		if key != "enabled" && key != "minSeverity" && key != "refreshMinutes" {
			return nil, fmt.Errorf("profile.weatherAlerts.%s cannot be changed here", key)
		}
	}
	enabled := true
	if rawEnabled, present := values["enabled"]; present {
		var err error
		enabled, err = profileBooleanValue("weatherAlerts.enabled", rawEnabled)
		if err != nil {
			return nil, err
		}
	}
	severity := "moderate"
	if rawSeverity, present := values["minSeverity"]; present {
		text, ok := rawSeverity.(string)
		severity = strings.ToLower(strings.TrimSpace(text))
		if !ok || (severity != "extreme" && severity != "severe" && severity != "moderate" && severity != "minor") {
			return nil, fmt.Errorf("profile.weatherAlerts.minSeverity is not supported")
		}
	}
	return map[string]any{"enabled": enabled, "refreshMinutes": 5, "minSeverity": severity}, nil
}

func clampProfileValue(key string, raw any) (any, error) {
	if !profileKeyAllowed(key) {
		return nil, fmt.Errorf("profile.%s cannot be changed here", key)
	}
	switch key {
	case "showSeconds", "showInteractiveMaps":
		return profileBooleanValue(key, raw)
	case "weatherAlerts":
		return ClampProfileWeatherAlerts(raw)
	}
	bounds, ok := profileValueRanges[key]
	if !ok {
		return nil, fmt.Errorf("profile.%s is not a numeric profile setting", key)
	}
	value, ok := NumberValue(raw)
	if !ok || !finiteWhole(value) {
		return nil, fmt.Errorf("profile.%s must be a whole number", key)
	}
	if value < bounds[0] || value > bounds[1] {
		return nil, fmt.Errorf("profile.%s must be between %g and %g", key, bounds[0], bounds[1])
	}
	return int(value), nil
}

// NormalizeProfileSet accepts only deliberate performance controls.
func NormalizeProfileSet(set map[string]any) (map[string]any, error) {
	if len(set) == 0 {
		return nil, fmt.Errorf("profile set requires at least one setting")
	}
	keys := slices.Sorted(maps.Keys(set))
	out := make(map[string]any, len(set))
	for _, key := range keys {
		value, err := clampProfileValue(key, set[key])
		if err != nil {
			return nil, err
		}
		out[key] = value
	}
	return out, nil
}

// UpdateProfileValues applies reviewed Custom overrides and persists the real
// base profile rather than a synthetic Custom tier.
func (s *Service) UpdateProfileValues(set map[string]any) (map[string]any, error) {
	clean, err := NormalizeProfileSet(set)
	if err != nil {
		return nil, err
	}
	return s.Update(func(values map[string]any) {
		values["profile"] = s.ProfileBaseForSettings(values)
		for key, value := range clean {
			values[key] = value
		}
	})
}

// ApplyProfilePreset resets only profile-owned values, preserving household and
// unrelated settings exactly as before.
func (s *Service) ApplyProfilePreset(name string) (map[string]any, error) {
	profile, ok := ProfileByName(name)
	if !ok {
		return nil, fmt.Errorf("unknown performance profile")
	}
	return s.Update(func(values map[string]any) {
		for key, value := range profile.Values {
			values[key] = value
		}
		values["profile"] = profile.Name
	})
}
