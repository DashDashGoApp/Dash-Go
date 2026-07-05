package main

import (
	"strings"
	"testing"
)

func TestSetConfigLocalFieldReplacesCompleteMultilineValue(t *testing.T) {
	body := []byte(`window.DASHBOARD_LOCAL = {
  weatherAlerts: {
    enabled: false,
    refreshMinutes: 90,
    nested: { preserve: true },
  },
  birthdays: [],
  keep: "unchanged",
};
`)
	updated, err := setConfigLocalField(body, "weatherAlerts", `{ enabled: true, refreshMinutes: 5, minSeverity: "severe" }`)
	if err != nil {
		t.Fatalf("setConfigLocalField: %v", err)
	}
	got := string(updated)
	if !strings.Contains(got, `weatherAlerts: { enabled: true, refreshMinutes: 5, minSeverity: "severe" },`) {
		t.Fatalf("updated value missing: %s", got)
	}
	if strings.Contains(got, "nested: { preserve: true }") || strings.Contains(got, "refreshMinutes: 90") {
		t.Fatalf("old multiline value was not replaced completely: %s", got)
	}
	if !strings.Contains(got, `keep: "unchanged"`) || !strings.Contains(got, "birthdays: []") {
		t.Fatalf("unrelated fields changed: %s", got)
	}
}

func TestRemoveConfigLocalFieldsTreatsKeysLiterally(t *testing.T) {
	body := []byte("window.DASHBOARD_LOCAL = {\n  weatherAlerts: { enabled: true },\n  birthdays: [],\n};\n")
	unchanged, err := removeConfigLocalFields(body, "weatherAlerts|birthdays")
	if err != nil {
		t.Fatalf("removeConfigLocalFields: %v", err)
	}
	if string(unchanged) != string(body) {
		t.Fatalf("regex-looking key changed unrelated fields: %s", unchanged)
	}
	updated, err := removeConfigLocalFields(body, "weatherAlerts")
	if err != nil {
		t.Fatalf("removeConfigLocalFields: %v", err)
	}
	if strings.Contains(string(updated), "weatherAlerts") || !strings.Contains(string(updated), "birthdays") {
		t.Fatalf("field removal mismatch: %s", updated)
	}
}
