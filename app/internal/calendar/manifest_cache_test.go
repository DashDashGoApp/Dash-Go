package calendar

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func jsonutilList(v any) []any { return jsonutil.List(v) }

// TestCalendarsPicksUpExternalManifestRegeneration is the explicit stale-cache
// gate for the /api/calendars GET: a CLI process that rewrites the manifest on
// disk (as --update-holidays / --update-iss-passes do) must be picked up on
// the next GET, while a warm GET performs no rewrite of its own.
func TestCalendarsPicksUpExternalManifestRegeneration(t *testing.T) {
	root := t.TempDir()
	dash, home := filepath.Join(root, "dashboard"), filepath.Join(root, "home")
	for _, dir := range []string{dash, home, filepath.Join(dash, "calendars"), filepath.Join(dash, "cache"), filepath.Join(dash, "logs")} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	cfg := ServiceConfig{
		DashDir: dash, HomeDir: home, CalendarDir: filepath.Join(dash, "calendars"), CacheDir: filepath.Join(dash, "cache"), LogDir: filepath.Join(dash, "logs"),
		Now: func() time.Time { return time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC) },
	}
	server := New(cfg)
	if err := os.WriteFile(filepath.Join(cfg.CalendarDir, "work.ics"), testCalendarBody("Work"), 0644); err != nil {
		t.Fatal(err)
	}
	if rows := jsonutilList(server.Calendars()); len(rows) != 1 {
		t.Fatalf("initial manifest rows=%#v", rows)
	}

	// Simulate a separate CLI process adding a source and regenerating.
	external := New(cfg)
	if err := os.WriteFile(filepath.Join(cfg.CalendarDir, "school.ics"), testCalendarBody("School"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := external.GenerateManifest(); err != nil {
		t.Fatal(err)
	}
	rows := jsonutilList(server.Calendars())
	if len(rows) != 2 {
		t.Fatalf("externally regenerated manifest not picked up on next GET: %#v", rows)
	}
}

func TestCalendarsWarmPathServesCacheWithoutRewriting(t *testing.T) {
	service := testService(t)
	calDir := service.CalendarDir()
	if err := os.WriteFile(filepath.Join(calDir, "work.ics"), testCalendarBody("Work"), 0644); err != nil {
		t.Fatal(err)
	}
	first := service.Calendars()
	manifest := filepath.Join(calDir, "calendars.json")
	before, err := os.Stat(manifest)
	if err != nil {
		t.Fatal(err)
	}
	second := service.Calendars()
	after, err := os.Stat(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if !before.ModTime().Equal(after.ModTime()) || before.Size() != after.Size() {
		t.Fatal("warm GET must not rewrite the manifest file")
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("warm GET changed the payload: %#v vs %#v", first, second)
	}
}

func TestCalendarsLogsRegenerationFailure(t *testing.T) {
	root := t.TempDir()
	dash, home := filepath.Join(root, "dashboard"), filepath.Join(root, "home")
	if err := os.MkdirAll(filepath.Join(dash, "logs"), 0755); err != nil {
		t.Fatal(err)
	}
	blocker := filepath.Join(dash, "blocker")
	if err := os.WriteFile(blocker, []byte("block"), 0644); err != nil {
		t.Fatal(err)
	}
	service := New(ServiceConfig{
		DashDir: dash, HomeDir: home,
		CalendarDir: filepath.Join(blocker, "calendars"), // MkdirAll cannot succeed under a regular file
		CacheDir:    filepath.Join(dash, "cache"), LogDir: filepath.Join(dash, "logs"),
		Now: func() time.Time { return time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC) },
	})
	rows := jsonutilList(service.Calendars())
	if len(rows) != 0 {
		t.Fatalf("failed regeneration must serve the previous (empty) manifest: %#v", rows)
	}
	logBody, err := os.ReadFile(filepath.Join(dash, "logs", "calendars.log"))
	if err != nil || !strings.Contains(string(logBody), "WARN manifest regeneration failed") {
		t.Fatalf("regeneration failure was not logged: %q (%v)", string(logBody), err)
	}
}
