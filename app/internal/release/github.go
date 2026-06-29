// Package release resolves Dash-Go releases from the canonical GitHub repository.
//
// It is intentionally independent from installer staging and replacement logic.
// The installer uses its small JSON result in a later migration phase; this
// package owns version parsing, release selection, public-asset contracts, and
// the bounded ETag cache used by manual update and Doctor checks.
package release

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	// CanonicalGitHubRepository is intentionally compiled into Dash-Go. Devices
	// never accept arbitrary update hosts from environment or UI settings.
	CanonicalGitHubRepository = "DashDashGoApp/Dash-Go"
	DefaultGitHubAPIBase      = "https://api.github.com"
	GitHubAPIVersion          = "2026-03-10"
	releaseUserAgent          = "Dash-Go release resolver"

	releaseCacheSchema     = 1
	maxReleaseResponseSize = 2 << 20
	maxReleaseCacheAge     = 6 * time.Hour
)

type Track string

const (
	TrackStable Track = "stable"
	TrackBeta   Track = "beta"
)

var versionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-beta\.(0|[1-9][0-9]*))?$`)
var sha256DigestPattern = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

// Version is the narrow Dash-Go release version grammar. Dash-Go deliberately
// does not accept generic SemVer labels: stable and beta are the only public
// tracks supported by the device updater.
type Version struct {
	Major  int
	Minor  int
	Patch  int
	Beta   int
	IsBeta bool
}

func ParseVersion(raw string) (Version, error) {
	raw = strings.TrimSpace(raw)
	m := versionPattern.FindStringSubmatch(raw)
	if len(m) == 0 {
		return Version{}, fmt.Errorf("invalid Dash-Go version %q", raw)
	}
	values := make([]int, 4)
	for i := 1; i < len(m); i++ {
		if m[i] == "" {
			continue
		}
		n, err := strconv.Atoi(m[i])
		if err != nil {
			return Version{}, fmt.Errorf("invalid Dash-Go version %q", raw)
		}
		values[i-1] = n
	}
	return Version{Major: values[0], Minor: values[1], Patch: values[2], Beta: values[3], IsBeta: m[4] != ""}, nil
}

func (v Version) String() string {
	base := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.IsBeta {
		return fmt.Sprintf("%s-beta.%d", base, v.Beta)
	}
	return base
}

func (v Version) Tag() string { return "v" + v.String() }

// Compare returns -1, 0, or 1. A beta is lower than its matching stable
// release, so a beta-track device may advance to that stable release.
func (v Version) Compare(other Version) int {
	for _, pair := range [][2]int{{v.Major, other.Major}, {v.Minor, other.Minor}, {v.Patch, other.Patch}} {
		if pair[0] < pair[1] {
			return -1
		}
		if pair[0] > pair[1] {
			return 1
		}
	}
	switch {
	case v.IsBeta && !other.IsBeta:
		return -1
	case !v.IsBeta && other.IsBeta:
		return 1
	case !v.IsBeta && !other.IsBeta:
		return 0
	case v.Beta < other.Beta:
		return -1
	case v.Beta > other.Beta:
		return 1
	default:
		return 0
	}
}

func NormalizeTrack(raw string, current Version) Track {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(TrackStable):
		return TrackStable
	case string(TrackBeta):
		return TrackBeta
	}
	if current.IsBeta {
		return TrackBeta
	}
	return TrackStable
}

func AssetNames(version Version) map[string]string {
	prefix := "Dash-Go_" + version.String()
	return map[string]string{
		"release":   prefix + "_release.tar.gz",
		"source":    prefix + "_source.tar.gz",
		"sbom":      prefix + "_sbom.spdx.json",
		"checksums": "SHA256SUMS",
	}
}

type Asset struct {
	Name               string `json:"name"`
	State              string `json:"state"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Digest             string `json:"digest"`
	Size               int64  `json:"size"`
}

