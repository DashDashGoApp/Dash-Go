package maps

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

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

func TestMapFallbackSVGIncludesEscapedRuneSafeReason(t *testing.T) {
	reason := "provider <temporarily unavailable> & retrying"
	svg := mapFallbackSVG(41.8781, -87.6298, 12, "standard", reason)
	if !strings.Contains(svg, "provider &lt;temporarily unavailable&gt; &amp; retrying") {
		t.Fatalf("fallback SVG omitted or failed to escape reason: %s", svg)
	}
	if strings.Contains(svg, "<temporarily unavailable>") {
		t.Fatalf("fallback SVG kept raw markup in reason: %s", svg)
	}
	longReason := strings.Repeat("界", 81)
	svg = mapFallbackSVG(41.8781, -87.6298, 12, "standard", longReason)
	if !utf8.ValidString(svg) {
		t.Fatal("fallback SVG is not valid UTF-8 after reason truncation")
	}
	if got := strings.Count(svg, "界"); got != 80 {
		t.Fatalf("fallback reason rune truncation count=%d, want 80", got)
	}
}
