package writeback

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/calendar/icalwrite"
	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
)

type Result struct {
	Source     string `json:"source"`
	UID        string `json:"uid"`
	Action     string `json:"action"`
	Collection string `json:"-"`
	Pair       string `json:"-"`
	Provider   string `json:"-"`
	// FinalDelete is true only when this completed delete left the exact trusted
	// vdir collection without any remaining regular .ics items. It is never
	// inferred merely from Action, because --force-delete may only be armed for
	// that narrow final-event state.
	FinalDelete bool `json:"-"`
}

func (s *Service) Create(source string, event icalwrite.Event) (Result, error) {
	cal, err := s.Resolve(source)
	if err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(event.UID) == "" {
		event.UID = icalwrite.NewUID(s.now())
	}
	body, err := event.Serialize(s.now())
	if err != nil {
		return Result{}, err
	}
	path := filepath.Join(cal.Collection, event.UID+".ics")
	if err := s.ensureItemPath(cal, path); err != nil {
		return Result{}, err
	}
	if _, statErr := os.Lstat(path); statErr == nil {
		return Result{}, errors.New("calendar event identifier already exists")
	} else if !os.IsNotExist(statErr) {
		return Result{}, statErr
	}
	if err := fileio.WriteAtomic(path, []byte(body), 0600); err != nil {
		return Result{}, fmt.Errorf("save calendar event: %w", err)
	}
	return Result{Source: cal.Source, UID: event.UID, Action: "created", Collection: cal.Collection, Pair: cal.Pair, Provider: cal.Provider}, nil
}
func (s *Service) Update(source, uid string, event icalwrite.Event) (Result, error) {
	cal, err := s.Resolve(source)
	if err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(uid) == "" {
		return Result{}, errors.New("event identifier required")
	}
	path, src, err := s.findItem(cal, uid)
	if err != nil {
		return Result{}, err
	}
	event.UID = uid
	body, err := icalwrite.ApplyEdit(src, event, s.now())
	if err != nil {
		return Result{}, err
	}
	if err := fileio.WriteAtomic(path, []byte(body), 0600); err != nil {
		return Result{}, fmt.Errorf("save calendar event: %w", err)
	}
	return Result{Source: cal.Source, UID: uid, Action: "updated", Collection: cal.Collection, Pair: cal.Pair, Provider: cal.Provider}, nil
}
func (s *Service) UpdateSeries(source, uid string, event icalwrite.Event) (Result, error) {
	cal, err := s.Resolve(source)
	if err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(uid) == "" {
		return Result{}, errors.New("event identifier required")
	}
	path, src, err := s.findItem(cal, uid)
	if err != nil {
		return Result{}, err
	}
	event.UID = uid
	if !icalwrite.SeriesEditable(src, uid) {
		return Result{}, errors.New("this repeating schedule uses an advanced repeat pattern; manage the series in its calendar app")
	}
	body, err := icalwrite.ApplySeriesEdit(src, event, s.now())
	if err != nil {
		return Result{}, err
	}
	if err := fileio.WriteAtomic(path, []byte(body), 0600); err != nil {
		return Result{}, fmt.Errorf("save recurring series: %w", err)
	}
	return Result{Source: cal.Source, UID: uid, Action: "series-updated", Collection: cal.Collection, Pair: cal.Pair, Provider: cal.Provider}, nil
}

func (s *Service) UpdateOccurrence(source, uid string, occurrence time.Time, event icalwrite.Event) (Result, error) {
	cal, err := s.Resolve(source)
	if err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(uid) == "" || occurrence.IsZero() {
		return Result{}, errors.New("calendar occurrence required")
	}
	path, src, err := s.findItem(cal, uid)
	if err != nil {
		return Result{}, err
	}
	if err := icalwrite.ValidateOccurrenceTarget(src, uid, occurrence); err != nil {
		return Result{}, err
	}
	event.UID = uid
	body, err := icalwrite.ApplyOccurrenceEdit(src, uid, occurrence, event, s.now())
	if err != nil {
		return Result{}, err
	}
	if err := fileio.WriteAtomic(path, []byte(body), 0600); err != nil {
		return Result{}, fmt.Errorf("save calendar occurrence: %w", err)
	}
	return Result{Source: cal.Source, UID: uid, Action: "occurrence-updated", Collection: cal.Collection, Pair: cal.Pair, Provider: cal.Provider}, nil
}

