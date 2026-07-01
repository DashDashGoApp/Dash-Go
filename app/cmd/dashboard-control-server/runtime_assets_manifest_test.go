package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// This is the semantic asset-order oracle. It freezes the reviewed source
// sequence without deriving runtime order from filenames. Update these lists
// only as part of a deliberately reviewed asset-order change.
func TestRuntimeAssetManifestsPreserveSemanticSourceOrder(t *testing.T) {
	root := runtimeAssetProjectRoot(t)

	gotJS, err := runtimeAssetManifestFiles(root, runtimeAssetJSManifestRel, "ui/js", ".js", "app", "control")
	if err != nil {
		t.Fatalf("read JavaScript manifest: %v", err)
	}
	wantJS := map[string][]string{
		"app": {
			"ui/js/app-dialog.js",
			"ui/js/tap.js",
			"ui/js/shared-osk.js",
			"ui/js/config-defaults.js",
			"ui/js/config-default-messages.js",
			"ui/js/config-themes-core.js",
			"ui/js/config-themes-foundation-seasonal.js",
			"ui/js/config-themes-color-moods.js",
			"ui/js/config-themes-occasions-accessibility.js",
			"ui/js/config-theme-meta.js",
			"ui/js/config-runtime.js",
			"ui/js/ics-parser.js",
			"ui/js/ics-recurrence.js",
			"ui/js/data-core.js",
			"ui/js/data-sources.js",
			"ui/js/event-cache.js",
			"ui/js/data-refresh.js",
			"ui/js/calendar-span-helpers.js",
			"ui/js/calendar-grid.js",
			"ui/js/calendar-span-bars.js",
			"ui/js/calendar-layout-finish.js",
			"ui/js/calendar-cull-overscan.js",
			"ui/js/calendar-agenda.js",
			"ui/js/calendar-list-overscan.js",
			"ui/js/calendar-decor-art-seasons.js",
			"ui/js/calendar-decor-art-seasonal.js",
			"ui/js/calendar-decor-art-observances.js",
			"ui/js/calendar-seasonal-decor.js",
			"ui/js/popup-overlays.js",
			"ui/js/map-interactive.js",
			"ui/js/map-keyboard-controls.js",
			"ui/js/event-maps.js",
			"ui/js/managed-schedules-popup.js",
			"ui/js/event-popup.js",
			"ui/js/day-popup.js",
			"ui/js/app-calendar-actions.js",
			"ui/js/weather-icons.js",
			"ui/js/radar-sources.js",
			"ui/js/radar-overlay.js",
			"ui/js/radar-lite.js",
			"ui/js/weather-sources.js",
			"ui/js/weather-blend.js",
			"ui/js/weather.js",
			"ui/js/clock.js",
			"ui/js/messages-holidays.js",
			"ui/js/messages-core.js",
			"ui/js/messages-fit.js",
			"ui/js/messages-lite-fit.js",
			"ui/js/messages-layout.js",
			"ui/js/messages-schedule.js",
			"ui/js/messages-options.js",
			"ui/js/messages-rotation.js",
			"ui/js/display-health.js",
			"ui/js/health-warning-actions.js",
			"ui/js/dashboard-fit.js",
			"ui/js/control-lazy-loader.js",
			"ui/js/settings-runtime.js",
			"ui/js/settings-display-sleep.js",
			"ui/js/settings-alerts.js",
			"ui/js/settings-idle-scroll.js",
			"ui/js/settings-calendar-geometry.js",
			"ui/js/settings-scroll-lifecycle.js",
			"ui/js/control-core.js",
			"ui/js/control-api.js",
			"ui/js/boot.js",
			"ui/js/app-launcher.js",
			"ui/js/lists-dock.js",
			"ui/js/household-app-loader.js",
			"ui/js/family-board-footer.js",
		},
		"control": {
			"ui/js/control-ui.js",
			"ui/js/control-status-health.js",
			"ui/js/control-updates.js",
			"ui/js/control-backups.js",
			"ui/js/control-cache.js",
			"ui/js/control-calendars-logs.js",
			"ui/js/control-navigation.js",
			"ui/js/control-terminal-profile.js",
			"ui/js/control-profile-editor.js",
			"ui/js/control-system-actions.js",
			"ui/js/control-theme.js",
			"ui/js/control-calendars.js",
			"ui/js/control-household-schedules.js",
			"ui/js/control-display-weather.js",
			"ui/js/control-dashboard-typography.js",
			"ui/js/control-content-osk.js",
			"ui/js/control-message-schedules.js",
			"ui/js/control-special-feeds.js",
			"ui/js/control-location-lock.js",
			"ui/js/control-lifecycle.js",
			"ui/js/control-lite-memory.js",
			"ui/js/control-visual-style.js",
			"ui/js/control-todo.js",
			"ui/js/control-people.js",
			"ui/js/control-people-pin-keypad.js",
		},
	}
	for _, bundle := range []string{"app", "control"} {
		if got, want := runtimeAssetRelativePaths(root, gotJS[bundle]), wantJS[bundle]; !reflect.DeepEqual(got, want) {
			t.Fatalf("%s JavaScript manifest source order differs from the semantic order fixture\n got: %q\nwant: %q", bundle, got, want)
		}
	}

	gotCSS, err := runtimeAssetManifestFiles(root, runtimeAssetCSSManifestRel, "ui/css", ".css", "dashboard", "control")
	if err != nil {
		t.Fatalf("read CSS manifest: %v", err)
	}
	wantCSS := map[string][]string{
		"dashboard": {
			"ui/css/dashboard/base.css",
			"ui/css/dashboard/demo-mode.css",
			"ui/css/dashboard/responsive.css",
			"ui/css/dashboard/lite-dashboard.css",
			"ui/css/dashboard/calendar.css",
			"ui/css/dashboard/app-calendar-actions.css",
			"ui/css/dashboard/sidebar-weather-messages.css",
			"ui/css/dashboard/control-overlay.css",
			"ui/css/dashboard/weather-alerts.css",
			"ui/css/dashboard/popups-alerts-maps.css",
			"ui/css/dashboard/day-timeline-popup.css",
			"ui/css/dashboard/managed-schedules-popup.css",
			"ui/css/dashboard/event-popup-compact.css",
			"ui/css/dashboard/control-status-maintenance.css",
			"ui/css/dashboard/control-theme-actions-lite.css",
			"ui/css/dashboard/control-visual-style.css",
			"ui/css/dashboard/touch-radar.css",
			"ui/css/dashboard/app-launcher.css",
			"ui/css/dashboard/lists-dock.css",
			"ui/css/dashboard/family-board-footer.css",
		},
		"control": {
			"ui/css/control/tokens.css",
			"ui/css/control/lite-performance.css",
			"ui/css/control/console-shell-tabs.css",
			"ui/css/control/display-profile.css",
			"ui/css/control/display-location.css",
			"ui/css/control/dashboard-typography.css",
			"ui/css/control/consistency.css",
			"ui/css/control/messages-osk.css",
			"ui/css/control/message-forms.css",
			"ui/css/control/message-scroll-feed.css",
			"ui/css/control/theme-weather.css",
			"ui/css/control/panel-polish.css",
			"ui/css/control/todo.css",
			"ui/css/control/household-schedules.css",
			"ui/css/control/people.css",
			"ui/css/control/people-pin-keypad.css",
			"ui/css/control/information-architecture.css",
			"ui/css/control/layout.css",
		},
	}
	for _, bundle := range []string{"dashboard", "control"} {
		if got, want := runtimeAssetRelativePaths(root, gotCSS[bundle]), wantCSS[bundle]; !reflect.DeepEqual(got, want) {
			t.Fatalf("%s CSS manifest source order differs from the semantic order fixture\n got: %q\nwant: %q", bundle, got, want)
		}
	}
}

