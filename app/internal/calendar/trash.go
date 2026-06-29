package calendar

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func (s *Service) TrashDir() string  { return filepath.Join(s.calendarDir, ".trash") }
func (s *Service) TrashFile() string { return filepath.Join(s.TrashDir(), "calendars.json") }

func trashDefault() map[string]any { return map[string]any{"schema": TrashSchema, "items": []any{}} }

func TrashRecordFrom(raw any) (TrashRecord, bool) {
	row := jsonutil.Map(raw)
	record := TrashRecord{
		ID: jsonutil.StringValue(row["id"]), Name: jsonutil.StringValue(row["name"]), URL: jsonutil.StringValue(row["url"]),
		TrashName: jsonutil.StringValue(row["trashName"]), DeletedAt: jsonutil.StringValue(row["deletedAt"]), PurgeAfter: jsonutil.StringValue(row["purgeAfter"]),
		WasEnabled: jsonutil.Truthy(row["wasEnabled"]), IsSymlink: jsonutil.Truthy(row["isSymlink"]),
	}
	if record.ID == "" || record.Name == "" || record.URL == "" || record.TrashName == "" {
		return TrashRecord{}, false
	}
	if _, err := time.Parse(time.RFC3339, record.DeletedAt); err != nil {
		return TrashRecord{}, false
	}
	if _, err := time.Parse(time.RFC3339, record.PurgeAfter); err != nil {
		return TrashRecord{}, false
	}
	if _, _, err := LocalPathForURL(record.URL, "", ""); err != nil {
		return TrashRecord{}, false
	}
	if filepath.Base(record.TrashName) != record.TrashName || !strings.HasSuffix(strings.ToLower(record.TrashName), ".ics") {
		return TrashRecord{}, false
	}
	return record, true
}

func (s *Service) loadTrashLocked() []TrashRecord {
	state := jsonutil.Map(s.readJSONDefault(s.TrashFile(), trashDefault()))
	out := []TrashRecord{}
	seen := map[string]bool{}
	for _, raw := range jsonutil.List(state["items"]) {
		record, ok := TrashRecordFrom(raw)
		if !ok || seen[record.ID] {
			continue
		}
		seen[record.ID] = true
		out = append(out, record)
	}
	slices.SortStableFunc(out, func(left, right TrashRecord) int {
		if left.DeletedAt > right.DeletedAt {
			return -1
		}
		if left.DeletedAt < right.DeletedAt {
			return 1
		}
		return 0
	})
	if len(out) > TrashLimit {
		out = out[:TrashLimit]
	}
	return out
}

func (s *Service) LoadTrash() []TrashRecord {
	if s == nil {
		return []TrashRecord{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadTrashLocked()
}

func (s *Service) writeTrashLocked(records []TrashRecord) error {
	items := make([]any, 0, len(records))
	for _, record := range records {
		items = append(items, map[string]any{
			"id": record.ID, "name": record.Name, "url": record.URL, "trashName": record.TrashName,
			"deletedAt": record.DeletedAt, "purgeAfter": record.PurgeAfter, "wasEnabled": record.WasEnabled, "isSymlink": record.IsSymlink,
		})
	}
	return fileio.WriteJSON(s.TrashFile(), map[string]any{"schema": TrashSchema, "items": items})
}

func (s *Service) WriteTrash(records []TrashRecord) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeTrashLocked(records)
}

func TrashID(now time.Time) string { return fmt.Sprintf("calendar_%d", now.UnixNano()) }

// LocalPathForURL accepts only direct local .ics sources under Dash-Go's two
// managed directories. It deliberately does not resolve symlinks: Calendar
// Trash moves the Dash-Go symlink itself, never its external target.
func LocalPathForURL(url, calDir, dashDir string) (string, string, error) {
	raw := strings.TrimSpace(url)
	if raw == "" || strings.Contains(raw, "\\") || strings.Contains(raw, "://") || strings.HasPrefix(raw, "/") {
		return "", "", fmt.Errorf("calendar source is not a managed local file")
	}
	clean := path.Clean(strings.TrimPrefix(raw, "./"))
	if clean != raw && clean != strings.TrimPrefix(raw, "./") {
		return "", "", fmt.Errorf("calendar source path is not canonical")
	}
	if strings.Contains(clean, "..") || !strings.HasSuffix(strings.ToLower(clean), ".ics") {
		return "", "", fmt.Errorf("calendar source path is invalid")
	}
	var dir, relative string
	switch {
	case strings.HasPrefix(clean, "calendars/"):
		dir, relative = calDir, strings.TrimPrefix(clean, "calendars/")
	case strings.HasPrefix(clean, "calendar/"):
		dir, relative = filepath.Join(dashDir, "calendar"), strings.TrimPrefix(clean, "calendar/")
	default:
		return "", "", fmt.Errorf("calendar source is not in a managed calendar directory")
	}
	if relative == "" || strings.Contains(relative, "/") {
		return "", "", fmt.Errorf("calendar source must be a direct calendar file")
	}
	base := path.Base(clean)
	if base == "." || base == ".." || strings.Contains(base, "/") {
		return "", "", fmt.Errorf("calendar source filename is invalid")
	}
	full := filepath.Join(dir, base)
	if filepath.Clean(filepath.Dir(full)) != filepath.Clean(dir) {
		return "", "", fmt.Errorf("calendar source path escapes its directory")
	}
	return full, clean, nil
}

func (s *Service) PathForURL(url string) (string, string, error) {
	return LocalPathForURL(url, s.calendarDir, s.dashDir)
}

func (s *Service) manifestEnabledLocked(url string) bool {
	for _, raw := range jsonutil.List(s.readJSONDefault(s.manifestPath(), []any{})) {
		row := jsonutil.Map(raw)
		if SourceIdentity(jsonutil.StringValue(row["url"])) == SourceIdentity(url) {
			return CalendarEntryEnabled(row)
		}
	}
	return true
}

func (s *Service) setManifestEnabledLocked(url string, enabled bool) error {
	items := jsonutil.List(s.readJSONDefault(s.manifestPath(), []any{}))
	for _, raw := range items {
		row := jsonutil.Map(raw)
		if SourceIdentity(jsonutil.StringValue(row["url"])) == SourceIdentity(url) {
			row["enabled"] = enabled
			return fileio.WriteJSON(s.manifestPath(), items)
		}
	}
	return fmt.Errorf("calendar source is missing from the manifest")
}

// PurgeExpiredTrash removes only entries past their durable retention time.
func (s *Service) PurgeExpiredTrash() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
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
