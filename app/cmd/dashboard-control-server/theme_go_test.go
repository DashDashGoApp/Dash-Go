package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestThemeNameAcceptsControlPayloadName(t *testing.T) {
	a := testApp(t)
	if got := a.themeNameFromBody(map[string]any{"name": "paper"}); got != "paper" {
		t.Fatalf("expected name payload to win, got %q", got)
	}
	if got := a.themeNameFromBody(map[string]any{"theme": "ruby"}); got != "ruby" {
		t.Fatalf("expected legacy theme payload, got %q", got)
	}
	if got := a.themeNameFromBody(map[string]any{}); got != "" {
		t.Fatalf("expected empty missing theme, got %q", got)
	}
}

func TestWriteThemeCreatesAndUpdatesConfigLocal(t *testing.T) {
	a := testApp(t)
	if err := os.Remove(a.configLocal); err != nil {
		t.Fatal(err)
	}
	if err := a.writeTheme("paper"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(a.configLocal)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `theme: "paper"`) {
		t.Fatalf("created config did not contain theme paper: %s", string(b))
	}
	if err := a.writeTheme("chalkboard"); err != nil {
		t.Fatal(err)
	}
	b, err = os.ReadFile(a.configLocal)
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	if !strings.Contains(text, `theme: "chalkboard"`) {
		t.Fatalf("updated config did not contain theme chalkboard: %s", text)
	}
	if strings.Contains(text, `theme: "paper"`) {
		t.Fatalf("old theme remained after update: %s", text)
	}
}

func TestValidThemeReadsSharedThemeCatalog(t *testing.T) {
	a := testApp(t)
	if err := os.WriteFile(filepath.Join(a.dash, "themes.list"), []byte("basic\npaper\nchalkboard\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if !a.validTheme("paper") {
		t.Fatal("paper should be a valid theme")
	}
	if a.validTheme("not-a-theme") {
		t.Fatal("not-a-theme should not be valid")
	}
}
