package weather

import "strings"

const weatherRefreshHardMinimumMinutes = 15
const weatherRefreshAPIKeyMinimumMinutes = 30
const weatherRefreshLowQuotaMinimumMinutes = 90

// weatherProviderRefreshMinimumMinutes keeps scheduled refreshes below the
// practical request budget for each configured source. Provider cooldown/backoff
// still protects transient failures and 429 responses after they happen.
func weatherProviderRefreshMinimumMinutes(id string) int {
	switch strings.ToLower(strings.TrimSpace(id)) {
	case "openmeteo", "nws":
		return weatherRefreshHardMinimumMinutes
	case "weatherbit":
		return weatherRefreshLowQuotaMinimumMinutes
	default:
		return weatherRefreshAPIKeyMinimumMinutes
	}
}

func weatherRefreshProviders(raw any, fallback []string) []string {
	values := []string{}
	switch xs := raw.(type) {
	case []any:
		for _, value := range xs {
			values = append(values, strings.TrimSpace(strings.ToLower(strOr(value, ""))))
		}
	case []string:
		values = append(values, xs...)
	default:
		return normalizeWeatherProviderListGo(fallback)
	}
	values = normalizeWeatherProviderListGo(values)
	if len(values) == 0 {
		return normalizeWeatherProviderListGo(fallback)
	}
	return values
}

func weatherRefreshMinimumForProviders(providers []string) (int, []string) {
	minimum := weatherRefreshHardMinimumMinutes
	guarded := []string{}
	seen := map[string]bool{}
	for _, raw := range providers {
		id := weatherNormalizeProviderIDGo(raw)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		providerMinimum := weatherProviderRefreshMinimumMinutes(id)
		minimum = max(minimum, providerMinimum)
		if providerMinimum > weatherRefreshHardMinimumMinutes {
			guarded = append(guarded, weatherProviderLabel(id))
		}
	}
	return minimum, guarded
}

func (s *Service) weatherRefreshMinimumForSettings(settings map[string]any) int {
	cfg := s.Config()
	providers := weatherRefreshProviders(settings["weatherProviders"], cfg.Providers)
	minimum, _ := weatherRefreshMinimumForProviders(providers)
	return minimum
}

// weatherRefreshProfileDefaultMinutes is intentionally small and predictable.
// Provider minimums always win, so a low-quota source cannot be over-polled.
func weatherRefreshProfileDefaultMinutes(profile string) int {
	if normalizeProfileName(profile) == "lite" {
		return 45
	}
	return 30
}

// weatherRefreshPolicyForSettings receives the already-loaded settings map so
// the Settings service can request a profile payload without importing Weather.
func (s *Service) weatherRefreshPolicyForSettings(settings map[string]any) map[string]any {
	cfg := s.Config()
	providers := weatherRefreshProviders(settings["weatherProviders"], cfg.Providers)
	minimum, guarded := weatherRefreshMinimumForProviders(providers)
	base := weatherRefreshProfileDefaultMinutes(s.profileBaseForSettings(settings))
	return map[string]any{
		"automatic":             true,
		"minimumMinutes":        minimum,
		"profileDefaultMinutes": base,
		"guardedProviders":      guarded,
		"effectiveMinutes":      weatherRefreshEffectiveMinutes(settings, minimum),
	}
}

// weatherRefreshEffectiveMinutes deliberately ignores legacy user-selected
// settings. Refresh cadence is now automatic: the selected profile establishes
// a normal budget and provider limits can only slow it further.
func weatherRefreshEffectiveMinutes(settings map[string]any, minimum int) int {
	profile := normalizeProfileName(strOr(settings["profile"], "balanced"))
	requested := weatherRefreshProfileDefaultMinutes(profile)
	requested = max(requested, minimum)
	return requested
}

func (s *Service) weatherRefreshMinutes() int {
	settings := s.loadSettings()
	return weatherRefreshEffectiveMinutes(settings, s.weatherRefreshMinimumForSettings(settings))
}
