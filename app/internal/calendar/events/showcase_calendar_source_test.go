package events

import (
	"path/filepath"
	"testing"
)

func TestCalendarSourcesResolveFromConfiguredCalendarDirectory(t *testing.T) {
	root := t.TempDir()
	service := New(ServiceConfig{
		DashDir:     filepath.Join(root, "immutable-app"),
		CalendarDir: filepath.Join(root, "scenario", "calendars"),
	})
	want := filepath.Join(service.CalendarDir(), "family.green.ics")
	if got := service.eventURLToPath("calendars/family.green.ics"); got != want {
		t.Fatalf("calendar source = %q, want %q", got, want)
	}
	if got := service.eventURLToPath("calendars/../outside.ics"); got != "" {
		t.Fatalf("calendar traversal escaped configured calendar directory: %q", got)
	}
	if got := service.eventURLToPath("ui/example.ics"); got != filepath.Join(root, "immutable-app", "ui", "example.ics") {
		t.Fatalf("immutable asset source = %q", got)
	}
}
