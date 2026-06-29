package calendar

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func (s *Service) managementRowsLocked(outputs map[string]bool) []map[string]any {
	// Reading Calendar Manager must not silently rewrite the manifest. Repair
	// calendar index is the explicit corrective action. Build a read-only view
	// from the current manifest plus direct managed local sources, so a missing
	// or stale manifest can be inspected before the user chooses Repair.
	entries := jsonutil.List(s.readJSONDefault(s.manifestPath(), []any{}))
	byURL := map[string]map[string]any{}
	for _, raw := range entries {
		row := jsonutil.Map(raw)
		url := jsonutil.StringValue(row["url"])
		key := SourceIdentity(url)
		if key == "" || byURL[key] != nil {
			continue
		}
		byURL[key] = row
	}
	for _, spec := range []struct{ dir, prefix string }{{s.calendarDir, "calendars/"}, {filepath.Join(s.dashDir, "calendar"), "calendar/"}} {
		paths, _ := filepath.Glob(filepath.Join(spec.dir, "*.ics"))
		slices.SortFunc(paths, func(left, right string) int { return compareFolded(filepath.Base(left), filepath.Base(right)) })
		for _, source := range paths {
			url := spec.prefix + filepath.Base(source)
			key := SourceIdentity(url)
			if key != "" && byURL[key] == nil {
				byURL[key] = map[string]any{"url": url}
			}
		}
	}
	rows := []map[string]any{}
	for _, entry := range byURL {
		rows = append(rows, s.managementRowLocked(jsonutil.StringValue(entry["url"]), entry, outputs))
	}
	for _, owner := range []string{"chore-wheel", "maintenance", "routines"} {
		if !s.appCalendarKnown(owner) {
			continue
		}
		url := "calendars/chore-wheel.ics"
		if owner == "maintenance" {
			url = "calendars/maintenance.ics"
		}
		if owner == "routines" {
			url = "calendars/routines.ics"
		}
		if _, found := byURL[SourceIdentity(url)]; !found {
			rows = append(rows, s.managementRowLocked(url, map[string]any{}, outputs))
		}
	}
	slices.SortStableFunc(rows, func(left, right map[string]any) int {
		leftApp, rightApp := left["kind"] == "app", right["kind"] == "app"
		if leftApp != rightApp {
			if leftApp {
				return -1
			}
			return 1
		}
		return compareFolded(jsonutil.StringValue(left["name"]), jsonutil.StringValue(right["name"]))
	})
	return rows
}

func (s *Service) managementRowLocked(url string, entry map[string]any, outputs map[string]bool) map[string]any {
	name, color := jsonutil.StringValue(entry["name"]), jsonutil.StringValue(entry["color"])
	enabled := CalendarEntryEnabled(entry)
	row := map[string]any{"url": SourceIdentity(url), "name": name, "color": color, "enabled": enabled, "exists": false}
	if owned, ok := OwnedSource(url); ok {
		owner := OwnedOwner(url)
		outputEnabled := outputEnabledFromSnapshot(outputs, url)
		if name == "" {
			name = owned.Name
		}
		if color == "" {
			color = owned.Color
		}
		row["name"], row["color"] = name, color
		row["kind"], row["owner"], row["deleteMode"] = "app", owner, "disable-app-calendar"
		row["outputEnabled"] = outputEnabled
		row["enabled"] = outputEnabled && enabled
		row["sourceLabel"] = "App calendar · " + map[string]string{"chore-wheel": "Chore Wheel", "maintenance": "Maintenance Tracker", "routines": "Routines"}[owner]
		row["exists"] = fileio.Exists(filepath.Join(s.calendarDir, filepath.Base(owned.URL)))
		return row
	}
	path, _, err := s.PathForURL(url)
	if err != nil {
		row["kind"], row["deleteMode"], row["sourceLabel"] = "unknown", "hide-only", "Unmanaged calendar source"
		return row
	}
	info, statErr := os.Lstat(path)
	if os.IsNotExist(statErr) {
		row["kind"], row["deleteMode"], row["sourceLabel"] = "missing", "hide-only", "Missing local calendar source"
		return row
	}
	if statErr != nil || info.IsDir() {
		row["kind"], row["deleteMode"], row["sourceLabel"] = "unknown", "hide-only", "Unavailable local calendar source"
		return row
	}
	row["exists"] = true
	if info.Mode()&os.ModeSymlink != 0 {
		row["kind"], row["deleteMode"], row["sourceLabel"] = "symlink", "remove-calendar-link", "Local calendar link"
	} else {
		row["kind"], row["deleteMode"], row["sourceLabel"] = "local", "delete-local-calendar", "Local calendar file"
	}
	return row
}

