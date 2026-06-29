package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

const (
	maxConfigBackupEntries    = 2000
	maxConfigBackupEntryBytes = int64(8 << 20)
	maxConfigBackupTotalBytes = int64(64 << 20)
)

type configRestoreEntry struct {
	file *zip.File
	root string
	rel  string
}

// configBackupCalendarLinks reads optional link metadata from modern backups.
// Older archives did not record links, so a missing metadata entry remains a
// valid empty-link restore path.
func configBackupCalendarLinks(zr *zip.ReadCloser) ([]calendarBackupLink, error) {
	var meta *zip.File
	for _, f := range zr.File {
		if f.Name != "backup-meta.json" {
			continue
		}
		if meta != nil {
			return nil, errors.New("backup contains duplicate metadata")
		}
		meta = f
	}
	if meta == nil {
		return []calendarBackupLink{}, nil
	}
	if meta.UncompressedSize64 > uint64(maxConfigBackupEntryBytes) {
		return nil, errors.New("backup metadata is too large")
	}
	rc, err := meta.Open()
	if err != nil {
		return nil, err
	}
	data, readErr := io.ReadAll(io.LimitReader(rc, maxConfigBackupEntryBytes+1))
	closeErr := rc.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if int64(len(data)) > maxConfigBackupEntryBytes {
		return nil, errors.New("backup metadata is too large")
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, errors.New("backup metadata is malformed")
	}
	value, ok := raw["calendarLinks"]
	if !ok {
		return []calendarBackupLink{}, nil
	}
	value = bytes.TrimSpace(value)
	if len(value) == 0 || bytes.Equal(value, []byte("null")) || value[0] != '[' {
		return nil, errors.New("backup calendar link metadata is malformed")
	}
	links := []calendarBackupLink{}
	if err := json.Unmarshal(value, &links); err != nil {
		return nil, errors.New("backup calendar link metadata is malformed")
	}
	return normalizeCalendarBackupLinks(links)
}

func validateConfigBackupRestoreLinks(entries []configRestoreEntry, links []calendarBackupLink) error {
	regularCalendars := make(map[string]bool)
	for _, entry := range entries {
		if entry.root == "calendars" {
			regularCalendars[filepath.ToSlash(entry.rel)] = true
		}
	}
	for _, link := range links {
		if regularCalendars[link.Path] {
			return fmt.Errorf("backup calendar link metadata conflicts with archived file: %s", link.Path)
		}
	}
	return nil
}

// configBackupRestorePlan validates every extracted destination and all declared
// archive bounds before live configuration is touched.
func configBackupRestorePlan(zr *zip.ReadCloser) ([]configRestoreEntry, error) {
	if len(zr.File) > maxConfigBackupEntries {
		return nil, errors.New("backup has too many entries")
	}
	allowed := map[string]bool{"config": true, "calendars": true, "secrets": true}
	seen := map[string]bool{}
	entries := []configRestoreEntry{}
	retiredFamilyBoard, privateFamilyBoard := false, false
	var declaredTotal int64
	for _, f := range zr.File {
		arc := filepath.ToSlash(f.Name)
		if f.FileInfo().IsDir() {
			continue
		}
		if strings.HasPrefix(arc, "/") || strings.Contains(arc, "../") {
			continue
		}
		parts := strings.SplitN(arc, "/", 2)
		if len(parts) != 2 || !allowed[parts[0]] {
			continue
		}
		rel := filepath.Clean(filepath.FromSlash(parts[1]))
		if rel == "." || rel == "" || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return nil, errors.New("backup contains an unsafe path")
		}
		key := parts[0] + "/" + filepath.ToSlash(rel)
		if parts[0] == "secrets" && key != "secrets/family-board.json" && key != "secrets/family-board-inbox-pins.json" && key != "secrets/apprise-routes.json" && key != "secrets/terminal-access.env" {
			return nil, errors.New("backup contains an unknown private secret")
		}
		if seen[key] {
			return nil, errors.New("backup contains duplicate restore entries")
		}
		seen[key] = true
		if f.UncompressedSize64 > uint64(maxConfigBackupEntryBytes) {
			return nil, errors.New("backup entry too large")
		}
		declaredTotal += int64(f.UncompressedSize64)
		if declaredTotal > maxConfigBackupTotalBytes {
			return nil, errors.New("backup too large")
		}
		if key == "config/family-board.json" {
			retiredFamilyBoard = true
			continue
		}
		if key == "secrets/family-board.json" {
			privateFamilyBoard = true
		}
		entries = append(entries, configRestoreEntry{file: f, root: parts[0], rel: rel})
	}
	if retiredFamilyBoard && !privateFamilyBoard {
		return nil, errors.New("backup contains retired config/family-board.json without secrets/family-board.json")
	}
	return entries, nil
}

