package events

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func testService(t *testing.T) *Service {
	t.Helper()
	root := t.TempDir()
	calDir := filepath.Join(root, "calendars")
	cacheDir := filepath.Join(root, "cache")
	if err := os.MkdirAll(calDir, 0755); err != nil {
		t.Fatal(err)
	}
	return New(ServiceConfig{
		DashDir:       root,
		CalendarDir:   calDir,
		CacheDir:      cacheDir,
		OutputEnabled: func(string) bool { return true },
		Now:           func() time.Time { return time.Date(2026, 6, 20, 12, 0, 0, 0, time.Local) },
	})
}

func TestParseAndExpandKeepDateOnlyExclusionBehavior(t *testing.T) {
	old := time.Local
	loc, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Skip(err)
	}
	time.Local = loc
	t.Cleanup(func() { time.Local = old })
	ics := "BEGIN:VCALENDAR\nBEGIN:VEVENT\nUID:skip-day\nDTSTART:20260606T090000\nRRULE:FREQ=WEEKLY;BYDAY=SA;COUNT=4\nEXDATE;VALUE=DATE:20260620\nSUMMARY:Skipped date\nEND:VEVENT\nEND:VCALENDAR\n"
	parsed := ParseICS(ics, CalendarSource{})
	got := Expand(parsed[0], time.Date(2026, 6, 1, 0, 0, 0, 0, time.Local), time.Date(2026, 6, 30, 23, 59, 59, 0, time.Local))
	if len(got) != 3 || got[2].Start.Format("20060102") != "20260627" {
		t.Fatalf("date-only exdate expansion=%#v", got)
	}
}

func TestRefreshPreservesCompactCacheContract(t *testing.T) {
	s := testService(t)
	ics := "BEGIN:VCALENDAR\nBEGIN:VEVENT\nUID:weekly@test\nDTSTART:20260601T120000\nDTEND:20260601T123000\nRRULE:FREQ=WEEKLY;COUNT=4;BYDAY=MO\nSUMMARY:Weekly Test\nEND:VEVENT\nEND:VCALENDAR\n"
	if err := os.WriteFile(filepath.Join(s.CalendarDir(), "personal.green.ics"), []byte(ics), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := s.Refresh(true, 30, 60)
	if err != nil || result["generator"] != "go" || jsonutil.Int(result["eventCount"], 0) == 0 {
		t.Fatalf("refresh=%#v err=%v", result, err)
	}
	cachePath := filepath.Join(s.CacheDir(), "events.cache.json")
	if !fileio.Exists(cachePath) {
		t.Fatal("cache was not written")
	}
	cache := jsonutil.Map(readJSONDefault(cachePath, map[string]any{}))
	if jsonutil.Int(cache["version"], 0) != CacheVersion || jsonutil.Int(cache["fingerprintVersion"], 0) != FingerprintVersion {
		t.Fatalf("cache contract=%#v", cache)
	}
}
