package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func writeShowcaseContractFixture(t *testing.T, dash string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dash, "release"), 0755); err != nil {
		t.Fatal(err)
	}
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not resolve Showcase contract test source path")
	}
	fixturePath := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", "..", "release", "showcase-contract.json"))
	fixture, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read Showcase contract fixture %s: %v", fixturePath, err)
	}
	if err := os.WriteFile(filepath.Join(dash, "release", "showcase-contract.json"), fixture, 0644); err != nil {
		t.Fatal(err)
	}
}

func writeShowcaseCalendarFixture(t *testing.T, path string, uid string) {
	t.Helper()
	start := time.Now().Add(24 * time.Hour).UTC().Format("20060102T150405Z")
	end := time.Now().Add(25 * time.Hour).UTC().Format("20060102T150405Z")
	body := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:" + uid + "\r\nDTSTART:" + start + "\r\nDTEND:" + end + "\r\nSUMMARY:Showcase planning\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

func newShowcaseTestApp(t *testing.T) *app {
	t.Helper()
	root := t.TempDir()
	dash := filepath.Join(root, "app")
	data := filepath.Join(root, "scenario")
	writeShowcaseContractFixture(t, dash)
	if err := os.MkdirAll(filepath.Join(data, "calendars"), 0755); err != nil {
		t.Fatal(err)
	}
	collection := "home/.dashboard-vdirsyncer/collections/showcase-family"
	manifest := showcaseScenarioManifest{
		Schema:   1,
		Contract: showcaseContractName,
		Profile:  showcaseProfileName,
		Scenario: "contract-test",
		Cache:    showcaseCacheWindow{DaysPast: 1, DaysFuture: 14},
		Calendars: []showcaseCalendar{{
			Source:         "calendars/family.green.ics",
			Name:           "Family",
			Color:          "#3aa981",
			Collection:     collection,
			Writable:       true,
			Enabled:        true,
			ExpectedEvents: 1,
		}},
	}
	manifestPath := filepath.Join(data, "showcase-manifest.json")
	body, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, body, 0600); err != nil {
		t.Fatal(err)
	}
	writeShowcaseCalendarFixture(t, filepath.Join(data, "calendars", "family.green.ics"), "showcase-family")

	t.Setenv("DASHGO_RUNTIME_PROFILE", showcaseProfileName)
	t.Setenv("DASHGO_SHOWCASE_MANIFEST", manifestPath)
	t.Setenv("DASHGO_DATA_ROOT", data)
	runtime, err := loadShowcaseRuntime(dash)
	if err != nil {
		t.Fatalf("load Showcase runtime: %v", err)
	}
	a := &app{
		dash:         dash,
		home:         runtime.homeDir,
		configDir:    filepath.Join(data, "config"),
		calDir:       filepath.Join(data, "calendars"),
		cacheDir:     filepath.Join(data, "cache"),
		logDir:       filepath.Join(data, "logs"),
		binDir:       filepath.Join(dash, "bin"),
		settingsFile: filepath.Join(data, "config", "settings.json"),
		configLocal:  filepath.Join(data, "config", "config.local.js"),
		todoDir:      filepath.Join(data, "config", "todo"),
		fontsDir:     filepath.Join(dash, "fonts"),
		showcase:     runtime,
		todoStreams:  map[chan []byte]bool{},
	}
	for _, path := range []string{a.home, a.configDir, a.calDir, a.cacheDir, a.logDir, a.todoDir} {
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Fatal(err)
		}
	}
	return a
}