// stageConfigBackup extracts only validated, allowlisted files under cache. A
// malformed or oversized archive therefore never partially changes live state.
func (a *app) stageConfigBackup(entries []configRestoreEntry, calendarLinks []calendarBackupLink) (string, int, error) {
	stage, err := os.MkdirTemp(a.cacheDir, ".config-restore-")
	if err != nil {
		return "", 0, err
	}
	cleanup := func(err error) (string, int, error) {
		_ = os.RemoveAll(stage)
		return "", 0, err
	}
	for _, root := range []string{"config", "calendars", "secrets"} {
		if err := os.MkdirAll(filepath.Join(stage, root), 0755); err != nil {
			return cleanup(err)
		}
	}
	var total int64
	for _, entry := range entries {
		dest := filepath.Join(stage, entry.root, entry.rel)
		root := filepath.Join(stage, entry.root)
		if !strings.HasPrefix(dest, root+string(os.PathSeparator)) {
			return cleanup(errors.New("backup contains an unsafe path"))
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return cleanup(err)
		}
		rc, err := entry.file.Open()
		if err != nil {
			return cleanup(err)
		}
		mode := os.FileMode(0644)
		if entry.root == "secrets" {
			mode = 0600
		}
		out, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
		if err != nil {
			_ = rc.Close()
			return cleanup(err)
		}
		n, copyErr := io.Copy(out, io.LimitReader(rc, maxConfigBackupEntryBytes+1))
		closeErr := out.Close()
		rcErr := rc.Close()
		if copyErr != nil {
			return cleanup(copyErr)
		}
		if closeErr != nil {
			return cleanup(closeErr)
		}
		if rcErr != nil {
			return cleanup(rcErr)
		}
		if n > maxConfigBackupEntryBytes {
			return cleanup(errors.New("backup entry too large"))
		}
		total += n
		if total > maxConfigBackupTotalBytes {
			return cleanup(errors.New("backup too large"))
		}
	}
	for _, link := range calendarLinks {
		dest := filepath.Join(stage, "calendars", filepath.FromSlash(link.Path))
		root := filepath.Join(stage, "calendars")
		if !strings.HasPrefix(dest, root+string(os.PathSeparator)) {
			return cleanup(errors.New("backup contains an unsafe calendar link path"))
		}
		if _, err := os.Lstat(dest); err == nil {
			return cleanup(fmt.Errorf("backup calendar link destination already exists: %s", link.Path))
		} else if !os.IsNotExist(err) {
			return cleanup(err)
		}
		if err := os.Symlink(link.Target, dest); err != nil {
			return cleanup(err)
		}
	}
	return stage, len(entries) + len(calendarLinks), nil
}

