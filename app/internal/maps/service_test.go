package maps

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func testService(t *testing.T) *Service {
	t.Helper()
	root := t.TempDir()
	return New(ServiceConfig{
		CacheDir:  filepath.Join(root, "cache"),
		ConfigDir: filepath.Join(root, "config"),
		LogDir:    filepath.Join(root, "logs"),
		LoadSettings: func() map[string]any {
			return map[string]any{}
		},
	})
}

func TestServiceCacheStatusInvalidatesAfterMaintenance(t *testing.T) {
	service := testService(t)
	if err := os.MkdirAll(service.ImageDir(), 0755); err != nil {
		t.Fatal(err)
	}
	first := service.CacheStatus()
	if got := jsonutil.Int(first["imageCount"], -1); got != 0 {
		t.Fatalf("initial image count=%d", got)
	}
	path := filepath.Join(service.ImageDir(), "standard_z15_p41.8781_m87.6298.svg")
	if err := os.WriteFile(path, []byte("<svg/>"), 0644); err != nil {
		t.Fatal(err)
	}
	withinTTL := service.CacheStatus()
	if got := jsonutil.Int(withinTTL["imageCount"], -1); got != 0 {
		t.Fatalf("map snapshot unexpectedly walked cache within TTL: %d", got)
	}
	service.InvalidateStatusCache()
	fresh := service.CacheStatus()
	if got := jsonutil.Int(fresh["imageCount"], -1); got != 1 {
		t.Fatalf("invalidated map snapshot count=%d want 1", got)
	}
}

func TestServiceOwnsMapPaths(t *testing.T) {
	service := testService(t)
	if got, want := filepath.Base(service.ImageDir()), "map-cache-img"; got != want {
		t.Fatalf("image directory=%q want %q", got, want)
	}
	if got, want := filepath.Base(service.TileDir()), "map-cache-tiles"; got != want {
		t.Fatalf("tile directory=%q want %q", got, want)
	}
	if got, want := filepath.Base(service.CacheFile()), "map-cache.json"; got != want {
		t.Fatalf("cache file=%q want %q", got, want)
	}
	if got, want := filepath.Base(service.ProviderFile()), "map-provider.json"; got != want {
		t.Fatalf("provider file=%q want %q", got, want)
	}
}