// GitHubRelease contains only the public metadata required to validate an
// immutable Dash-Go release. It is stored in a small private cache so a 304
// response can reuse exactly the metadata that GitHub previously supplied.
type GitHubRelease struct {
	TagName    string  `json:"tag_name"`
	Draft      bool    `json:"draft"`
	Prerelease bool    `json:"prerelease"`
	Immutable  bool    `json:"immutable"`
	HTMLURL    string  `json:"html_url"`
	Assets     []Asset `json:"assets"`
}

type Resolved struct {
	Repository string           `json:"repository"`
	Version    string           `json:"version"`
	Tag        string           `json:"tag"`
	Track      Track            `json:"track"`
	ReleaseURL string           `json:"releaseUrl"`
	Immutable  bool             `json:"immutable"`
	Assets     map[string]Asset `json:"assets"`
}

// FetchInfo records how GitHub metadata was obtained. It deliberately carries
// no secrets, headers, or arbitrary endpoint information.
type FetchInfo struct {
	Source            string `json:"source"`
	FetchedAt         int64  `json:"fetchedAt"`
	CacheAgeSeconds   int64  `json:"cacheAgeSeconds,omitempty"`
	RetryAfterSeconds int64  `json:"retryAfterSeconds,omitempty"`
}

// RateLimitError is returned only when GitHub has rate-limited a discovery
// request and a recent cached metadata set is unavailable. Callers can present
// a calm retry message without treating it as a broken local installation.
type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	if e == nil || e.RetryAfter <= 0 {
		return "GitHub release discovery is temporarily rate limited"
	}
	return fmt.Sprintf("GitHub release discovery is temporarily rate limited; retry after %s", e.RetryAfter.Round(time.Second))
}

type Client struct {
	Repository string
	APIBase    string
	HTTPClient *http.Client
}

func NewClient() Client {
	return Client{
		Repository: CanonicalGitHubRepository,
		APIBase:    DefaultGitHubAPIBase,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c Client) normalized() (Client, error) {
	if strings.TrimSpace(c.Repository) == "" {
		c.Repository = CanonicalGitHubRepository
	}
	if c.Repository != CanonicalGitHubRepository {
		return Client{}, errors.New("GitHub repository is not canonical")
	}
	if strings.TrimSpace(c.APIBase) == "" {
		c.APIBase = DefaultGitHubAPIBase
	}
	base, err := url.Parse(c.APIBase)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return Client{}, errors.New("GitHub API base URL is invalid")
	}
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}
	return c, nil
}