func TestRuntimeAssetSplitSourcesUseSemanticNames(t *testing.T) {
	root := runtimeAssetProjectRoot(t)
	areas := []struct {
		rel      string
		ext      string
		excluded map[string]bool
	}{
		{
			rel: "ui/js",
			ext: ".js",
			excluded: map[string]bool{
				"app.bundle.js":         true,
				"app.control.bundle.js": true,
			},
		},
		{rel: "ui/css", ext: ".css", excluded: map[string]bool{}},
	}
	for _, area := range areas {
		area := area
		t.Run(area.rel, func(t *testing.T) {
			err := filepath.WalkDir(filepath.Join(root, filepath.FromSlash(area.rel)), func(path string, entry os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.IsDir() || filepath.Ext(path) != area.ext || area.excluded[filepath.Base(path)] {
					return nil
				}
				base := filepath.Base(path)
				if base[0] >= '0' && base[0] <= '9' {
					rel, relErr := filepath.Rel(root, path)
					if relErr != nil {
						return relErr
					}
					return fmt.Errorf("split source retains a numeric ordering prefix: %s", filepath.ToSlash(rel))
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRuntimeAssetManifestRejectsUnsafeOrDuplicateSources(t *testing.T) {
	cases := []struct {
		name     string
		manifest string
		want     string
	}{
		{
			name:     "duplicate across bundles",
			manifest: `{"schema":1,"bundles":{"app":["one.js"],"control":["one.js"]}}`,
			want:     "duplicates bundle",
		},
		{
			name:     "source traversal",
			manifest: `{"schema":1,"bundles":{"app":["../outside.js"],"control":["two.js"]}}`,
			want:     "escapes its source directory",
		},
		{
			name:     "generated bundle cannot be source",
			manifest: `{"schema":1,"bundles":{"app":["app.bundle.js"],"control":["two.js"]}}`,
			want:     "generated JavaScript bundle",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeRuntimeAssetTestFile(t, filepath.Join(root, "ui", "js", "one.js"), "one\n")
			writeRuntimeAssetTestFile(t, filepath.Join(root, "ui", "js", "two.js"), "two\n")
			writeRuntimeAssetTestFile(t, filepath.Join(root, "ui", "js", "bundle.manifest.json"), tc.manifest)
			_, err := runtimeAssetManifestFiles(root, runtimeAssetJSManifestRel, "ui/js", ".js", "app", "control")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func runtimeAssetProjectRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate runtime asset test")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
}

func runtimeAssetRelativePaths(root string, files []string) []string {
	out := make([]string, 0, len(files))
	for _, path := range files {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			panic(err)
		}
		out = append(out, filepath.ToSlash(rel))
	}
	return out
}

func writeRuntimeAssetTestFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}
