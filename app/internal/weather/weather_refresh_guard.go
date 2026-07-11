package weather

import (
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

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

// weatherRefreshProfileDefaultMinutes controls how often the dashboard checks
// the aggregate forecast. Each provider cache separately enforces its own
// quota-safe minimum before a live request is attempted.
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
	providerMinutes := map[string]any{}
	for _, id := range providers {
		providerMinutes[id] = max(base, weatherProviderRefreshMinimumMinutes(id))
	}
	return map[string]any{
		"automatic":              true,
		"minimumMinutes":         weatherRefreshHardMinimumMinutes,
		"profileDefaultMinutes":  base,
		"guardedProviders":       guarded,
		"providerMinutes":        providerMinutes,
		"slowestProviderMinutes": minimum,
		"effectiveMinutes":       weatherRefreshEffectiveMinutes(settings, weatherRefreshHardMinimumMinutes),
	}
}

// The dashboard checks the aggregate forecast at its profile cadence. Each
// provider cache independently enforces its own quota-safe minimum, so a
// low-quota source no longer makes every other selected source equally stale.
func weatherRefreshEffectiveMinutes(settings map[string]any, minimum int) int {
	profile := normalizeProfileName(strOr(settings["profile"], "balanced"))
	return max(weatherRefreshProfileDefaultMinutes(profile), minimum)
}

func (s *Service) weatherRefreshMinutes() int {
	settings := s.loadSettings()
	return weatherRefreshEffectiveMinutes(settings, weatherRefreshHardMinimumMinutes)
}

func (s *Service) weatherProviderFreshTTLGo(id string) time.Duration {
	profile := normalizeProfileName(jsonutil.StringValue(s.profilePayload()["base"]))
	if profile == "" {
		profile = normalizeProfileName(jsonutil.StringValue(s.profilePayload()["current"]))
	}
	minutes := max(weatherRefreshProfileDefaultMinutes(profile), weatherProviderRefreshMinimumMinutes(id))
	return time.Duration(minutes) * time.Minute
}