func TestShowcaseContractBootstrapsManifestWritebackAndCache(t *testing.T) {
	a := newShowcaseTestApp(t)
	if err := a.initializeShowcaseRuntime(); err != nil {
		t.Fatalf("initialize Showcase runtime: %v", err)
	}
	status := a.showcaseStatus()
	if status["ready"] != true {
		t.Fatalf("Showcase status is not ready: %#v", status)
	}
	if status["contract"] != showcaseContractName || status["profile"] != showcaseProfileName {
		t.Fatalf("Showcase status did not expose the contract identity: %#v", status)
	}
	cache := status["cache"].(map[string]any)
	if cache["events"].(int) < 1 || cache["writebackCandidates"].(int) < 1 {
		t.Fatalf("Showcase cache did not expose the expected event/writeback counts: %#v", cache)
	}
	calendars := status["calendars"].([]map[string]any)
	if len(calendars) != 1 || calendars[0]["writebackRegistered"] != true {
		t.Fatalf("Showcase registry did not expose the manifest-owned writable calendar: %#v", calendars)
	}
	if _, err := os.Stat(filepath.Join(a.configDir, calendarWritebackRegistryFile)); err != nil {
		t.Fatalf("Showcase writeback registry was not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(a.cacheDir, "events.cache.json")); err != nil {
		t.Fatalf("Showcase event cache was not created: %v", err)
	}
}

func TestShowcaseContractRejectsAppRootDataAndEscapingCollection(t *testing.T) {
	root := t.TempDir()
	writeShowcaseContractFixture(t, root)
	manifestPath := filepath.Join(root, "showcase-manifest.json")
	if err := os.WriteFile(manifestPath, []byte(`{"schema":1,"contract":"dashgo-showcase/v1","profile":"showcase","scenario":"bad","cache":{"daysPast":0,"daysFuture":1},"calendars":[{"source":"calendars/family.green.ics","name":"Family","color":"#3aa981","collection":"home/.dashboard-vdirsyncer/collections/../escape","writable":true,"enabled":true,"expectedEvents":0}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DASHGO_RUNTIME_PROFILE", showcaseProfileName)
	t.Setenv("DASHGO_SHOWCASE_MANIFEST", manifestPath)
	t.Setenv("DASHGO_DATA_ROOT", root)
	if _, err := loadShowcaseRuntime(root); err == nil {
		t.Fatal("Showcase runtime accepted an app-root data directory")
	}
}

func TestShowcaseStaticDataPathsStayAllowlisted(t *testing.T) {
	a := newShowcaseTestApp(t)
	if _, ok := a.showcaseStaticDataPath("calendars/family.green.ics"); !ok {
		t.Fatal("manifest-declared Showcase calendar was not exposed")
	}
	windowsPath, ok := a.showcaseStaticDataPath(`\calendars\family.green.ics`)
	if !ok {
		t.Fatal("manifest-declared Showcase calendar was not exposed through Windows-shaped separators")
	}
	wantWindowsPath := filepath.Join(a.calDir, "family.green.ics")
	if windowsPath != wantWindowsPath {
		t.Fatalf("Windows-shaped Showcase calendar path = %q, want %q", windowsPath, wantWindowsPath)
	}
	if _, ok := a.showcaseStaticDataPath("config/family-board.json"); ok {
		t.Fatal("private config file was exposed through the Showcase static allowlist")
	}
	if _, ok := a.showcaseStaticDataPath("../outside"); ok {
		t.Fatal("path traversal was exposed through the Showcase static allowlist")
	}
	if _, ok := a.showcaseStaticDataPath("/C:/outside"); ok {
		t.Fatal("drive-qualified path was exposed through the Showcase static allowlist")
	}
}

func TestShowcaseContractStatusEndpointAndStaticCalendar(t *testing.T) {
	a := newShowcaseTestApp(t)
	if err := a.initializeShowcaseRuntime(); err != nil {
		t.Fatalf("initialize Showcase runtime: %v", err)
	}

	statusRequest := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8090/api/showcase/status", nil)
	statusRequest.RemoteAddr = "127.0.0.1:41712"
	statusResponse := httptest.NewRecorder()
	a.handle(statusResponse, statusRequest)
	if statusResponse.Code != http.StatusOK {
		t.Fatalf("Showcase status endpoint returned %d: %s", statusResponse.Code, statusResponse.Body.String())
	}
	var status map[string]any
	if err := json.Unmarshal(statusResponse.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status["ready"] != true {
		t.Fatalf("Showcase status endpoint is not ready: %#v", status)
	}
	cache := status["cache"].(map[string]any)
	if cache["rebuilt"] != true {
		t.Fatalf("Showcase status endpoint did not report a rebuilt cache: %#v", cache)
	}

	if err := os.WriteFile(a.configLocal, []byte("// Generated for Dash-Go Showcase Studio.\nwindow.DASHBOARD_LOCAL={};\n"), 0644); err != nil {
		t.Fatal(err)
	}
	configRequest := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8090/config/config.local.js", nil)
	configResponse := httptest.NewRecorder()
	a.handle(configResponse, configRequest)
	if configResponse.Code != http.StatusOK || !strings.Contains(configResponse.Body.String(), "Generated for Dash-Go Showcase Studio") {
		t.Fatalf("Showcase static configuration was not served from the data root: %d %q", configResponse.Code, configResponse.Body.String())
	}

	calendarRequest := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8090/calendars/family.green.ics", nil)
	calendarResponse := httptest.NewRecorder()
	a.handle(calendarResponse, calendarRequest)
	if calendarResponse.Code != http.StatusOK || !strings.Contains(calendarResponse.Body.String(), "UID:showcase-family") {
		t.Fatalf("Showcase static calendar was not served from the data root: %d %q", calendarResponse.Code, calendarResponse.Body.String())
	}

	privateRequest := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8090/config/family-board.json", nil)
	privateResponse := httptest.NewRecorder()
	a.handle(privateResponse, privateRequest)
	if privateResponse.Code != http.StatusNotFound {
		t.Fatalf("Showcase private configuration path returned %d", privateResponse.Code)
	}
}

func TestShowcaseContractRejectsEscapingCollection(t *testing.T) {
	root := t.TempDir()
	dash := filepath.Join(root, "app")
	data := filepath.Join(root, "scenario")
	writeShowcaseContractFixture(t, dash)
	if err := os.MkdirAll(filepath.Join(data, "calendars"), 0755); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(data, "showcase-manifest.json")
	manifest := `{"schema":1,"contract":"dashgo-showcase/v1","profile":"showcase","scenario":"bad","cache":{"daysPast":0,"daysFuture":1},"calendars":[{"source":"calendars/family.green.ics","name":"Family","color":"#3aa981","collection":"home/.dashboard-vdirsyncer/collections/../escape","writable":true,"enabled":true,"expectedEvents":0}]}`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DASHGO_RUNTIME_PROFILE", showcaseProfileName)
	t.Setenv("DASHGO_SHOWCASE_MANIFEST", manifestPath)
	t.Setenv("DASHGO_DATA_ROOT", data)
	if _, err := loadShowcaseRuntime(dash); err == nil {
		t.Fatal("Showcase runtime accepted a collection that escapes its isolated vdir root")
	}
}

func TestShowcaseContractIsDormantWithoutExplicitProfile(t *testing.T) {
	t.Setenv("DASHGO_RUNTIME_PROFILE", "")
	t.Setenv("DASHGO_SHOWCASE_MANIFEST", "")
	t.Setenv("DASHGO_DATA_ROOT", "")
	runtime, err := loadShowcaseRuntime(t.TempDir())
	if err != nil || runtime != nil {
		t.Fatalf("ordinary Dash-Go runtime unexpectedly enabled Showcase: runtime=%#v err=%v", runtime, err)
	}
}

func TestShowcaseContractRejectsPartialEnvironment(t *testing.T) {
	t.Setenv("DASHGO_RUNTIME_PROFILE", "")
	t.Setenv("DASHGO_SHOWCASE_MANIFEST", filepath.Join(t.TempDir(), "showcase-manifest.json"))
	t.Setenv("DASHGO_DATA_ROOT", "")
	if _, err := loadShowcaseRuntime(t.TempDir()); err == nil {
		t.Fatal("Showcase manifest environment was accepted without an explicit Showcase profile")
	}
}
