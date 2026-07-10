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

func TestRefreshPersistsAndUsesCacheMetadataFastPath(t *testing.T) {
	s := testService(t)
	ics := "BEGIN:VCALENDAR\nBEGIN:VEVENT\nUID:fast-path@test\nDTSTART:20260621T120000\nDTEND:20260621T123000\nSUMMARY:Fast path\nEND:VEVENT\nEND:VCALENDAR\n"
	if err := os.WriteFile(filepath.Join(s.CalendarDir(), "personal.green.ics"), []byte(ics), 0644); err != nil {
		t.Fatal(err)
	}
	first, err := s.Refresh(true, 30, 60)
	if err != nil || jsonutil.Truthy(first["unchanged"]) {
		t.Fatalf("first refresh=%#v err=%v", first, err)
	}
	metaPath := filepath.Join(s.CacheDir(), ".events-cache.meta.json")
	meta, ok := readCacheMetadata(metaPath)
	if !ok {
		t.Fatal("cache metadata was not readable")
	}
	if meta.CacheVersion != CacheVersion || meta.FingerprintVersion != FingerprintVersion || meta.EventCount != 1 || meta.CacheSize <= 0 || meta.CacheMtimeNs <= 0 {
		t.Fatalf("cache metadata summary=%#v", meta)
	}
	second, err := s.Refresh(false, 30, 60)
	if err != nil || !jsonutil.Truthy(second["unchanged"]) || jsonutil.Int(second["eventCount"], -1) != 1 {
		t.Fatalf("fast-path refresh=%#v err=%v", second, err)
	}
	metaAfter, ok := readCacheMetadata(metaPath)
	if !ok || metaAfter.CacheSize != meta.CacheSize || metaAfter.CacheMtimeNs != meta.CacheMtimeNs {
		t.Fatalf("fast-path metadata=%#v", metaAfter)
	}
}

func TestRefreshUpgradesLegacyMetadataAndRecoversFromInvalidExternalEdit(t *testing.T) {
	s := testService(t)
	ics := "BEGIN:VCALENDAR\nBEGIN:VEVENT\nUID:legacy-meta@test\nDTSTART:20260621T120000\nSUMMARY:Legacy metadata\nEND:VEVENT\nEND:VCALENDAR\n"
	if err := os.WriteFile(filepath.Join(s.CalendarDir(), "personal.green.ics"), []byte(ics), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Refresh(true, 30, 60); err != nil {
		t.Fatal(err)
	}
	metaPath := filepath.Join(s.CacheDir(), ".events-cache.meta.json")
	meta, ok := readCacheMetadata(metaPath)
	if !ok {
		t.Fatal("cache metadata was not readable")
	}
	legacy := map[string]any{
		"fingerprint":        meta.Fingerprint,
		"fingerprintVersion": meta.FingerprintVersion,
		"updatedAt":          meta.UpdatedAt,
		"lastSuccessAt":      meta.LastSuccessAt,
		"generator":          "go",
	}
	if err := fileio.WriteJSON(metaPath, legacy); err != nil {
		t.Fatal(err)
	}
	upgradedResult, err := s.Refresh(false, 30, 60)
	if err != nil || !jsonutil.Truthy(upgradedResult["unchanged"]) {
		t.Fatalf("legacy metadata refresh=%#v err=%v", upgradedResult, err)
	}
	upgraded, ok := readCacheMetadata(metaPath)
	if !ok || upgraded.CacheVersion != CacheVersion || upgraded.CacheSize <= 0 || upgraded.CacheMtimeNs <= 0 || upgraded.EventCount != 1 {
		t.Fatalf("upgraded metadata=%#v", upgraded)
	}

	cachePath := filepath.Join(s.CacheDir(), "events.cache.json")
	body, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cachePath, append(body, 'x'), 0644); err != nil {
		t.Fatal(err)
	}
	recovered, err := s.Refresh(false, 30, 60)
	if err != nil || jsonutil.Truthy(recovered["unchanged"]) {
		t.Fatalf("invalid external edit recovery=%#v err=%v", recovered, err)
	}
	cache := jsonutil.Map(readJSONDefault(cachePath, map[string]any{}))
	if jsonutil.Int(cache["version"], 0) != CacheVersion || len(jsonutil.List(cache["events"])) != 1 {
		t.Fatalf("recovered cache=%#v", cache)
	}
}

func TestPrivateCalendarCapabilityIsCachedIndependentlyFromMasterEditState(t *testing.T) {
	s := testService(t)
	const source = "calendars/private.green.ics"
	ics := "BEGIN:VCALENDAR\nBEGIN:VEVENT\nUID:private-one@test\nDTSTART:20260621T120000\nDTEND:20260621T123000\nSUMMARY:Private one-time event\nEND:VEVENT\nEND:VCALENDAR\n"
	if err := os.WriteFile(filepath.Join(s.CalendarDir(), "private.green.ics"), []byte(ics), 0644); err != nil {
		t.Fatal(err)
	}
	fingerprint := "master-off"
	s.knownWritebackSource = func(url string) bool { return url == source }
	s.capabilityFingerprint = func() string { return fingerprint }

	result, err := s.Refresh(true, 30, 60)
	if err != nil {
		t.Fatal(err)
	}
	if jsonutil.Truthy(result["unchanged"]) {
		t.Fatalf("forced refresh unexpectedly unchanged: %#v", result)
	}
	cache := jsonutil.Map(readJSONDefault(filepath.Join(s.CacheDir(), "events.cache.json"), map[string]any{}))
	events := jsonutil.List(cache["events"])
	if len(events) != 1 {
		t.Fatalf("events=%#v", events)
	}
	writeback := jsonutil.Map(jsonutil.Map(events[0])["writeback"])
	if !jsonutil.Truthy(writeback["candidate"]) || !jsonutil.Truthy(writeback["canEdit"]) {
		t.Fatalf("private one-time event did not retain static writeback eligibility: %#v", writeback)
	}

	fingerprint = "master-on"
	result, err = s.Refresh(false, 30, 60)
	if err != nil {
		t.Fatal(err)
	}
	if jsonutil.Truthy(result["unchanged"]) {
		t.Fatalf("writeback capability fingerprint change must rebuild cache: %#v", result)
	}
}
