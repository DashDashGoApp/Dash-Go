package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCalendarDomainBoundaryOwnsManagementWithoutCoreDependency(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate test source")
	}
	serverRoot := filepath.Dir(thisFile)
	projectRoot := filepath.Clean(filepath.Join(serverRoot, "..", ".."))
	facade, err := os.ReadFile(filepath.Join(serverRoot, "calendar_facade.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`calendarpkg.New(calendarpkg.ServiceConfig{`,
		`OutputEnabled:        a.appCalendarOutputEnabled,`,
		`SetAppOutput:         a.setAppCalendarOutputState,`,
		`RefreshCacheSync:     a.refreshEventCacheAfterCalendarWrite,`,
		`GenerateMoon:         a.generateMoonCalendar,`,
		`calendarOutputSnapshot`,
	} {
		if !strings.Contains(string(facade), want) {
			t.Fatalf("calendar facade lost required seam %q", want)
		}
	}
	for _, retired := range []string{
		"calendars_defaults.go", "calendars_holidays.go", "calendars_iss.go", "calendars_pickup.go",
		"calendars_trash.go", "calendars_owned.go", "calendars_management.go",
	} {
		if _, err := os.Stat(filepath.Join(serverRoot, retired)); !os.IsNotExist(err) {
			t.Fatalf("retired main-package calendar implementation survived: %s (%v)", retired, err)
		}
	}
	serviceRoot := filepath.Join(projectRoot, "internal", "calendar")
	seen := 0
	err = filepath.WalkDir(serviceRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		seen++
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(body), "cmd/dashboard-control-server") || strings.Contains(string(body), "*app") {
			t.Fatalf("calendar service leaked a core dependency in %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if seen == 0 {
		t.Fatal("calendar service source is missing")
	}
}
