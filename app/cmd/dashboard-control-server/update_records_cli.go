package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Update records are private durable status data. Keeping their atomic write
// path separate prevents runner/status persistence from being coupled to
// release-manifest verification or payload cleanup.

func writeJSONPrivateFile(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	// Best-effort directory sync makes the atomic rename durable on filesystems
	// that support it; metadata status must never be written with broad modes.
	if dir, err := os.Open(filepath.Dir(path)); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	return nil
}

func runUpdateRecordCLI(args []string, kind string) int {
	fs := flag.NewFlagSet(kind, flag.ContinueOnError)
	file := fs.String("file", "", "record JSON file")
	state := fs.String("state", "", "state")
	label := fs.String("label", "", "label")
	detail := fs.String("detail", "", "detail")
	code := fs.Int("code", 0, "exit code")
	source := fs.String("source", "", "source")
	target := fs.String("target", "", "target")
	track := fs.String("track", "", "track")
	version := fs.String("version", "", "version")
	previous := fs.String("previous-version", "", "previous version")
	jobID := fs.String("job-id", "", "job ID")
	stage := fs.String("stage", "", "rollback stage")
	healthChecked := fs.String("health-checked", "", "health checked true/false")
	rolledBack := fs.String("rolled-back", "", "rolled back true/false")
	rollbackAttempted := fs.String("rollback-attempted", "", "rollback attempted true/false")
	rollbackSucceeded := fs.String("rollback-succeeded", "", "rollback succeeded true/false")
	if err := fs.Parse(args); err != nil {
		return 64
	}
	if *file == "" {
		fmt.Fprintln(os.Stderr, "--file required")
		return 64
	}
	current := map[string]any{}
	if b, err := os.ReadFile(*file); err == nil {
		_ = json.Unmarshal(b, &current)
	}
	setString := func(key, value string) {
		if value != "" {
			current[key] = value
		}
	}
	setString("state", *state)
	setString("label", *label)
	setString("detail", *detail)
	setString("source", *source)
	setString("target", *target)
	setString("track", *track)
	setString("version", *version)
	setString("previousVersion", *previous)
	if kind == "update-job" {
		setString("id", *jobID)
	} else {
		setString("jobId", *jobID)
	}
	setString("stage", *stage)
	if *code != 0 || *state == "failed" || *state == "rolledback" || *state == "success" {
		current["exitCode"] = *code
	}
	if *healthChecked != "" {
		current["healthChecked"] = strings.EqualFold(*healthChecked, "true") || *healthChecked == "1"
	}
	if *rolledBack != "" {
		current["rolledBack"] = strings.EqualFold(*rolledBack, "true") || *rolledBack == "1"
	}
	if *rollbackAttempted != "" {
		current["rollbackAttempted"] = strings.EqualFold(*rollbackAttempted, "true") || *rollbackAttempted == "1"
	}
	if *rollbackSucceeded != "" {
		current["rollbackSucceeded"] = strings.EqualFold(*rollbackSucceeded, "true") || *rollbackSucceeded == "1"
	}
	current["updatedAt"] = time.Now().Unix()
	current["schema"] = 1
	if kind == "update-job" && current["requestedAt"] == nil {
		current["requestedAt"] = time.Now().Unix()
	}
	if err := writeJSONPrivateFile(*file, current); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func (a *app) runUpdateStatusCLI(args []string) int { return runUpdateRecordCLI(args, "update-status") }
func (a *app) runUpdateJobCLI(args []string) int    { return runUpdateRecordCLI(args, "update-job") }
