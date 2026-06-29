package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestEventDomainBoundaryUsesServiceAndCalendarPolicyFacade(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate test source")
	}
	serverRoot := filepath.Dir(thisFile)
	projectRoot := filepath.Clean(filepath.Join(serverRoot, "..", ".."))
	facade, err := os.ReadFile(filepath.Join(serverRoot, "events_facade.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`eventspkg.New(eventspkg.ServiceConfig{`,
		`OutputEnabled:  a.calendarOutputEnabledForURL,`,
		`SourceIdentity: calendarSourceIdentity,`,
		`OwnedSource:    ownedCalendarSource,`,
		`return a.eventService().Refresh(force, daysPast, daysFuture)`,
	} {
		if !strings.Contains(string(facade), want) {
			t.Fatalf("event facade lost required seam %q", want)
		}
	}
	for _, retired := range []string{
		"events_ics_parse.go", "events_recurrence.go", "events_sources.go", "events_cache_pipeline.go", "events_utils.go",
	} {
		if _, err := os.Stat(filepath.Join(serverRoot, retired)); !os.IsNotExist(err) {
			t.Fatalf("retired main-package event implementation survived: %s (%v)", retired, err)
		}
	}
	serviceRoot := filepath.Join(projectRoot, "internal", "calendar", "events")
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
			t.Fatalf("event service leaked a core dependency in %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if seen == 0 {
		t.Fatal("event service source is missing")
	}
}
