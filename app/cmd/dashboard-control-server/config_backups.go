package main

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func (a *app) backupDir() string { return filepath.Join(a.cacheDir, "config-backups") }
func (a *app) configBackupKeepLimit() int {
	return clamp(jsonutil.Int(os.Getenv("DASH_CONFIG_BACKUP_KEEP"), 50), 5, 200)
}
func (a *app) ensureBackupDir() error {
	if err := os.MkdirAll(a.backupDir(), 0700); err != nil {
		return err
	}
	return os.Chmod(a.backupDir(), 0700)
}
func (a *app) safeZipMeta(path string) map[string]any {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return map[string]any{}
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name != "backup-meta.json" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return map[string]any{}
		}
		defer rc.Close()
		var m map[string]any
		if json.NewDecoder(rc).Decode(&m) == nil {
			return m
		}
	}
	return map[string]any{}
}

type configBackupRecord struct {
	Name     string
	FullPath string
	Size     int64
	Modified time.Time
	Meta     map[string]any
}

// configBackupRecords discovers regular archive files inside Dash-Go's private
// backup directory. HTTP-provided names only select an existing record later;
// they never become a filesystem path. Symlinked/device backup entries are
// deliberately ignored so restore/delete cannot follow a swapped-in object.
func (a *app) configBackupRecords() []configBackupRecord {
	if a.ensureBackupDir() != nil {
		return []configBackupRecord{}
	}
	ents, err := os.ReadDir(a.backupDir())
	if err != nil {
		return []configBackupRecord{}
	}
	out := []configBackupRecord{}
	for _, e := range ents {
		name := e.Name()
		if e.IsDir() || filepath.Base(name) != name || !strings.HasSuffix(name, ".zip") {
			continue
		}
		full := filepath.Join(a.backupDir(), name)
		st, err := os.Lstat(full)
		if err != nil || !st.Mode().IsRegular() {
			continue
		}
		out = append(out, configBackupRecord{Name: name, FullPath: full, Size: st.Size(), Modified: st.ModTime(), Meta: a.safeZipMeta(full)})
	}
	slices.SortFunc(out, func(left, right configBackupRecord) int {
		return right.Modified.Compare(left.Modified)
	})
	return out
}

func configBackupRecordPayload(record configBackupRecord) map[string]any {
	meta := record.Meta
	kind := jsonutil.TextValue(meta["kind"])
	if kind == "" {
		kind = "manual"
	}
	reason := jsonutil.TextValue(meta["reason"])
	if reason == "" {
		reason = "Manual backup"
	}
	return map[string]any{"name": record.Name, "size": record.Size, "mtime": record.Modified.Unix(), "version": strOr(meta["version"], ""), "createdAt": jsonutil.Int(meta["createdAt"], 0), "kind": kind, "reason": reason, "preAction": strOr(meta["preAction"], "")}
}

