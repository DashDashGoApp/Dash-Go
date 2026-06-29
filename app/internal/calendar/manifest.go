package calendar

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

var (
	reHexSix   = regexp.MustCompile(`^[0-9A-Fa-f]{6}$`)
	reHexColor = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
)

func CalendarEntryEnabled(row map[string]any) bool { return row["enabled"] != false }

// SourceIdentity canonicalizes managed local source names and keeps historical
// case-insensitive manifest matching behavior intact.
func SourceIdentity(url string) string {
	value := strings.TrimSpace(url)
	if value == "" {
		return ""
	}
	local := strings.TrimPrefix(value, "./")
	lower := strings.ToLower(local)
	if strings.HasPrefix(lower, "calendars/") || strings.HasPrefix(lower, "calendar/") {
		value = strings.TrimPrefix(filepath.ToSlash(filepath.Clean(local)), "./")
	}
	return strings.ToLower(value)
}

func OwnedOwner(url string) string {
	switch SourceIdentity(url) {
	case "calendars/chore-wheel.ics":
		return "chore-wheel"
	case "calendars/maintenance.ics":
		return "maintenance"
	case "calendars/routines.ics":
		return "routines"
	default:
		return ""
	}
}

// OwnedSource holds the one canonical presentation for Dash-Go-generated feeds.
func OwnedSource(url string) (Source, bool) {
	switch OwnedOwner(url) {
	case "chore-wheel":
		return Source{URL: "calendars/chore-wheel.ics", Name: "Chores", Color: "#7fc4c4", Owner: "chore-wheel"}, true
	case "maintenance":
		return Source{URL: "calendars/maintenance.ics", Name: "Maintenance", Color: "#d9c074", Owner: "maintenance"}, true
	case "routines":
		return Source{URL: "calendars/routines.ics", Name: "Routines", Color: "#a999d4", Owner: "routines"}, true
	default:
		return Source{}, false
	}
}

func (s *Service) outputEnabledForURL(url string) bool {
	owner := OwnedOwner(url)
	if owner == "" || s == nil || s.outputEnabled == nil {
		return true
	}
	return s.outputEnabled(owner)
}

// outputSnapshot reads household-owned output flags before Calendar takes its
// mutex. This is critical because household commits hold their own lock while
// entering Calendar; querying them after Calendar is locked would invert the
// established household → Calendar transaction order.
func (s *Service) outputSnapshot() map[string]bool {
	state := map[string]bool{}
	if s == nil || s.outputEnabled == nil {
		return state
	}
	for _, owner := range []string{"chore-wheel", "maintenance", "routines"} {
		state[owner] = s.outputEnabled(owner)
	}
	return state
}

func outputEnabledFromSnapshot(state map[string]bool, url string) bool {
	owner := OwnedOwner(url)
	if owner == "" {
		return true
	}
	enabled, found := state[owner]
	return !found || enabled
}

func (s *Service) appCalendarKnown(owner string) bool {
	if s == nil || s.appKnown == nil {
		return false
	}
	return s.appKnown(owner)
}

func compareFolded(left, right string) int {
	left, right = strings.ToLower(strings.TrimSpace(left)), strings.ToLower(strings.TrimSpace(right))
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}

func (s *Service) readJSONDefault(path string, def any) any {
	b, err := os.ReadFile(path)
	if err != nil {
		return def
	}
	var value any
	if json.Unmarshal(b, &value) != nil {
		return def
	}
	return value
}

func (s *Service) manifestPath() string { return filepath.Join(s.calendarDir, "calendars.json") }

// GenerateManifest reconciles direct local ICS source files into the durable
// manifest while preserving explicitly hidden entries and canonical app feeds.
func (s *Service) GenerateManifest() error {
	if s == nil {
		return nil
	}
	outputs := s.outputSnapshot()
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.generateManifestLocked(outputs)
}