func (s *Service) Delete(source, uid string) (Result, error) {
	cal, err := s.Resolve(source)
	if err != nil {
		return Result{}, err
	}
	path, src, err := s.findItem(cal, uid)
	if err != nil {
		return Result{}, err
	}
	if icalwrite.HasSchedulingProperties(src) || icalwrite.IsRecurring(src) || icalwrite.HasRecurrenceID(src) {
		return Result{}, errors.New("this event must be managed in its calendar app")
	}
	if err := fileio.RemoveDurable(path); err != nil {
		return Result{}, fmt.Errorf("delete calendar event: %w", err)
	}
	// Only the local collection's actual post-delete state may authorize the
	// wrapper's destructive one-run override. If inspection fails, fail closed:
	// the event is already deleted locally, but the later sync retains its normal
	// empty-collection guard.
	remaining, inspectErr := collectionFiles(cal.Collection)
	finalDelete := inspectErr == nil && len(remaining) == 0
	return Result{Source: cal.Source, UID: uid, Action: "deleted", Collection: cal.Collection, Pair: cal.Pair, Provider: cal.Provider, FinalDelete: finalDelete}, nil
}
func (s *Service) SkipOccurrence(source, uid string, occurrence time.Time) (Result, error) {
	cal, err := s.Resolve(source)
	if err != nil {
		return Result{}, err
	}
	if occurrence.IsZero() {
		return Result{}, errors.New("occurrence time required")
	}
	path, src, err := s.findItem(cal, uid)
	if err != nil {
		return Result{}, err
	}
	if overridden, err := icalwrite.OccurrenceHasOverride(src, uid, occurrence); err != nil {
		return Result{}, err
	} else if overridden {
		return Result{}, errors.New("this occurrence already has a custom change; edit this occurrence instead")
	}
	body, err := icalwrite.AppendExdate(src, occurrence, false, s.now())
	if err != nil {
		return Result{}, err
	}
	if err := fileio.WriteAtomic(path, []byte(body), 0600); err != nil {
		return Result{}, fmt.Errorf("skip calendar occurrence: %w", err)
	}
	return Result{Source: cal.Source, UID: uid, Action: "skipped", Collection: cal.Collection, Pair: cal.Pair, Provider: cal.Provider}, nil
}

func (s *Service) ensureItemPath(cal Calendar, path string) error {
	if err := s.validateCollection(cal.Collection); err != nil {
		return err
	}
	if filepath.Dir(path) != filepath.Clean(cal.Collection) || filepath.Ext(path) != ".ics" {
		return errors.New("invalid calendar event path")
	}
	return os.MkdirAll(cal.Collection, 0700)
}
func (s *Service) findItem(cal Calendar, uid string) (string, string, error) {
	if strings.TrimSpace(uid) == "" || strings.ContainsAny(uid, "/\\\r\n") {
		return "", "", errors.New("invalid event identifier")
	}
	// Do not trust the conventional <UID>.ics filename as unique. Providers may
	// choose different filenames, and a duplicate UID must fail closed rather
	// than letting a recurrence edit touch whichever file happens to be first.
	files, err := collectionFiles(cal.Collection)
	if err != nil {
		return "", "", err
	}
	var foundPath, foundBody string
	needle := []byte(uid)
	for _, path := range files {
		body, err := safeReadItem(path)
		// Avoid full iCalendar parsing for the overwhelming majority of unrelated
		// vdir items. A byte match is only a prefilter: every possible match still
		// passes HasUID, preserving the existing duplicate-UID fail-closed rule.
		if err != nil || !bytes.Contains(body, needle) || !icalwrite.HasUID(string(body), uid) {
			continue
		}
		if foundPath != "" {
			return "", "", errors.New("calendar event identifier is ambiguous")
		}
		foundPath, foundBody = path, string(body)
	}
	if foundPath == "" {
		return "", "", errors.New("calendar event is no longer available")
	}
	return foundPath, foundBody, nil
}
func safeReadItem(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("calendar item is not a regular file")
	}
	return os.ReadFile(path)
}
func collectionFiles(root string) ([]string, error) {
	root = filepath.Clean(root)
	files := []string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".ics") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.Sort(files)
	return files, nil
}