func (s *Service) ManagementStatus() map[string]any {
	if s == nil {
		return map[string]any{"calendars": []map[string]any{}, "trash": []any{}, "retentionDays": TrashRetentionDays}
	}
	outputs := s.outputSnapshot()
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.purgeExpiredTrashLocked()
	rows := s.managementRowsLocked(outputs)
	trashItems := make([]any, 0)
	for _, record := range s.loadTrashLocked() {
		trashItems = append(trashItems, map[string]any{"id": record.ID, "name": record.Name, "url": record.URL, "deletedAt": record.DeletedAt, "purgeAfter": record.PurgeAfter, "isSymlink": record.IsSymlink})
	}
	return map[string]any{"calendars": rows, "trash": trashItems, "retentionDays": TrashRetentionDays}
}

func (s *Service) Archive(url, displayName string) (TrashRecord, error) {
	if s == nil {
		return TrashRecord{}, fmt.Errorf("local calendar source is unavailable")
	}
	outputs := s.outputSnapshot()
	var record TrashRecord
	if err := func() error {
		s.mu.Lock()
		defer s.mu.Unlock()

		if OwnedOwner(url) != "" {
			return fmt.Errorf("app calendar output must be disabled from Calendar Manager")
		}
		path, canonical, err := s.PathForURL(url)
		if err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil || info.IsDir() {
			return fmt.Errorf("local calendar source is unavailable")
		}
		if err := os.MkdirAll(s.TrashDir(), 0755); err != nil {
			return err
		}
		now := s.now().UTC()
		record = TrashRecord{ID: TrashID(now), Name: strings.TrimSpace(displayName), URL: canonical, TrashName: TrashID(now) + "-" + filepath.Base(path), DeletedAt: now.Format(time.RFC3339), PurgeAfter: now.AddDate(0, 0, TrashRetentionDays).Format(time.RFC3339), WasEnabled: s.manifestEnabledLocked(canonical), IsSymlink: info.Mode()&os.ModeSymlink != 0}
		if record.Name == "" {
			record.Name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		}
		destination := filepath.Join(s.TrashDir(), record.TrashName)
		if err := os.Rename(path, destination); err != nil {
			return fmt.Errorf("move calendar to trash: %w", err)
		}
		records := append([]TrashRecord{record}, s.loadTrashLocked()...)
		if err := s.writeTrashLocked(records); err != nil {
			_ = os.Rename(destination, path)
			return err
		}
		if err := s.generateManifestLocked(outputs); err != nil {
			_ = s.writeTrashLocked(records[1:])
			if rollbackErr := os.Rename(destination, path); rollbackErr != nil {
				return fmt.Errorf("refresh calendar index: %w (calendar rollback: %v)", err, rollbackErr)
			}
			return fmt.Errorf("refresh calendar index: %w", err)
		}
		return nil
	}(); err != nil {
		return TrashRecord{}, err
	}
	// Refresh happens only after Calendar releases its mutation lock.
	if s.refreshCacheSync != nil {
		if err := s.refreshCacheSync(); err != nil {
			return TrashRecord{}, fmt.Errorf("calendar archived but event cache refresh failed: %w", err)
		}
	}
	return record, nil
}

