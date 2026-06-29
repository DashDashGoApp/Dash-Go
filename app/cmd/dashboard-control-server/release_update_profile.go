package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// updateProfileSchema stores only the durable Stable/Beta selection. Update
// discovery is compiled to the canonical GitHub repository, so release hosts
// and credentials never belong on household devices.
const updateProfileSchema = 2
const legacyUpdateProfileSchema = 1

type savedUpdateProfile struct {
	Schema int    `json:"schema"`
	Track  string `json:"track"`
}

func (a *app) updateEnvPath() string { return filepath.Join(a.home, ".dashboard-update.env") }
func (a *app) updateProfilePath() string {
	return filepath.Join(a.home, ".dashboard-update-profile.json")
}

func readPrivateUpdateProfile(path string) (map[string]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("profile is not a regular file")
	}
	if info.Size() > 32*1024 {
		return nil, fmt.Errorf("profile is too large")
	}
	if info.Mode().Perm()&0077 != 0 {
		return nil, fmt.Errorf("profile permissions must be owner-only")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 32*1024))
	var profile savedUpdateProfile
	if err := decoder.Decode(&profile); err != nil {
		return nil, fmt.Errorf("invalid JSON")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("profile contains more than one JSON value")
		}
		return nil, fmt.Errorf("invalid trailing JSON")
	}
	if profile.Schema != updateProfileSchema && profile.Schema != legacyUpdateProfileSchema {
		return nil, fmt.Errorf("unsupported schema")
	}
	return map[string]string{
		"DASH_TRACK": strings.TrimSpace(profile.Track),
		"schema":     fmt.Sprint(profile.Schema),
	}, nil
}

// resolveUpdateTrack reads only a track preference. Old update-host credentials
// are intentionally ignored even while the installer performs its one-time
// owner-only migration.
func (a *app) resolveUpdateTrack() (map[string]string, string, error) {
	profile, err := readPrivateUpdateProfile(a.updateProfilePath())
	if err == nil {
		if profile["schema"] == fmt.Sprint(legacyUpdateProfileSchema) {
			return profile, "private-json-v1-pending-migration", nil
		}
		return profile, "private-json-v2", nil
	}
	if !os.IsNotExist(err) {
		return nil, "private-json", err
	}
	return map[string]string{"DASH_TRACK": os.Getenv("DASH_TRACK")}, "process-env", nil
}
