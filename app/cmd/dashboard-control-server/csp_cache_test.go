package main

import (
	"strings"
	"testing"
)

// TestDashboardContentSecurityPolicyCachesUntilSettingsChange is the P2 gate:
// the CSP string is byte-identical across warm static-asset requests (no
// settings read, no reassembly) and is rebuilt the moment the settings change.
func TestDashboardContentSecurityPolicyCachesUntilSettingsChange(t *testing.T) {
	a := testProfileApp(t)

	// A custom tile host must survive the CSP sanitizer: https URL, no user
	// part, non-empty host — otherwise the "changed" assertion below could
	// pass trivially.
	baseSettings := map[string]any{"radarCustomTiles": "https://tiles.example.com/{z}/{x}/{y}.png"}
	a.settingsService().Write(baseSettings)
	first := a.dashboardContentSecurityPolicy()
	second := a.dashboardContentSecurityPolicy()
	if first != second {
		t.Fatalf("CSP changed across warm calls:\n%s\n%s", first, second)
	}
	if !strings.Contains(first, "https://tiles.example.com") {
		t.Fatalf("custom tile host missing from CSP: %q", first)
	}

	// A settings write that changes the effective radar values must invalidate
	// the cache and change the CSP.
	changedSettings := map[string]any{"radarCustomTiles": "https://other.example.com/{z}/{x}/{y}.png"}
	if err := a.settingsService().Write(changedSettings); err != nil {
		t.Fatal(err)
	}
	third := a.dashboardContentSecurityPolicy()
	if third == first {
		t.Fatal("CSP was not rebuilt after the radar settings changed")
	}
	if !strings.Contains(third, "https://other.example.com") {
		t.Fatalf("updated custom tile host missing from CSP: %q", third)
	}
}

// TestDashboardContentSecurityPolicySanitizesInvalidTileURLs asserts the pure
// assembly: non-https, user-bearing, and empty URLs never reach the policy.
func TestDashboardContentSecurityPolicySanitizesInvalidTileURLs(t *testing.T) {
	policy := dashboardCSPPolicyString("http://tiles.example.com/{z}/{x}/{y}.png", "https://user:pass@example.com/{z}/{x}/{y}.png")
	if strings.Contains(policy, "http://tiles.example.com") {
		t.Fatalf("non-https tile URL leaked into CSP: %q", policy)
	}
	if strings.Contains(policy, "user:pass@example.com") {
		t.Fatalf("user-bearing tile URL leaked into CSP: %q", policy)
	}
	if strings.Contains(policy, "https://example.com") {
		t.Fatalf("host of user-bearing URL leaked into CSP: %q", policy)
	}
	if !strings.Contains(policy, "https://rainviewer.com") {
		t.Fatalf("baseline sources missing from CSP: %q", policy)
	}
}
