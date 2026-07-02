package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	releasepkg "github.com/DashDashGoApp/Dash-Go/app/internal/release"
)

func cliTestRelease(t *testing.T, version string) releasepkg.GitHubRelease {
	t.Helper()
	parsed, err := releasepkg.ParseVersion(version)
	if err != nil {
		t.Fatal(err)
	}
	names := releasepkg.AssetNames(parsed)
	const digest = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	assets := make([]releasepkg.Asset, 0, len(names))
	for _, name := range []string{names["release"], names["source"], names["sbom"], names["checksums"]} {
		assets = append(assets, releasepkg.Asset{Name: name, State: "uploaded", BrowserDownloadURL: "https://example.invalid/" + name, Digest: digest, Size: 1})
	}
	return releasepkg.GitHubRelease{TagName: parsed.Tag(), Prerelease: parsed.IsBeta, Immutable: true, HTMLURL: "https://example.invalid/releases/" + parsed.Tag(), Assets: assets}
}

func TestResolveGitHubReleaseCLIUsesExplicitVersionWhenInstalledVersionIsDamaged(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/repos/DashDashGoApp/Dash-Go/releases" {
			http.NotFound(w, request)
			return
		}
		_ = json.NewEncoder(w).Encode([]releasepkg.GitHubRelease{cliTestRelease(t, "1.5.6-beta.1")})
	}))
	defer server.Close()

	previous := newGitHubReleaseClient
	newGitHubReleaseClient = func() releasepkg.Client {
		return releasepkg.Client{Repository: releasepkg.CanonicalGitHubRepository, APIBase: server.URL, HTTPClient: server.Client()}
	}
	t.Cleanup(func() { newGitHubReleaseClient = previous })

	a := &app{releaseVersion: "damaged-version", cacheDir: t.TempDir()}
	cacheFile := filepath.Join(t.TempDir(), "github-release-cache.json")
	if code := a.runResolveGitHubReleaseCLI([]string{"--track", "beta", "--version", "1.5.6-beta.1", "--cache-file", cacheFile}); code != 0 {
		t.Fatalf("explicit repair resolution returned %d with damaged installed VERSION", code)
	}
}
