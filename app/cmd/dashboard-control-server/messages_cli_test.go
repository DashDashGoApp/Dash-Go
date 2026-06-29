package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func TestMessageSourcesCLIUsesCanonicalValidationAndWritesPrefs(t *testing.T) {
	a := testApp(t)
	if got := a.runMessageSourcesCLI([]string{"--set", "quotes,jokes"}); got != 0 {
		t.Fatalf("set current message sources returned %d", got)
	}
	prefs := jsonutil.Map(a.readJSONDefault(filepath.Join(a.configDir, "message-sources.json"), map[string]any{}))
	enabled := jsonutil.List(prefs["enabled"])
	if len(enabled) != 2 || enabled[0] != "jokes" || enabled[1] != "quotes" {
		t.Fatalf("canonical source preferences were not written: %#v", prefs)
	}
	if got := a.runMessageSourcesCLI([]string{"--set", "quotes-calm"}); got != 64 {
		t.Fatalf("retired source returned %d, want 64", got)
	}
	if got := a.runMessageSourcesCLI([]string{"--set", "unknown-source"}); got != 64 {
		t.Fatalf("unknown source returned %d, want 64", got)
	}
	if got := a.runMessageSourcesCLI([]string{"--set", ""}); got != 0 {
		t.Fatalf("clearing source preferences returned %d", got)
	}
	b, err := os.ReadFile(filepath.Join(a.configDir, "message-sources.json"))
	if err != nil {
		t.Fatal(err)
	}
	got := jsonutil.Map(a.readJSONDefault(filepath.Join(a.configDir, "message-sources.json"), map[string]any{}))
	if len(jsonutil.List(got["enabled"])) != 0 {
		t.Fatalf("expected empty enabled list after clear: %s", b)
	}
}
