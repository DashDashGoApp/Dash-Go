package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// The Maps bounded context keeps request orchestration, cache-schema handling,
// deterministic query cleanup, and provider networking separate. The core
// facade is intentionally thin and may not reclaim those responsibilities.
func TestMapGeocodeRuntimeResponsibilitiesStaySplit(t *testing.T) {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate map geocode source test")
	}
	serverRoot := filepath.Dir(thisFile)
	mapsRoot := filepath.Join(serverRoot, "..", "..", "internal", "maps")
	files := map[string][]string{
		"maps_geocode.go": {
			"func (s *Service) eventMapLookup",
			"func mapQueryKey",
		},
		"maps_geocode_cache.go": {
			"func (s *Service) cachedEventMap",
			"func (s *Service) decorateMapData",
		},
		"maps_geocode_queries.go": {
			"func eventMapQueryVariants",
			"func cleanLocationPiece",
		},
		"maps_geocode_providers.go": {
			"func (s *Service) nominatimSearch",
			"func (s *Service) censusSearch",
			"func (s *Service) geocode(q string)",
		},
	}
	for name, required := range files {
		body, err := os.ReadFile(filepath.Join(mapsRoot, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if lines := bytes.Count(body, []byte("\n")) + 1; lines > 400 {
			t.Fatalf("%s grew to %d lines", name, lines)
		}
		for _, needle := range required {
			if !bytes.Contains(body, []byte(needle)) {
				t.Fatalf("%s is missing %q", name, needle)
			}
		}
	}
	orchestrator, err := os.ReadFile(filepath.Join(mapsRoot, "maps_geocode.go"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(orchestrator, []byte("http.Client")) || bytes.Contains(orchestrator, []byte("http.NewRequest")) {
		t.Fatal("event-map lookup orchestration must not absorb provider network implementation")
	}
	facade, err := os.ReadFile(filepath.Join(serverRoot, "maps_facade.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(facade, []byte("mapspkg.New")) || !bytes.Contains(facade, []byte("EventLookup")) {
		t.Fatal("maps facade must construct and delegate to the Maps service")
	}
	if bytes.Contains(facade, []byte("http.Client")) || bytes.Contains(facade, []byte("os.ReadFile")) {
		t.Fatal("maps facade must not absorb provider or cache implementation")
	}
}