func (s *Service) Restore(id string) (TrashRecord, error) {
	if s == nil {
		return TrashRecord{}, fmt.Errorf("deleted calendar is no longer available")
	}
	outputs := s.outputSnapshot()
	var record TrashRecord
	if err := func() error {
		s.mu.Lock()
		defer s.mu.Unlock()

		id = strings.TrimSpace(id)
		records := s.loadTrashLocked()
		original := append([]TrashRecord(nil), records...)
		index := -1
		for i, item := range records {
			if item.ID == id {
				record, index = item, i
				break
			}
		}
		if index < 0 {
			return fmt.Errorf("deleted calendar is no longer available")
		}
		path, _, err := s.PathForURL(record.URL)
		if err != nil {
			return err
		}
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("a calendar source already exists at the restore location")
		} else if !os.IsNotExist(err) {
			return err
		}
		from := filepath.Join(s.TrashDir(), record.TrashName)
		if err := os.Rename(from, path); err != nil {
			return fmt.Errorf("restore calendar: %w", err)
		}
		records = append(records[:index], records[index+1:]...)
		if err := s.writeTrashLocked(records); err != nil {
			_ = os.Rename(path, from)
			return err
		}
		if err := s.generateManifestLocked(outputs); err != nil {
			_ = s.writeTrashLocked(original)
			if rollbackErr := os.Rename(path, from); rollbackErr != nil {
				return fmt.Errorf("refresh calendar index: %w (calendar rollback: %v)", err, rollbackErr)
			}
			return fmt.Errorf("refresh calendar index: %w", err)
		}
		if err := s.setManifestEnabledLocked(record.URL, record.WasEnabled); err != nil {
			return fmt.Errorf("restore calendar visibility: %w", err)
		}
		return nil
	}(); err != nil {
		return TrashRecord{}, err
	}
	// Refresh happens only after Calendar releases its mutation lock.
	if s.refreshCacheSync != nil {
		if err := s.refreshCacheSync(); err != nil {
			return TrashRecord{}, fmt.Errorf("calendar restored but event cache refresh failed: %w", err)
		}
	}
	return record, nil
}

func (s *Service) SetOwnedOutput(owner string, enabled bool) (map[string]any, error) {
	owner = strings.TrimSpace(owner)
	switch owner {
	case "chore-wheel", "maintenance", "routines":
	default:
		return nil, fmt.Errorf("unknown app calendar")
	}
	if s == nil || s.setAppOutput == nil {
		return nil, fmt.Errorf("unknown app calendar")
	}
	return s.setAppOutput(owner, enabled)
}

func (s *Service) Repair() (map[string]any, error) {
	if s == nil {
		return nil, fmt.Errorf("calendar index is unavailable")
	}
	outputs := s.outputSnapshot()
	before, after := 0, 0
	if err := func() error {
		s.mu.Lock()
		defer s.mu.Unlock()

		before = len(jsonutil.List(s.readJSONDefault(s.manifestPath(), []any{})))
		if err := s.generateManifestLocked(outputs); err != nil {
			return err
		}
		after = len(jsonutil.List(s.readJSONDefault(s.manifestPath(), []any{})))
		return nil
	}(); err != nil {
		return nil, err
	}
	// Refresh happens only after Calendar releases its mutation lock.
	if s.refreshCacheSync != nil {
		if err := s.refreshCacheSync(); err != nil {
			return nil, fmt.Errorf("calendar index repaired but event cache refresh failed: %w", err)
		}
	}
	return map[string]any{"ok": true, "before": before, "after": after, "removed": max(0, before-after)}, nil
}

func (s *Service) purgeExpiredTrashLocked() int {
	records := s.loadTrashLocked()
	if len(records) == 0 {
		return 0
	}
	now := s.now()
	keep := make([]TrashRecord, 0, len(records))
	removed := 0
	for _, record := range records {
		until, _ := time.Parse(time.RFC3339, record.PurgeAfter)
		if now.Before(until) {
			keep = append(keep, record)
			continue
		}
		if err := os.Remove(filepath.Join(s.TrashDir(), record.TrashName)); err == nil || os.IsNotExist(err) {
			removed++
			continue
		}
		keep = append(keep, record)
	}
	if removed > 0 {
		_ = s.writeTrashLocked(keep)
	}
	return removed
}