func (c Client) listURL() (string, error) {
	base, err := url.Parse(strings.TrimRight(c.APIBase, "/") + "/")
	if err != nil {
		return "", err
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/repos/DashDashGoApp/Dash-Go/releases"
	query := base.Query()
	query.Set("per_page", "100")
	query.Set("page", "1")
	base.RawQuery = query.Encode()
	return base.String(), nil
}

func parseRetryAfter(response *http.Response) time.Duration {
	if response == nil {
		return 0
	}
	if raw := strings.TrimSpace(response.Header.Get("Retry-After")); raw != "" {
		if seconds, err := strconv.ParseInt(raw, 10, 64); err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
		if when, err := http.ParseTime(raw); err == nil {
			if wait := time.Until(when); wait > 0 {
				return wait
			}
		}
	}
	if reset, err := strconv.ParseInt(strings.TrimSpace(response.Header.Get("X-RateLimit-Reset")), 10, 64); err == nil && reset > time.Now().Unix() {
		return time.Until(time.Unix(reset, 0))
	}
	return 0
}

func (c Client) fetchList(ctx context.Context, etag string) ([]GitHubRelease, string, bool, error) {
	normalized, err := c.normalized()
	if err != nil {
		return nil, "", false, err
	}
	u, err := normalized.listURL()
	if err != nil {
		return nil, "", false, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, "", false, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", GitHubAPIVersion)
	req.Header.Set("User-Agent", releaseUserAgent)
	if strings.TrimSpace(etag) != "" {
		req.Header.Set("If-None-Match", etag)
	}
	response, err := normalized.HTTPClient.Do(req)
	if err != nil {
		return nil, "", false, fmt.Errorf("GitHub release discovery failed: %w", err)
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case http.StatusNotModified:
		return nil, strings.TrimSpace(etag), true, nil
	case http.StatusOK:
		body, err := io.ReadAll(io.LimitReader(response.Body, maxReleaseResponseSize))
		if err != nil {
			return nil, "", false, err
		}
		if len(body) >= maxReleaseResponseSize {
			return nil, "", false, errors.New("GitHub release response is too large")
		}
		var releases []GitHubRelease
		if err := json.Unmarshal(body, &releases); err != nil {
			return nil, "", false, fmt.Errorf("GitHub release response is invalid JSON: %w", err)
		}
		return releases, strings.TrimSpace(response.Header.Get("ETag")), false, nil
	case http.StatusTooManyRequests:
		return nil, "", false, &RateLimitError{RetryAfter: parseRetryAfter(response)}
	case http.StatusForbidden:
		if strings.TrimSpace(response.Header.Get("X-RateLimit-Remaining")) == "0" || response.Header.Get("Retry-After") != "" {
			return nil, "", false, &RateLimitError{RetryAfter: parseRetryAfter(response)}
		}
		return nil, "", false, fmt.Errorf("GitHub release discovery returned HTTP %d", response.StatusCode)
	default:
		return nil, "", false, fmt.Errorf("GitHub release discovery returned HTTP %d", response.StatusCode)
	}
}

// List fetches the current release list without a persisted cache. It remains a
// small testing/programmatic helper; installed Dash-Go uses ListCached.
func (c Client) List(ctx context.Context) ([]GitHubRelease, error) {
	releases, _, _, err := c.fetchList(ctx, "")
	return releases, err
}

type releaseCache struct {
	Schema    int             `json:"schema"`
	ETag      string          `json:"etag"`
	FetchedAt int64           `json:"fetchedAt"`
	Releases  []GitHubRelease `json:"releases"`
}

func readReleaseCache(path string) (releaseCache, bool) {
	if strings.TrimSpace(path) == "" {
		return releaseCache{}, false
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxReleaseResponseSize {
		return releaseCache{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 || len(data) > maxReleaseResponseSize {
		return releaseCache{}, false
	}
	var cached releaseCache
	if json.Unmarshal(data, &cached) != nil || cached.Schema != releaseCacheSchema || cached.FetchedAt <= 0 || len(cached.Releases) == 0 {
		return releaseCache{}, false
	}
	return cached, true
}

func writeReleaseCache(path string, cached releaseCache) {
	if strings.TrimSpace(path) == "" {
		return
	}
	data, err := json.Marshal(cached)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".github-release-cache.*")
	if err != nil {
		return
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return
	}
	if _, err := tmp.Write(data); err != nil || tmp.Close() != nil {
		return
	}
	_ = os.Chmod(name, 0600)
	_ = os.Rename(name, path)
	_ = os.Chmod(path, 0600)
}

// ListCached conditionally validates cached public release metadata with ETag.
// A cache is used only for a 304 response, or for a rate-limit response when
// it is at most six hours old. Network/JSON/contract errors never become a
// package-install fallback.
func (c Client) ListCached(ctx context.Context, cachePath string) ([]GitHubRelease, FetchInfo, error) {
	cached, hasCache := readReleaseCache(cachePath)
	etag := ""
	if hasCache {
		etag = cached.ETag
	}
	releases, responseETag, notModified, err := c.fetchList(ctx, etag)
	now := time.Now()
	if err == nil {
		if notModified {
			if !hasCache {
				return nil, FetchInfo{}, errors.New("GitHub returned not-modified without cached release metadata")
			}
			cached.FetchedAt = now.Unix()
			writeReleaseCache(cachePath, cached)
			return cached.Releases, FetchInfo{Source: "not-modified", FetchedAt: cached.FetchedAt}, nil
		}
		newCache := releaseCache{Schema: releaseCacheSchema, ETag: responseETag, FetchedAt: now.Unix(), Releases: releases}
		writeReleaseCache(cachePath, newCache)
		return releases, FetchInfo{Source: "network", FetchedAt: newCache.FetchedAt}, nil
	}
	var limited *RateLimitError
	if errors.As(err, &limited) && hasCache {
		age := now.Sub(time.Unix(cached.FetchedAt, 0))
		if age >= 0 && age <= maxReleaseCacheAge {
			return cached.Releases, FetchInfo{Source: "rate-limit-cache", FetchedAt: cached.FetchedAt, CacheAgeSeconds: int64(age.Seconds()), RetryAfterSeconds: int64(limited.RetryAfter.Seconds())}, nil
		}
	}
	return nil, FetchInfo{}, err
}

func parseTag(tag string) (Version, error) {
	if !strings.HasPrefix(tag, "v") {
		return Version{}, fmt.Errorf("release tag %q is not a Dash-Go tag", tag)
	}
	return ParseVersion(strings.TrimPrefix(tag, "v"))
}

func validateAsset(asset Asset, want string) error {
	if asset.Name != want {
		return fmt.Errorf("unexpected asset name %q", asset.Name)
	}
	if asset.State != "uploaded" {
		return fmt.Errorf("asset %s is not uploaded", want)
	}
	if asset.Size <= 0 {
		return fmt.Errorf("asset %s has invalid size", want)
	}
	if !sha256DigestPattern.MatchString(strings.ToLower(asset.Digest)) {
		return fmt.Errorf("asset %s does not provide a SHA-256 digest", want)
	}
	if strings.TrimSpace(asset.BrowserDownloadURL) == "" {
		return fmt.Errorf("asset %s has no download URL", want)
	}
	return nil
}

func validateRelease(raw GitHubRelease, version Version, track Track) (Resolved, error) {
	if raw.Draft {
		return Resolved{}, fmt.Errorf("release %s is still a draft", raw.TagName)
	}
	if !raw.Immutable {
		return Resolved{}, fmt.Errorf("release %s is not immutable", raw.TagName)
	}
	if raw.Prerelease != version.IsBeta {
		return Resolved{}, fmt.Errorf("release %s prerelease flag does not match its version", raw.TagName)
	}
	if track == TrackStable && version.IsBeta {
		return Resolved{}, fmt.Errorf("stable track cannot select beta release %s", raw.TagName)
	}
	names := AssetNames(version)
	byName := make(map[string]Asset, len(raw.Assets))
	for _, asset := range raw.Assets {
		if _, exists := byName[asset.Name]; exists {
			return Resolved{}, fmt.Errorf("release %s has duplicate asset %s", raw.TagName, asset.Name)
		}
		byName[asset.Name] = asset
	}
	out := make(map[string]Asset, len(names))
	for role, name := range names {
		asset, ok := byName[name]
		if !ok {
			return Resolved{}, fmt.Errorf("release %s is missing required asset %s", raw.TagName, name)
		}
		if err := validateAsset(asset, name); err != nil {
			return Resolved{}, err
		}
		out[role] = asset
	}
	return Resolved{
		Repository: CanonicalGitHubRepository,
		Version:    version.String(),
		Tag:        raw.TagName,
		Track:      track,
		ReleaseURL: raw.HTMLURL,
		Immutable:  raw.Immutable,
		Assets:     out,
	}, nil
}

type candidate struct {
	version Version
	raw     GitHubRelease
}

func candidatesFromReleases(releases []GitHubRelease, track Track) ([]candidate, error) {
	switch track {
	case TrackStable, TrackBeta:
	default:
		return nil, fmt.Errorf("unsupported release track %q", track)
	}
	candidates := make([]candidate, 0, len(releases))
	seen := map[string]bool{}
	for _, raw := range releases {
		version, err := parseTag(strings.TrimSpace(raw.TagName))
		if err != nil {
			// The canonical repository may contain administrative releases. They are
			// never device-update candidates.
			continue
		}
		if track == TrackStable && version.IsBeta {
			continue
		}
		key := version.String()
		if seen[key] {
			return nil, fmt.Errorf("duplicate Dash-Go release version %s", key)
		}
		seen[key] = true
		candidates = append(candidates, candidate{version: version, raw: raw})
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no eligible %s GitHub release is published", track)
	}
	return candidates, nil
}

func (c Client) resolveCandidates(ctx context.Context, track Track) ([]candidate, error) {
	releases, err := c.List(ctx)
	if err != nil {
		return nil, err
	}
	return candidatesFromReleases(releases, track)
}

func (c Client) resolveCandidatesCached(ctx context.Context, track Track, cachePath string) ([]candidate, FetchInfo, error) {
	releases, info, err := c.ListCached(ctx, cachePath)
	if err != nil {
		return nil, FetchInfo{}, err
	}
	candidates, err := candidatesFromReleases(releases, track)
	if err != nil {
		return nil, FetchInfo{}, err
	}
	return candidates, info, nil
}

// Resolve selects the newest valid release for a Dash-Go device track. It
// deliberately validates the newest candidate instead of silently falling back
// to an older release if the newest publication is incomplete or malformed.
func (c Client) Resolve(ctx context.Context, track Track) (Resolved, error) {
	candidates, err := c.resolveCandidates(ctx, track)
	if err != nil {
		return Resolved{}, err
	}
	slices.SortFunc(candidates, func(a, b candidate) int {
		return -a.version.Compare(b.version)
	})
	return validateRelease(candidates[0].raw, candidates[0].version, track)
}

// ResolveCached has the same strict release contract as Resolve but uses a
// bounded private ETag cache for conditional manual checks and rate-limit calm.
func (c Client) ResolveCached(ctx context.Context, track Track, cachePath string) (Resolved, FetchInfo, error) {
	candidates, info, err := c.resolveCandidatesCached(ctx, track, cachePath)
	if err != nil {
		return Resolved{}, FetchInfo{}, err
	}
	slices.SortFunc(candidates, func(a, b candidate) int {
		return -a.version.Compare(b.version)
	})
	resolved, err := validateRelease(candidates[0].raw, candidates[0].version, track)
	return resolved, info, err
}

// ResolveVersion validates one exact published Dash-Go version. Repair uses
// this rather than latest resolution so a normal repair restores the installed
// version unless the operator explicitly asks to update.
func (c Client) ResolveVersion(ctx context.Context, track Track, wanted Version) (Resolved, error) {
	if track == TrackStable && wanted.IsBeta {
		return Resolved{}, fmt.Errorf("stable track cannot select beta version %s", wanted.String())
	}
	candidates, err := c.resolveCandidates(ctx, track)
	if err != nil {
		return Resolved{}, err
	}
	for _, candidate := range candidates {
		if candidate.version.Compare(wanted) == 0 {
			return validateRelease(candidate.raw, candidate.version, track)
		}
	}
	return Resolved{}, fmt.Errorf("Dash-Go release %s is not published on the %s track", wanted.String(), track)
}

// ResolveVersionCached is the exact-version repair equivalent of ResolveCached.
func (c Client) ResolveVersionCached(ctx context.Context, track Track, wanted Version, cachePath string) (Resolved, FetchInfo, error) {
	if track == TrackStable && wanted.IsBeta {
		return Resolved{}, FetchInfo{}, fmt.Errorf("stable track cannot select beta version %s", wanted.String())
	}
	candidates, info, err := c.resolveCandidatesCached(ctx, track, cachePath)
	if err != nil {
		return Resolved{}, FetchInfo{}, err
	}
	for _, candidate := range candidates {
		if candidate.version.Compare(wanted) == 0 {
			resolved, err := validateRelease(candidate.raw, candidate.version, track)
			return resolved, info, err
		}
	}
	return Resolved{}, FetchInfo{}, fmt.Errorf("Dash-Go release %s is not published on the %s track", wanted.String(), track)
}