// restoreTerminalAccess restores the optional SSH-only terminal switch only
// when a modern backup explicitly carries it. Older archives leave the
// current administrator choice untouched.
func (a *app) restoreTerminalAccess(stage string) error {
	data, err := os.ReadFile(filepath.Join(stage, "secrets", "terminal-access.env"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	enabled, valid := parseTerminalAccess(data)
	if !valid {
		return errors.New("terminal access backup is malformed")
	}
	return a.setTerminalAccessEnabled(enabled)
}

// restoreFamilyBoardPrivateStore restores the owner-only Family Message Board
// document carried by current backups.
func (a *app) restoreFamilyBoardPrivateStore(stage string) error {
	path := filepath.Join(stage, "secrets", "family-board.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return errors.New("Family Message Board backup is malformed")
	}
	payload := normalizeFamilyBoardPayload(raw)
	if err := a.writeFamilyBoardPrivatePayload(payload); err != nil {
		return err
	}
	return nil
}

// restoreFamilyBoardInboxPins applies the staged PIN-verifier file only after
// its schema is normalized. No PIN value is ever recovered or logged.
func (a *app) restoreFamilyBoardInboxPins(stage string) error {
	path := filepath.Join(stage, "secrets", "family-board-inbox-pins.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return errors.New("inbox PIN backup is malformed")
	}
	normalized := normalizeFamilyBoardInboxPins(raw)
	if len(jsonutil.Map(raw["pins"])) > 0 && len(jsonutil.Map(normalized["pins"])) == 0 {
		return errors.New("inbox PIN backup contains no valid verifier records")
	}
	return a.writeFamilyBoardInboxPins(normalized)
}

// replaceRestoreTrees swaps the authoritative config/calendar trees only after
// staging and the pre-restore backup have both succeeded. The old roots are
// retained until both swaps complete so a filesystem failure can be rolled back.
func (a *app) replaceRestoreTrees(stage string) error {
	type tree struct{ name, live, old string }
	tag := fmt.Sprintf(".restore-old-%d", time.Now().UnixNano())
	trees := []tree{
		{name: "config", live: a.configDir, old: a.configDir + tag},
		{name: "calendars", live: a.calDir, old: a.calDir + tag},
	}
	renamed := []tree{}
	rollback := func() {
		for _, t := range trees {
			_ = os.RemoveAll(t.live)
		}
		for i := len(renamed) - 1; i >= 0; i-- {
			_ = os.Rename(renamed[i].old, renamed[i].live)
		}
	}
	for _, t := range trees {
		if _, err := os.Stat(t.live); err == nil {
			if err := os.Rename(t.live, t.old); err != nil {
				rollback()
				return err
			}
			renamed = append(renamed, t)
		} else if !os.IsNotExist(err) {
			rollback()
			return err
		}
	}
	for _, t := range trees {
		if err := os.Rename(filepath.Join(stage, t.name), t.live); err != nil {
			rollback()
			return err
		}
	}
	for _, t := range renamed {
		_ = os.RemoveAll(t.old)
	}
	return nil
}

func (a *app) restoreConfigBackup(name string) (map[string]any, error) {
	chosen, err := a.chooseBackup(name)
	if err != nil {
		return nil, err
	}
	zr, err := zip.OpenReader(filepath.Join(a.backupDir(), chosen))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	entries, err := configBackupRestorePlan(zr)
	if err != nil {
		return nil, err
	}
	calendarLinks, err := configBackupCalendarLinks(zr)
	if err != nil {
		return nil, err
	}
	if err := validateConfigBackupRestoreLinks(entries, calendarLinks); err != nil {
		return nil, err
	}
	stage, restored, err := a.stageConfigBackup(entries, calendarLinks)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stage)
	pre, err := a.createConfigBackup("pre-restore", "Automatic backup before restoring saved config", "restore", false)
	if err != nil {
		return nil, fmt.Errorf("pre-restore backup failed: %w", err)
	}
	if err := a.replaceRestoreTrees(stage); err != nil {
		return nil, fmt.Errorf("restore swap failed; pre-restore backup retained: %w", err)
	}
	if err := a.restoreFamilyBoardPrivateStore(stage); err != nil {
		return nil, fmt.Errorf("restore private Family Message Board store failed; pre-restore backup retained: %w", err)
	}
	if err := a.restoreFamilyBoardInboxPins(stage); err != nil {
		return nil, fmt.Errorf("restore private inbox PIN verifier failed; pre-restore backup retained: %w", err)
	}
	if err := a.restoreAppriseRoutes(stage); err != nil {
		return nil, fmt.Errorf("restore private Apprise routes failed; pre-restore backup retained: %w", err)
	}
	if err := a.restoreTerminalAccess(stage); err != nil {
		return nil, fmt.Errorf("restore terminal access setting failed; pre-restore backup retained: %w", err)
	}
	a.invalidateSettingsCache()
	_, _ = a.refreshEventCache(true, 90, 365)
	pruned := a.pruneConfigBackups(a.configBackupKeepLimit())
	return map[string]any{"ok": true, "name": chosen, "restored": restored, "preBackup": pre["name"], "pruned": pruned["removedCount"], "backups": a.listConfigBackups()}, nil
}
