package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func TestOwnedCalendarManifestUsesCanonicalMetadataAndVisibility(t *testing.T) {
	a := newTempCalendarApp(t)
	for _, name := range []string{"chore-wheel.ics", "maintenance.ics"} {
		if err := os.WriteFile(filepath.Join(a.calDir, name), []byte("BEGIN:VCALENDAR\nEND:VCALENDAR\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := fileio.WriteJSON(filepath.Join(a.calDir, "calendars.json"), []any{
		map[string]any{"url": "calendars/chore-wheel.ics", "name": "Chore wheel", "enabled": false},
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.generateCalendarManifest(); err != nil {
		t.Fatal(err)
	}

	entries := jsonutil.List(a.readJSONDefault(filepath.Join(a.calDir, "calendars.json"), []any{}))
	byURL := map[string]map[string]any{}
	for _, raw := range entries {
		row := jsonutil.Map(raw)
		byURL[strOr(row["url"], "")] = row
	}
	chore := byURL["calendars/chore-wheel.ics"]
	if chore["name"] != "Chores" || chore["color"] != "#7fc4c4" || calendarEntryEnabled(chore) {
		t.Fatalf("chore manifest metadata/visibility = %#v", chore)
	}
	maintenance := byURL["calendars/maintenance.ics"]
	if maintenance["name"] != "Maintenance" || maintenance["color"] != "#d9c074" || !calendarEntryEnabled(maintenance) {
		t.Fatalf("maintenance manifest metadata = %#v", maintenance)
	}

	for _, source := range a.loadEventCalendars() {
		if calendarSourceIdentity(source.URL) == "calendars/chore-wheel.ics" {
			t.Fatalf("explicitly disabled Chores feed was re-added: %#v", source)
		}
	}
}

func TestLoadEventCalendarsLoadsOwnedFeedOnce(t *testing.T) {
	a := newTempCalendarApp(t)
	if err := os.WriteFile(filepath.Join(a.calDir, "chore-wheel.ics"), []byte("BEGIN:VCALENDAR\nEND:VCALENDAR\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := a.loadEventCalendars(); len(got) != 1 || got[0].Name != "Chores" || got[0].Color != "#7fc4c4" {
		t.Fatalf("fallback owned source = %#v", got)
	}
	if err := fileio.WriteJSON(filepath.Join(a.calDir, "calendars.json"), []any{
		map[string]any{"url": "calendars/chore-wheel.ics", "name": "Chore wheel", "color": "#8fc4a6", "enabled": true},
		map[string]any{"url": "calendars/./chore-wheel.ics", "name": "Chores", "color": "#7fc4c4", "enabled": true},
	}); err != nil {
		t.Fatal(err)
	}
	got := a.loadEventCalendars()
	if len(got) != 1 || got[0].Name != "Chore wheel" || got[0].Color != "#8fc4a6" {
		t.Fatalf("configured Chores feed was not deduplicated: %#v", got)
	}
}
