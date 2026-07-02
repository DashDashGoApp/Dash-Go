package writeback

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
)

// MergeCollection recreates the dashboard's derived one-file source from one
// trusted vdir collection. It deliberately copies component bodies verbatim,
// preserving VEVENT, VTIMEZONE, VALARM, and provider properties.
func (s *Service) MergeCollection(source, collection string) error {
	if err := s.validateCollection(collection); err != nil {
		return err
	}
	dest, err := s.sourcePath(source)
	if err != nil {
		return err
	}
	files, err := collectionFiles(collection)
	if err != nil {
		return err
	}
	var body bytes.Buffer
	body.WriteString("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Dash-Go//vdirsyncer//EN\r\nCALSCALE:GREGORIAN\r\n")
	eventCount := 0
	for _, path := range files {
		raw, err := safeReadItem(path)
		if err != nil {
			return err
		}
		components, events := calendarComponents(string(raw))
		eventCount += events
		for _, line := range components {
			body.WriteString(line)
			body.WriteString("\r\n")
		}
	}
	body.WriteString("END:VCALENDAR\r\n")
	// An empty remote collection is valid. Writing an empty derived calendar is
	// essential after deleting the final local item; retaining an old mirror
	// would make a deleted event reappear until the next full sync.
	_ = eventCount
	if err := fileio.WriteAtomic(dest, body.Bytes(), 0644); err != nil {
		return fmt.Errorf("merge calendar collection: %w", err)
	}
	return nil
}
func calendarComponents(src string) ([]string, int) {
	raw := strings.Split(strings.ReplaceAll(strings.ReplaceAll(src, "\r\n", "\n"), "\r", "\n"), "\n")
	out := []string{}
	depth, events := 0, 0
	for _, line := range raw {
		line = strings.TrimSuffix(line, "\r")
		upper := strings.ToUpper(line)
		if upper == "BEGIN:VCALENDAR" || upper == "END:VCALENDAR" || line == "" {
			continue
		}
		if strings.HasPrefix(upper, "BEGIN:V") {
			depth++
			if upper == "BEGIN:VEVENT" {
				events++
			}
			out = append(out, line)
			continue
		}
		if strings.HasPrefix(upper, "END:V") {
			if depth > 0 {
				out = append(out, line)
				depth--
			}
			continue
		}
		if depth > 0 {
			out = append(out, line)
		}
	}
	return out, events
}

func (s *Service) CollectionForSource(source string) (string, error) {
	cal, err := s.Resolve(source)
	if err != nil {
		return "", err
	}
	return filepath.Clean(cal.Collection), nil
}
func (s *Service) SourceForCollection(collection string) (string, error) {
	status, err := s.Status()
	if err != nil {
		return "", err
	}
	want := filepath.Clean(collection)
	for _, cal := range status.Calendars {
		if filepath.Clean(cal.Collection) == want {
			return cal.Source, nil
		}
	}
	return "", os.ErrNotExist
}
