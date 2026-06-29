package release

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testDigest = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func makeRelease(version string, immutable bool) GitHubRelease {
	parsed, err := ParseVersion(version)
	if err != nil {
		panic(err)
	}
	names := AssetNames(parsed)
	assets := make([]Asset, 0, len(names))
	for _, name := range []string{names["release"], names["source"], names["sbom"], names["checksums"]} {
		assets = append(assets, Asset{Name: name, State: "uploaded", BrowserDownloadURL: "https://example.invalid/download/" + name, Digest: testDigest, Size: 1})
	}
	return GitHubRelease{TagName: parsed.Tag(), Prerelease: parsed.IsBeta, Immutable: immutable, HTMLURL: "https://example.invalid/releases/" + parsed.Tag(), Assets: assets}
}

func testClient(t *testing.T, releases []GitHubRelease) Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/repos/DashDashGoApp/Dash-Go/releases"; got != want {
			http.NotFound(w, r)
			return
		}
		if got := r.URL.Query().Get("per_page"); got != "100" {
			http.Error(w, "missing per_page", http.StatusBadRequest)
			return
		}
		if got := r.Header.Get("Accept"); got != "application/vnd.github+json" {
			http.Error(w, "missing accept", http.StatusBadRequest)
			return
		}
		if got := r.Header.Get("X-GitHub-Api-Version"); got != GitHubAPIVersion {
			http.Error(w, "missing api version", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(releases)
	}))
	t.Cleanup(server.Close)
	return Client{Repository: CanonicalGitHubRepository, APIBase: server.URL, HTTPClient: server.Client()}
}

func TestParseVersionStrict(t *testing.T) {
	good := []string{"1.5.0", "1.5.0-beta.0", "1.5.0-beta.33", "0.0.0", "10.20.30-beta.1"}
	for _, raw := range good {
		if _, err := ParseVersion(raw); err != nil {
			t.Fatalf("ParseVersion(%q): %v", raw, err)
		}
	}
	bad := []string{"v1.5.0", "1.5", "1.5.0-beta.00", "1.5.0-rc.1", "1.05.0", "1.5.0+build"}
	for _, raw := range bad {
		if _, err := ParseVersion(raw); err == nil {
			t.Fatalf("ParseVersion(%q) unexpectedly succeeded", raw)
		}
	}
}

func TestStableSelectionIgnoresPrereleases(t *testing.T) {
	client := testClient(t, []GitHubRelease{makeRelease("1.5.0-beta.33", true), makeRelease("1.4.9", true), makeRelease("1.5.0", true)})
	got, err := client.Resolve(context.Background(), TrackStable)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != "1.5.0" || got.Track != TrackStable {
		t.Fatalf("resolved=%#v", got)
	}
}

func TestBetaSelectionCanAdvanceToStable(t *testing.T) {
	client := testClient(t, []GitHubRelease{makeRelease("1.5.0-beta.33", true), makeRelease("1.5.0", true), makeRelease("1.5.1-beta.0", true)})
	got, err := client.Resolve(context.Background(), TrackBeta)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != "1.5.1-beta.0" || got.Track != TrackBeta {
		t.Fatalf("resolved=%#v", got)
	}
}

func TestResolveRejectsNewestNonImmutableRelease(t *testing.T) {
	client := testClient(t, []GitHubRelease{makeRelease("1.5.0", true), makeRelease("1.5.1", false)})
	_, err := client.Resolve(context.Background(), TrackStable)
	if err == nil || !strings.Contains(err.Error(), "not immutable") {
		t.Fatalf("error=%v, want non-immutable rejection", err)
	}
}

func TestResolveRejectsMissingRequiredAsset(t *testing.T) {
	broken := makeRelease("1.5.0", true)
	broken.Assets = broken.Assets[:3]
	client := testClient(t, []GitHubRelease{broken})
	_, err := client.Resolve(context.Background(), TrackStable)
	if err == nil || !strings.Contains(err.Error(), "missing required asset") {
		t.Fatalf("error=%v, want missing-asset rejection", err)
	}
}

func TestResolveRejectsInvalidDigest(t *testing.T) {
	broken := makeRelease("1.5.0", true)
	broken.Assets[0].Digest = "sha256:not-a-digest"
	client := testClient(t, []GitHubRelease{broken})
	_, err := client.Resolve(context.Background(), TrackStable)
	if err == nil || !strings.Contains(err.Error(), "SHA-256 digest") {
		t.Fatalf("error=%v, want digest rejection", err)
	}
}

func TestResolveRejectsDuplicateVersions(t *testing.T) {
	client := testClient(t, []GitHubRelease{makeRelease("1.5.0", true), makeRelease("1.5.0", true)})
	_, err := client.Resolve(context.Background(), TrackStable)
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("error=%v, want duplicate rejection", err)
	}
}

