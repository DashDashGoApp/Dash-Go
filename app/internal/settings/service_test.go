package settings

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func testService(t *testing.T) *Service {
	t.Helper()
	root := t.TempDir()
	return New(Config{
		SettingsFile:    filepath.Join(root, "config", "settings.json"),
		ConfigLocal:     filepath.Join(root, "config", "config.local.js"),
		CacheDir:        filepath.Join(root, "cache"),
		ThemeCatalog:    filepath.Join(root, "themes.list"),
		FontsDir:        filepath.Join(root, "fonts"),
		BundledFontsDir: filepath.Join(root, "ui", "fonts"),
		ValidateRadar:   func(map[string]any) error { return nil },
	})
}

func TestServiceMutateSerializesAndWritesLastGood(t *testing.T) {
	service := testService(t)
	if err := service.Write(map[string]any{}); err != nil {
		t.Fatal(err)
	}
	const writers = 10
	var wg sync.WaitGroup
	errs := make(chan error, writers)
	for i := range writers {
		wg.Go(func() {
			_, err := service.Update(func(values map[string]any) { values["writer"] = i })
			errs <- err
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(service.lastGoodFile()); err != nil {
		t.Fatalf("last-good missing: %v", err)
	}
	if got := service.Load()["writer"]; got == nil {
		t.Fatal("serialized updates did not persist a value")
	}
}

func TestProfilePayloadUsesInjectedWeatherPolicyOnly(t *testing.T) {
	service := testService(t)
	values, err := service.ApplyProfilePreset("lite")
	if err != nil {
		t.Fatal(err)
	}
	policy := map[string]any{"automatic": true, "minimumMinutes": 15}
	payload := service.ProfilePayloadForSettings(values, policy)
	if payload["current"] != "lite" || payload["base"] != "lite" {
		t.Fatalf("profile payload=%#v", payload)
	}
	if got := payload["weatherRefresh"]; !reflect.DeepEqual(got, policy) {
		t.Fatalf("weather policy was not preserved: %#v", got)
	}
}

func TestThemeWriteAndRuntimeFontPathRemainBounded(t *testing.T) {
	service := testService(t)
	if err := service.WriteTheme("paper"); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(service.ConfigLocal())
	if err != nil || !strings.Contains(string(body), `theme: "paper"`) {
		t.Fatalf("theme write=%q err=%v", body, err)
	}
	if _, ok := service.RuntimeFontPath("../font.ttf"); ok {
		t.Fatal("font traversal unexpectedly resolved")
	}
}

func TestFontDownloadRejectsBadDigestWithoutReplacingLiveFile(t *testing.T) {
	service := testService(t)
	if err := os.MkdirAll(service.FontsDir(), 0755); err != nil {
		t.Fatal(err)
	}
	live := filepath.Join(service.FontsDir(), "fixture.ttf")
	if err := os.WriteFile(live, []byte("known good"), 0644); err != nil {
		t.Fatal(err)
	}
	payload := make([]byte, 4096)
	payload[0], payload[1], payload[2], payload[3] = 0, 1, 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(payload) }))
	defer server.Close()
	spec := RuntimeFontSpec{Key: "fixture", Family: "Fixture", Assets: []RuntimeFontAsset{{File: "fixture.ttf", Family: "Fixture", Weight: "400", URL: server.URL, SHA256: strings.Repeat("0", 64)}}}
	if err := service.DownloadRuntimeFontWithClient(spec, server.Client()); err == nil {
		t.Fatal("bad digest unexpectedly succeeded")
	}
	got, err := os.ReadFile(live)
	if err != nil || string(got) != "known good" {
		t.Fatalf("failed download replaced live font: %q err=%v", got, err)
	}
}

func TestOpenRuntimeFontUsesPinnedAssetAndRejectsSymlinks(t *testing.T) {
	originalSpecs := runtimeFontSpecs
	payload := make([]byte, 4096)
	payload[0], payload[1], payload[2], payload[3] = 0, 1, 0, 0
	for i := 4; i < len(payload); i++ {
		payload[i] = byte(i % 251)
	}
	sum := sha256.Sum256(payload)
	asset := RuntimeFontAsset{File: "Fixture.ttf", Family: "Fixture", Weight: "400", SHA256: hex.EncodeToString(sum[:])}
	runtimeFontSpecs = map[string]RuntimeFontSpec{"fixture": {Key: "fixture", Family: "Fixture", Assets: []RuntimeFontAsset{asset}}}
	t.Cleanup(func() { runtimeFontSpecs = originalSpecs })

	service := testService(t)
	if err := os.MkdirAll(service.FontsDir(), 0755); err != nil {
		t.Fatal(err)
	}
	live := filepath.Join(service.FontsDir(), asset.File)
	if err := os.WriteFile(live, payload, 0644); err != nil {
		t.Fatal(err)
	}
	font, info, publicName, ok := service.OpenRuntimeFont(asset.File)
	if !ok || font == nil || info == nil || publicName != asset.File {
		t.Fatalf("pinned runtime font did not open: ok=%v name=%q info=%#v", ok, publicName, info)
	}
	_ = font.Close()
	if _, _, _, ok := service.OpenRuntimeFont("../Fixture.ttf"); ok {
		t.Fatal("traversal leaf unexpectedly opened")
	}
	if err := os.Remove(live); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(t.TempDir(), "outside.ttf"), live); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, _, _, ok := service.OpenRuntimeFont(asset.File); ok {
		t.Fatal("symlinked runtime font unexpectedly opened")
	}
}
