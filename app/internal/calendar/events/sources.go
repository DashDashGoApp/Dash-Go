package events

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"hash"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

var reHexSix = regexp.MustCompile(`^[0-9a-fA-F]{6}$`)
var reAbsoluteURL = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*://`)

func calendarEntryEnabled(m map[string]any) bool {
	v, ok := m["enabled"]
	if !ok {
		return true
	}
	s := strings.ToLower(jsonutil.TextValue(v))
	if s == "" {
		return true
	}
	return jsonutil.Truthy(v)
}

func defaultSourceIdentity(url string) string {
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

func (s *Service) loadCalendars() []CalendarSource {
	arr := jsonutil.List(readJSONDefault(filepath.Join(s.calendarDir, "calendars.json"), []any{}))
	if len(arr) > 0 {
		// calendars.json is the normal visibility-control surface. Keep its first
		// declaration for each source, including an explicit disabled entry, so a
		// generated Dash-Go feed cannot be re-added under a second label.
		seen, out := map[string]bool{}, []CalendarSource{}
		for _, raw := range arr {
			m := jsonutil.Map(raw)
			url := legacyText(m["url"])
			key := s.sourceIdentity(url)
			if key == "" || !s.outputEnabled(url) || seen[key] {
				continue
			}
			seen[key] = true
			if !calendarEntryEnabled(m) {
				continue
			}
			source := CalendarSource{URL: url, Name: legacyText(m["name"]), Color: legacyText(m["color"]), Tag: legacyText(m["tag"]), Owner: legacyText(m["owner"])}
			if owned, ok := s.ownedSource(url); ok {
				if source.Name == "" {
					source.Name = owned.Name
				}
				if source.Color == "" {
					source.Color = owned.Color
				}
				if source.Owner == "" {
					source.Owner = owned.Owner
				}
			}
			out = append(out, source)
		}
		return out
	}
	out := []CalendarSource{}
	for _, spec := range []struct{ dir, prefix string }{{s.calendarDir, "calendars/"}, {s.dashDir, ""}} {
		entries, _ := os.ReadDir(spec.dir)
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".ics") {
				continue
			}
			if !s.outputEnabled(spec.prefix + e.Name()) {
				continue
			}
			out = append(out, s.calendarFromFilename(e.Name(), spec.prefix))
		}
	}
	slices.SortStableFunc(out, func(left, right CalendarSource) int { return strings.Compare(left.URL, right.URL) })
	return out
}

func (s *Service) calendarFromFilename(fn, prefix string) CalendarSource {
	if owned, ok := s.ownedSource(prefix + fn); ok {
		return owned
	}
	palette := map[string]string{"green": "#8fc4a6", "blue": "#8bb4d4", "red": "#d99a9a", "gold": "#d9c074", "violet": "#9a8fb0", "purple": "#9a8fb0", "amber": "#cda76a", "teal": "#7fc4c4", "orange": "#d9a878"}
	base := strings.TrimSuffix(fn, ".ics")
	parts := strings.Split(base, ".")
	name := strings.ReplaceAll(strings.ReplaceAll(parts[0], "-", " "), "_", " ")
	if name != "" {
		name = strings.ToUpper(name[:1]) + name[1:]
	}
	color, tag := "#7fd6a8", ""
	for _, part := range parts[1:] {
		low := strings.ToLower(part)
		if low == "holiday" {
			tag = "holiday"
		} else if reHexSix.MatchString(part) {
			color = "#" + part
		} else if v, ok := palette[low]; ok {
			color = v
		} else if part != "" {
			color = part
		}
	}
	return CalendarSource{URL: prefix + fn, Name: name, Color: color, Tag: tag}
}

func (s *Service) eventURLToPath(url string) string {
	if url == "" || strings.HasPrefix(url, "/") || reAbsoluteURL.MatchString(url) {
		return ""
	}
	u := strings.Split(strings.Split(url, "?")[0], "#")[0]
	p := filepath.Clean(filepath.Join(s.dashDir, u))
	dashClean := filepath.Clean(s.dashDir)
	if p != dashClean && strings.HasPrefix(p, dashClean+string(os.PathSeparator)) {
		return p
	}
	return ""
}

func (s *Service) statSources(cals []CalendarSource) []SourceMeta {
	out := []SourceMeta{}
	for _, cal := range cals {
		out = append(out, s.sourceMeta(cal.URL, cal.Name, cal.Color, cal.Tag, cal.Owner, s.eventURLToPath(cal.URL)))
	}
	calFile := filepath.Join(s.calendarDir, "calendars.json")
	if fileio.Exists(calFile) {
		out = append(out, s.sourceMeta("calendars.json", "calendars.json", "", "meta", "", calFile))
	}
	return out
}

func (s *Service) sourceMeta(url, name, color, tag, owner, path string) SourceMeta {
	item := SourceMeta{URL: url, Name: name, Color: color, Tag: tag, Owner: owner, Path: "", Exists: false, RealPath: "", IsSymlink: false}
	if path != "" {
		item.Path = filepath.Base(path)
		if rp, err := filepath.EvalSymlinks(path); err == nil {
			item.RealPath = rp
		} else {
			item.RealPath = path
		}
	}
	if path == "" {
		return item
	}
	st, err := os.Lstat(path)
	if err != nil {
		return item
	}
	item.Exists = true
	mtime := st.ModTime().UnixMilli()
	size := st.Size()
	item.MtimeMs = &mtime
	item.Size = &size
	if st.Mode()&os.ModeSymlink != 0 {
		item.IsSymlink = true
	}
	if abs, err := filepath.Abs(path); err == nil && item.RealPath != "" && abs != item.RealPath {
		item.IsSymlink = true
	}
	if h, err := fileHashHex(path, sha256.New()); err == nil {
		item.SHA256 = &h
	} else {
		item.HashError = err.Error()
	}
	return item
}

func eventFingerprint(sources []SourceMeta, start, end time.Time) string {
	payload := map[string]any{"sources": sources, "windowStart": epochMs(start), "windowEnd": epochMs(end), "version": FingerprintVersion}
	b, _ := json.Marshal(payload)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func fileHashHex(path string, h hash.Hash) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func readJSONDefault(path string, def any) any {
	var v any
	b, err := os.ReadFile(path)
	if err != nil {
		return def
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return def
	}
	return v
}

func legacyText(v any) string { return jsonutil.TextValue(v) }
