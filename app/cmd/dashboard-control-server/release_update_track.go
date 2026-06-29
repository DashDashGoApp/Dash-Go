package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
)

var errDashboardUpdateTrackBusy = errors.New("cannot switch update tracks while an update is active")

// alternateReleaseTrack returns the only other supported channel. The saved
// value is normalized first so a missing/legacy track follows the same
// installed-version fallback used by update availability checks.
func alternateReleaseTrack(current string) string {
	if normalizeReleaseTrack(current, "") == "beta" {
		return "stable"
	}
	return "beta"
}

func privateUpdateFile(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("saved update state is not a regular file")
	}
	if info.Mode().Perm()&0077 != 0 {
		return nil, fmt.Errorf("saved update state permissions must be owner-only")
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("saved update state is too large")
	}
	return os.ReadFile(path)
}

// updateEnvWithTrack deliberately discards all historical content. Before the
// GitHub Release migration this file could contain an arbitrary host and
// authentication material. The canonical repository is compiled into the
// resolver now, so the only durable shell-compatible state is the track.
func updateEnvWithTrack(track string) string {
	track = normalizeReleaseTrack(track, "")
	return "# Dash-Go update track; canonical GitHub Releases are compiled into Dash-Go.\n" +
		"DASH_TRACK=" + track + "\n"
}

func stagePrivateUpdateFile(path string, body []byte) (string, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".track-*")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return "", err
	}
	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return "", err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return "", err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	return tmpPath, nil
}

func commitPrivateUpdateFile(staged, path string) error {
	if err := os.Rename(staged, path); err != nil {
		return err
	}
	if dir, err := os.Open(filepath.Dir(path)); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	return nil
}

func writePrivateUpdateFile(path string, body []byte) error {
	staged, err := stagePrivateUpdateFile(path, body)
	if err != nil {
		return err
	}
	defer os.Remove(staged)
	return commitPrivateUpdateFile(staged, path)
}

func privateUpdateProfileBody(track string) ([]byte, error) {
	profile := savedUpdateProfile{Schema: updateProfileSchema, Track: normalizeReleaseTrack(track, "")}
	body, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(body, '\n'), nil
}

// toggleUpdateTrack changes only the selected release channel. It deliberately
// does not start an update or expose legacy connection values. Both private
// records are staged before replacement, then rewritten in their sanitized
// v2 form. A missing old record is not an error: beta.34 recreates it with the
// installed-version default and thereby completes an interrupted migration.
func (a *app) toggleUpdateTrack() (map[string]any, error) {
	a.updateMu.Lock()
	defer a.updateMu.Unlock()

	a.reconcileInterruptedUpdateStateLocked()
	job := a.readUpdateJob()
	lockHeld, err := a.updateLockHeld()
	if err != nil {
		return nil, fmt.Errorf("could not inspect the update lock: %w", err)
	}
	if updateStateActive(strOr(job["state"], "")) || lockHeld {
		return nil, errDashboardUpdateTrackBusy
	}

	installed := fileio.ReadString(filepath.Join(a.dash, "VERSION"), "")
	trackRaw := ""
	if profile, err := readPrivateUpdateProfile(a.updateProfilePath()); err == nil {
		trackRaw = profile["DASH_TRACK"]
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	// Existing legacy env state is intentionally not parsed or sourced here.
	// The installer owns its one-time migration; an absent profile falls back
	// safely to the installed version rather than trusting shell content.
	if _, err := privateUpdateFile(a.updateEnvPath(), 64*1024); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	current := normalizeReleaseTrack(trackRaw, installed)
	next := alternateReleaseTrack(current)
	envPath := a.updateEnvPath()
	envStage, err := stagePrivateUpdateFile(envPath, []byte(updateEnvWithTrack(next)))
	if err != nil {
		return nil, err
	}
	defer os.Remove(envStage)

	profilePath := a.updateProfilePath()
	profileBody, err := privateUpdateProfileBody(next)
	if err != nil {
		return nil, err
	}
	profileStage, err := stagePrivateUpdateFile(profilePath, profileBody)
	if err != nil {
		return nil, err
	}
	defer os.Remove(profileStage)

	// Commit the shell file first: even if the subsequent profile rename fails,
	// the legacy arbitrary-host/credential state is not retained in the
	// companion file. The next repair/toggle recreates the small JSON profile.
	if err := commitPrivateUpdateFile(envStage, envPath); err != nil {
		return nil, err
	}
	if err := commitPrivateUpdateFile(profileStage, profilePath); err != nil {
		return nil, fmt.Errorf("could not save the Dashboard Control update-track profile: %w", err)
	}

	a.updateAvailabilityMu.Lock()
	a.updateAvailabilityCache = nil
	a.updateAvailabilityAt = time.Time{}
	a.updateAvailabilityMu.Unlock()
	return map[string]any{"ok": true, "track": next, "profileSource": "private-json-v2"}, nil
}
