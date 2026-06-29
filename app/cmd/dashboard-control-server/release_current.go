package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	releasepkg "github.com/DashDashGoApp/Dash-Go/app/internal/release"
)

func (a *app) githubReleaseCachePath() string {
	return filepath.Join(a.cacheDir, "github-release-cache.json")
}

func normalizeReleaseTrack(raw, current string) string {
	currentVersion, err := releasepkg.ParseVersion(strings.TrimSpace(current))
	if err != nil {
		currentVersion, _ = releasepkg.ParseVersion("0.0.0")
	}
	return string(releasepkg.NormalizeTrack(raw, currentVersion))
}

func (a *app) resolveGitHubRelease(ctx context.Context, track releasepkg.Track) (releasepkg.Resolved, error) {
	if a.releaseResolver != nil {
		return a.releaseResolver(ctx, track)
	}
	resolved, _, err := releasepkg.NewClient().ResolveCached(ctx, track, a.githubReleaseCachePath())
	return resolved, err
}

// checkUpdateAvailability reads the local track selection and resolves only the
// compiled canonical GitHub repository. No configurable release endpoint,
// stored credential, or static catalog can influence this path.
func (a *app) checkUpdateAvailability() map[string]any {
	curRaw := strings.TrimSpace(fileio.ReadString(filepath.Join(a.dash, "VERSION"), ""))
	now := time.Now().Unix()
	profile, profileSource, profileErr := a.resolveUpdateTrack()
	trackName := normalizeReleaseTrack(profile["DASH_TRACK"], curRaw)
	track := releasepkg.Track(trackName)
	if profileErr != nil {
		return map[string]any{
			"ok": false, "status": "profile-error", "label": "Saved update track needs repair",
			"detail":         "Saved update-track state cannot be used: " + profileErr.Error() + ". Run install.sh --repair from SSH to recreate it.",
			"currentVersion": curRaw, "track": trackName, "fetchedAt": now, "profileSource": profileSource,
		}
	}
	resolved, err := a.resolveGitHubRelease(context.Background(), track)
	if err != nil {
		detail := fmt.Sprintf("GitHub Release discovery failed: %v. No release package was downloaded.", err)
		return map[string]any{
			"ok": false, "status": "unreachable", "label": "GitHub Release unavailable", "detail": detail,
			"currentVersion": curRaw, "track": trackName, "fetchedAt": now, "profileSource": profileSource,
			"problems": []string{detail},
		}
	}
	current, currentErr := releasepkg.ParseVersion(curRaw)
	available, availableErr := releasepkg.ParseVersion(resolved.Version)
	if currentErr != nil || availableErr != nil {
		detail := "Installed or resolved Dash-Go version is invalid; update is blocked until the local installation is repaired."
		return map[string]any{"ok": false, "status": "blocked", "label": "Version check blocked", "detail": detail, "track": trackName, "currentVersion": curRaw, "availableVersion": resolved.Version, "fetchedAt": now, "profileSource": profileSource, "problems": []string{detail}}
	}
	updateAvailable := available.Compare(current) > 0
	status, label := "current", "Up to date"
	detail := fmt.Sprintf("%s is current.", curRaw)
	if updateAvailable {
		status, label, detail = "available", "Update Available", fmt.Sprintf("%s is available.", resolved.Version)
	}
	releaseAsset := resolved.Assets["release"]
	checksumsAsset := resolved.Assets["checksums"]
	return map[string]any{
		"ok": true, "status": status, "label": label, "detail": detail,
		"source": "GitHub Releases", "repository": resolved.Repository, "releaseUrl": resolved.ReleaseURL,
		"track": trackName, "currentVersion": curRaw, "availableVersion": resolved.Version, "updateAvailable": updateAvailable,
		"releaseAsset": releaseAsset.Name, "releaseDigest": releaseAsset.Digest, "checksumsAsset": checksumsAsset.Name, "checksumsDigest": checksumsAsset.Digest,
		"immutable": resolved.Immutable, "fetchedAt": now, "profileSource": profileSource,
	}
}