func (a *app) listConfigBackups() []map[string]any {
	records := a.configBackupRecords()
	out := make([]map[string]any, 0, len(records))
	for _, record := range records {
		out = append(out, configBackupRecordPayload(record))
	}
	return out
}
func strOr(v any, def string) string {
	s := jsonutil.TextValue(v)
	if s == "" {
		return def
	}
	return s
}
func sanitizeName(s string, max int) string {
	re := reBackupFilenameSafe
	s = re.ReplaceAllString(s, "_")
	if s == "" {
		s = "unknown"
	}
	if len(s) > max {
		s = s[:max]
	}
	return s
}
func (a *app) createConfigBackup(kind, reason, preAction string, prune bool) (map[string]any, error) {
	if err := a.ensureBackupDir(); err != nil {
		return nil, err
	}
	ver := strings.TrimSpace(fileio.ReadString(filepath.Join(a.dash, "VERSION"), "unknown"))
	if ver == "" {
		ver = "unknown"
	}
	if kind == "" {
		kind = "manual"
	}
	if reason == "" {
		reason = "Manual backup"
	}
	name := fmt.Sprintf("dashboard-config-%s-%s-v%s.zip", time.Now().Format("20060102-150405"), sanitizeName(kind, 32), sanitizeName(ver, 48))
	path := filepath.Join(a.backupDir(), name)
	tmp, err := os.CreateTemp(a.backupDir(), "."+name+".tmp-")
	if err != nil {
		return nil, err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return nil, err
	}
	z := zip.NewWriter(tmp)
	abort := func(err error) (map[string]any, error) {
		_ = z.Close()
		_ = tmp.Close()
		return nil, err
	}
	meta := map[string]any{"createdAt": time.Now().Unix(), "version": ver, "dashboard": "dash-go", "kind": kind, "reason": reason, "preAction": preAction, "retentionKeep": a.configBackupKeepLimit()}
	count := 0
	// Migrate the legacy config-tree Board document before collecting config/ so
	// a newly created backup never contains a second, browser-addressable copy.
	if err := a.ensureFamilyBoardPrivateStore(); err != nil {
		return abort(err)
	}
	n, err := a.addZipTree(z, a.configDir, "config", nil)
	if err != nil {
		return abort(err)
	}
	count += n
	calendarLinks := []calendarBackupLink{}
	n, err = a.addZipTree(z, a.calDir, "calendars", &calendarLinks)
	if err != nil {
		return abort(err)
	}
	count += n
	calendarLinks, err = normalizeCalendarBackupLinks(calendarLinks, a.calendarBackupLinkPolicy())
	if err != nil {
		return abort(err)
	}
	if len(calendarLinks) > 0 {
		meta["calendarLinks"] = calendarLinks
	}
	mb, err := json.MarshalIndent(meta, "", " ")
	if err != nil {
		return abort(err)
	}
	mw, err := z.Create("backup-meta.json")
	if err != nil {
		return abort(err)
	}
	if _, err := mw.Write(mb); err != nil {
		return abort(err)
	}
	count++
	// Family Board private data and personal inbox PIN verifiers live outside
	// config/ because legacy static layouts can serve that tree. Include both
	// only in this owner-only Dashboard Control backup.
	for _, secret := range []struct{ source, name string }{
		{a.familyBoardFile(), "secrets/family-board.json"},
		{a.familyBoardInboxPinsFile(), "secrets/family-board-inbox-pins.json"},
		{a.appriseRoutesFile(), "secrets/apprise-routes.json"},
		{a.terminalAccessFile(), "secrets/terminal-access.env"},
	} {
		if _, err := os.Stat(secret.source); err == nil {
			if err := addZipFile(z, secret.source, secret.name, 0600); err != nil {
				return abort(err)
			}
			count++
		} else if !os.IsNotExist(err) {
			return abort(err)
		}
	}
	for _, rel := range []string{"VERSION", "manifest.json"} {
		p := filepath.Join(a.dash, rel)
		if data, err := os.ReadFile(p); err == nil {
			w, err := z.Create(rel)
			if err != nil {
				return abort(err)
			}
			if _, err := w.Write(data); err != nil {
				return abort(err)
			}
			count++
		}
	}
	if err := z.Close(); err != nil {
		_ = tmp.Close()
		return nil, err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0600); err != nil {
		_ = os.Remove(path)
		return nil, err
	}
	validated, err := a.validateConfigBackupArchive(path)
	if err != nil {
		_ = os.Remove(path)
		return nil, fmt.Errorf("backup validation failed: %w", err)
	}
	pruned := map[string]any{"removedCount": 0, "removed": []string{}}
	if prune {
		pruned = a.pruneConfigBackups(a.configBackupKeepLimit())
	}
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "name": name, "file": path, "size": st.Size(), "files": count, "validatedFiles": validated, "calendarLinks": len(calendarLinks), "kind": kind, "reason": reason, "backupKeep": a.configBackupKeepLimit(), "pruned": pruned["removedCount"], "backups": a.listConfigBackups()}, nil
}
func (a *app) pruneConfigBackups(keep int) map[string]any {
	keep = clamp(keep, 5, 200)
	backups := a.configBackupRecords()
	removed := []string{}
	for i, record := range backups {
		if i < keep {
			continue
		}
		if os.Remove(record.FullPath) == nil {
			removed = append(removed, record.Name)
		}
	}
	return map[string]any{"ok": true, "keep": keep, "removed": removed, "removedCount": len(removed), "backups": a.listConfigBackups()}
}

func (a *app) chooseBackup(name string) (configBackupRecord, error) {
	name = strings.TrimSpace(name)
	records := a.configBackupRecords()
	if len(records) == 0 {
		return configBackupRecord{}, errors.New("no local config backups found")
	}
	if name == "" {
		return records[0], nil
	}
	if filepath.Base(name) != name || !strings.HasSuffix(name, ".zip") {
		return configBackupRecord{}, errors.New("invalid backup name")
	}
	for _, record := range records {
		if record.Name == name {
			return record, nil
		}
	}
	return configBackupRecord{}, errors.New("backup not found")
}

func (a *app) deleteConfigBackup(name string) (map[string]any, error) {
	chosen, err := a.chooseBackup(name)
	if err != nil {
		return nil, err
	}
	if err := os.Remove(chosen.FullPath); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "deleted": chosen.Name, "backups": a.listConfigBackups()}, nil
}
