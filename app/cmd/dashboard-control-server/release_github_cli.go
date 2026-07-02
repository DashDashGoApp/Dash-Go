package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	releasepkg "github.com/DashDashGoApp/Dash-Go/app/internal/release"
)

// newGitHubReleaseClient is a narrow test seam. Production always resolves
// only the canonical GitHub Release repository through releasepkg.NewClient.
var newGitHubReleaseClient = releasepkg.NewClient

// runResolveGitHubReleaseCLI is the narrow installer-facing seam for the
// GitHub Release migration. It resolves only canonical public metadata;
// staging, downloading, checksums, and replacement remain installer work.
func (a *app) runResolveGitHubReleaseCLI(args []string) int {
	fs := flag.NewFlagSet("resolve-github-release", flag.ContinueOnError)
	trackRaw := fs.String("track", "", "stable or beta")
	versionRaw := fs.String("version", "", "exact Dash-Go version for repair")
	cacheFile := fs.String("cache-file", filepath.Join(a.cacheDir, "github-release-cache.json"), "private bounded GitHub release metadata cache")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 {
		fmt.Fprintln(fs.Output(), "usage: --resolve-github-release [--track stable|beta] [--version X.Y.Z[-beta.N]] [--cache-file FILE]")
		return 64
	}
	requestedTrack := strings.ToLower(strings.TrimSpace(*trackRaw))
	if requestedTrack != "" && requestedTrack != string(releasepkg.TrackStable) && requestedTrack != string(releasepkg.TrackBeta) {
		fmt.Fprintln(fs.Output(), "--track must be stable or beta")
		return 64
	}
	current, currentErr := releasepkg.ParseVersion(strings.TrimSpace(a.releaseVersion))
	if currentErr != nil {
		// A damaged/missing VERSION must not turn into an arbitrary channel.
		current, _ = releasepkg.ParseVersion("0.0.0")
	}
	track := releasepkg.NormalizeTrack(*trackRaw, current)
	client := newGitHubReleaseClient()
	var resolved releasepkg.Resolved
	var resolveErr error
	if requested := strings.TrimSpace(*versionRaw); requested != "" {
		wanted, parseErr := releasepkg.ParseVersion(requested)
		if parseErr != nil {
			fmt.Fprintln(os.Stderr, parseErr)
			return 64
		}
		resolved, _, resolveErr = client.ResolveVersionCached(context.Background(), track, wanted, *cacheFile)
	} else {
		resolved, _, resolveErr = client.ResolveCached(context.Background(), track, *cacheFile)
	}
	if resolveErr != nil {
		fmt.Fprintln(os.Stderr, resolveErr)
		return 1
	}
	payload, err := json.Marshal(resolved)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(string(payload))
	return 0
}