func TestResolveVersionFindsExactPublishedRelease(t *testing.T) {
	client := testClient(t, []GitHubRelease{makeRelease("1.5.0-beta.38", true), makeRelease("1.5.0-beta.33", true), makeRelease("1.5.0", true)})
	wanted, err := ParseVersion("1.5.0-beta.33")
	if err != nil {
		t.Fatal(err)
	}
	got, err := client.ResolveVersion(context.Background(), TrackBeta, wanted)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != wanted.String() {
		t.Fatalf("version=%q want %q", got.Version, wanted.String())
	}
}

func TestResolveVersionRejectsStableTrackBetaRequest(t *testing.T) {
	client := testClient(t, []GitHubRelease{makeRelease("1.5.0-beta.38", true)})
	wanted, err := ParseVersion("1.5.0-beta.38")
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.ResolveVersion(context.Background(), TrackStable, wanted)
	if err == nil || !strings.Contains(err.Error(), "stable track") {
		t.Fatalf("error=%v, want stable-track beta rejection", err)
	}
}

func TestResolveVersionDoesNotFallBackWhenExactVersionIsMissing(t *testing.T) {
	client := testClient(t, []GitHubRelease{makeRelease("1.5.0", true), makeRelease("1.5.1", true)})
	wanted, err := ParseVersion("1.5.2")
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.ResolveVersion(context.Background(), TrackStable, wanted)
	if err == nil || !strings.Contains(err.Error(), "is not published") {
		t.Fatalf("error=%v, want exact-version absence rejection", err)
	}
}

func TestResolveCachedUsesETagAndNotModifiedResponse(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 2 {
			if got := r.Header.Get("If-None-Match"); got != `"dash-go-etag-1"` {
				http.Error(w, "missing If-None-Match", http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"dash-go-etag-1"`)
		_ = json.NewEncoder(w).Encode([]GitHubRelease{makeRelease("1.5.0-beta.38", true)})
	}))
	defer server.Close()
	cachePath := filepath.Join(t.TempDir(), "github-release-cache.json")
	client := Client{Repository: CanonicalGitHubRepository, APIBase: server.URL, HTTPClient: server.Client()}
	first, firstInfo, err := client.ResolveCached(context.Background(), TrackBeta, cachePath)
	if err != nil || first.Version != "1.5.0-beta.38" || firstInfo.Source != "network" {
		t.Fatalf("first resolved=%#v info=%#v err=%v", first, firstInfo, err)
	}
	second, secondInfo, err := client.ResolveCached(context.Background(), TrackBeta, cachePath)
	if err != nil || second.Version != first.Version || secondInfo.Source != "not-modified" {
		t.Fatalf("second resolved=%#v info=%#v err=%v", second, secondInfo, err)
	}
	if calls != 2 {
		t.Fatalf("calls=%d, want 2", calls)
	}
	info, err := os.Stat(cachePath)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("cache permissions info=%v err=%v", info, err)
	}
}

func TestResolveCachedUsesFreshCacheOnlyDuringRateLimit(t *testing.T) {
	mode := "network"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if mode == "limited" {
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("ETag", `"dash-go-etag-2"`)
		_ = json.NewEncoder(w).Encode([]GitHubRelease{makeRelease("1.5.0", true)})
	}))
	defer server.Close()
	cachePath := filepath.Join(t.TempDir(), "github-release-cache.json")
	client := Client{Repository: CanonicalGitHubRepository, APIBase: server.URL, HTTPClient: server.Client()}
	if _, info, err := client.ResolveCached(context.Background(), TrackStable, cachePath); err != nil || info.Source != "network" {
		t.Fatalf("cache seed info=%#v err=%v", info, err)
	}
	mode = "limited"
	got, info, err := client.ResolveCached(context.Background(), TrackStable, cachePath)
	if err != nil || got.Version != "1.5.0" || info.Source != "rate-limit-cache" || info.RetryAfterSeconds != 60 {
		t.Fatalf("resolved=%#v info=%#v err=%v", got, info, err)
	}
}

func TestResolveCachedRejectsStaleRateLimitCache(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "github-release-cache.json")
	cached := releaseCache{Schema: releaseCacheSchema, ETag: `"old"`, FetchedAt: time.Now().Add(-maxReleaseCacheAge - time.Minute).Unix(), Releases: []GitHubRelease{makeRelease("1.5.0", true)}}
	writeReleaseCache(cachePath, cached)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	client := Client{Repository: CanonicalGitHubRepository, APIBase: server.URL, HTTPClient: server.Client()}
	_, _, err := client.ResolveCached(context.Background(), TrackStable, cachePath)
	var limited *RateLimitError
	if !errors.As(err, &limited) {
		t.Fatalf("error=%v, want RateLimitError", err)
	}
}
