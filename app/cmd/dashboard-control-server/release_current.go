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
	if availableErr != nil {
		detail := "Resolved Dash-Go release metadata contains an invalid version; no update can start."
		return map[string]any{"ok": false, "status": "blocked", "label": "Version check blocked", "detail": detail, "track": trackName, "currentVersion": curRaw, "availableVersion": resolved.Version, "fetchedAt": now, "profileSource": profileSource, "problems": []string{detail}}
	}
	releaseAsset := resolved.Assets["release"]
	checksumsAsset := resolved.Assets["checksums"]
	base := map[string]any{
		"ok": true, "source": "GitHub Releases", "repository": resolved.Repository, "releaseUrl": resolved.ReleaseURL,
		"track": trackName, "currentVersion": curRaw, "availableVersion": resolved.Version,
		"releaseAsset": releaseAsset.Name, "releaseDigest": releaseAsset.Digest, "checksumsAsset": checksumsAsset.Name, "checksumsDigest": checksumsAsset.Digest,
		"immutable": resolved.Immutable, "fetchedAt": now, "profileSource": profileSource,
	}
	// A missing or malformed installed VERSION is the one recovery exception to
	// normal strict-monotonic updates. The installer will still stage/verify the
	// resolved immutable release before replacing anything, but Control must not
	// force the user into an unsafe manual same-version reinstall first.
	if currentErr != nil {
		base["status"] = "recovery"
		base["label"] = "Installed version needs repair"
		base["detail"] = "The installed Dash-Go version is missing or invalid. A verified selected-track release can repair the installation."
		base["comparison"] = "invalid-installed"
		base["updateAvailable"] = true
		base["recoveryUpdate"] = true
		return base
	}
	comparison := "equal"
	status, label := "current", "Up to date"
	detail := fmt.Sprintf("%s is current.", curRaw)
	updateAvailable := false
	if cmp := available.Compare(current); cmp > 0 {
		comparison, status, label, detail, updateAvailable = "newer", "available", "Update available", fmt.Sprintf("%s is available.", resolved.Version), true
	} else if cmp < 0 {
		comparison, status, label, detail = "installed-newer", "installed-newer", "Installed version is newer", fmt.Sprintf("Installed %s is newer than selected %s release %s. Dash-Go will not downgrade.", curRaw, trackName, resolved.Version)
	}
	base["status"] = status
	base["label"] = label
	base["detail"] = detail
	base["comparison"] = comparison
	base["updateAvailable"] = updateAvailable
	base["noDowngrade"] = comparison == "installed-newer"
	return base
}