func (s *Service) generateManifestLocked(outputs map[string]bool) error {
	if err := os.MkdirAll(s.calendarDir, 0755); err != nil {
		return err
	}
	disabled := map[string]bool{}
	if b, err := os.ReadFile(filepath.Join(s.homeDir, ".dashboard-disabled-calendars")); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.ToLower(strings.TrimSpace(line))
			if line != "" {
				disabled[line] = true
			}
		}
	}

	previous := map[string]map[string]any{}
	remember := func(key string, row map[string]any) {
		if key == "" {
			return
		}
		// Duplicate registrations are repaired into one canonical source. If a
		// user hid any duplicate, preserve the safer hidden intent rather than
		// unexpectedly showing a calendar while repairing the index.
		if old := previous[key]; old != nil && !CalendarEntryEnabled(old) {
			return
		}
		if !CalendarEntryEnabled(row) {
			copy := jsonutil.CloneMap(row)
			copy["enabled"] = false
			previous[key] = copy
			return
		}
		previous[key] = row
	}
	for _, raw := range jsonutil.List(s.readJSONDefault(s.manifestPath(), []any{})) {
		row := jsonutil.Map(raw)
		if url := strings.ToLower(strings.TrimSpace(jsonutil.StringValue(row["url"]))); url != "" {
			remember("url:"+url, row)
		}
		if name := strings.ToLower(strings.TrimSpace(jsonutil.StringValue(row["name"]))); name != "" {
			remember("name:"+name, row)
		}
	}

	type foundCal struct{ path, prefix string }
	found := []foundCal{}
	seenReal := map[string]bool{}
	for _, spec := range []struct{ dir, prefix string }{{s.calendarDir, "calendars/"}, {filepath.Join(s.dashDir, "calendar"), "calendar/"}} {
		paths, _ := filepath.Glob(filepath.Join(spec.dir, "*.ics"))
		slices.SortFunc(paths, func(left, right string) int { return compareFolded(filepath.Base(left), filepath.Base(right)) })
		for _, source := range paths {
			real := source
			if resolved, err := filepath.EvalSymlinks(source); err == nil {
				real = resolved
			}
			real = filepath.Clean(real)
			if seenReal[real] {
				continue
			}
			seenReal[real] = true
			found = append(found, foundCal{path: source, prefix: spec.prefix})
		}
	}

	entries := []map[string]any{}
	for _, item := range found {
		base := strings.TrimSuffix(filepath.Base(item.path), ".ics")
		parts := strings.SplitN(base, ".", 3)
		name := parts[0]
		if name == "" || disabled[strings.ToLower(name)] {
			continue
		}
		color, tag := defaultColor, ""
		for _, part := range parts[1:] {
			if part == "" {
				continue
			}
			low := strings.ToLower(part)
			switch {
			case low == "holiday":
				tag = "holiday"
			case palette[low] != "":
				color = palette[low]
			case reHexSix.MatchString(part):
				color = "#" + part
			case reHexColor.MatchString(part):
				color = part
			}
		}
		display := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(name, "-", " "), "_", " "))
		if display != "" {
			display = strings.ToUpper(display[:1]) + display[1:]
		}
		url := item.prefix + filepath.Base(item.path)
		if !outputEnabledFromSnapshot(outputs, url) {
			continue
		}
		owner := ""
		if owned, ok := OwnedSource(url); ok {
			display, color, tag, owner = owned.Name, owned.Color, owned.Tag, owned.Owner
		}
		obj := map[string]any{"url": url, "name": display, "color": color, "enabled": true}
		if owner != "" {
			obj["owner"] = owner
		}
		if old := previous["url:"+strings.ToLower(url)]; old != nil {
			obj["enabled"] = CalendarEntryEnabled(old)
		} else if old := previous["name:"+strings.ToLower(display)]; old != nil {
			obj["enabled"] = CalendarEntryEnabled(old)
		}
		if tag != "" {
			obj["tag"] = tag
		}
		entries = append(entries, obj)
	}
	out, _ := json.Marshal(entries)
	path := s.manifestPath()
	if old, err := os.ReadFile(path); err == nil && string(old) == string(out) {
		return nil
	}
	if err := fileio.WriteAtomic(path, out, 0644); err != nil {
		return err
	}
	s.appendLog("calendars.log", fmt.Sprintf("%s: wrote calendars/calendars.json -> %s\n", s.now().Format(time.ANSIC), string(out)))
	s.trimLog("calendars.log", 400, 200)
	return nil
}

func (s *Service) Calendars() any {
	if s == nil {
		return []any{}
	}
	_ = s.GenerateManifest()
	return s.readJSONDefault(s.manifestPath(), []any{})
}

func (s *Service) Toggle(name, url string) (map[string]any, error) {
	if s == nil {
		return nil, fmt.Errorf("unknown calendar")
	}
	outputs := s.outputSnapshot()
	s.mu.Lock()
	_ = s.generateManifestLocked(outputs)
	items := jsonutil.List(s.readJSONDefault(s.manifestPath(), []any{}))
	for _, raw := range items {
		row := jsonutil.Map(raw)
		matchesURL := url != "" && strings.EqualFold(jsonutil.StringValue(row["url"]), url)
		matchesName := name != "" && strings.EqualFold(jsonutil.StringValue(row["name"]), name)
		if matchesURL || matchesName {
			enabled := !CalendarEntryEnabled(row)
			row["enabled"] = enabled
			if err := fileio.WriteJSON(s.manifestPath(), items); err != nil {
				s.mu.Unlock()
				return nil, err
			}
			result := map[string]any{"ok": true, "name": jsonutil.StringValue(row["name"]), "url": jsonutil.StringValue(row["url"]), "enabled": enabled, "calendars": items}
			s.mu.Unlock()
			if s.refreshCacheSync != nil {
				_ = s.refreshCacheSync()
			}
			return result, nil
		}
	}
	s.mu.Unlock()
	return nil, fmt.Errorf("unknown calendar")
}

// OutputEnabledForURL exposes the visibility policy to the event-cache child
// without giving that child access to household app state.
func (s *Service) OutputEnabledForURL(url string) bool { return s.outputEnabledForURL(url) }
