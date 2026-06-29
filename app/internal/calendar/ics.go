package calendar

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
)

var reICSUIDSafe = regexp.MustCompile(`[^A-Za-z0-9_.-]+`)

func DateOnly(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}
func AllDayEvent(year int, month time.Month, day int, summary, uid string) Event {
	return Event{Date: DateOnly(year, month, day), Summary: summary, UID: uid}
}

func RemoveFile(path string) bool { return os.Remove(path) == nil }

func icsEsc(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `;`, `\;`)
	value = strings.ReplaceAll(value, `,`, `\,`)
	value = strings.ReplaceAll(value, "\n", `\n`)
	return value
}
func ymd(value time.Time) string    { return value.UTC().Format("20060102") }
func zstamp(value time.Time) string { return value.UTC().Format("20060102T150405Z") }

// WriteICSFile preserves the legacy generated-feed layout, key ordering,
// deterministic UID construction, and app-owner marker.
func WriteICSFile(path, name string, events []Event) error {
	now := time.Now().UTC().Format("20060102T150405Z")
	var body strings.Builder
	body.WriteString("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Dash-Go//Go Calendars//EN\r\n")
	body.WriteString("X-WR-CALNAME:" + icsEsc(name) + "\r\n")
	for index, event := range events {
		uid := event.UID
		if uid == "" {
			uid = "event"
		}
		uid = reICSUIDSafe.ReplaceAllString(uid, "-")
		if len(uid) > 48 {
			uid = uid[:48]
		}
		body.WriteString("BEGIN:VEVENT\r\n")
		body.WriteString(fmt.Sprintf("UID:%s-%s-%d@dash-go\r\nDTSTAMP:%s\r\n", uid, ymd(event.Date), index+1, now))
		if event.Start != nil {
			end := event.Start.Add(5 * time.Minute)
			if event.End != nil {
				end = *event.End
			}
			body.WriteString("DTSTART:" + zstamp(*event.Start) + "\r\nDTEND:" + zstamp(end) + "\r\n")
		} else {
			body.WriteString("DTSTART;VALUE=DATE:" + ymd(event.Date) + "\r\nDTEND;VALUE=DATE:" + ymd(event.Date.AddDate(0, 0, 1)) + "\r\n")
			body.WriteString("TRANSP:TRANSPARENT\r\n")
		}
		body.WriteString("SUMMARY:" + icsEsc(event.Summary) + "\r\n")
		if event.Description != "" {
			body.WriteString("DESCRIPTION:" + icsEsc(event.Description) + "\r\n")
		}
		if owner := strings.TrimSpace(event.AppOwner); owner != "" {
			body.WriteString("X-DASHGO-APP-OWNER:" + icsEsc(owner) + "\r\n")
		}
		body.WriteString("END:VEVENT\r\n")
	}
	body.WriteString("END:VCALENDAR\r\n")
	return fileio.WriteAtomic(path, []byte(body.String()), 0644)
}

func (s *Service) ownedCalendarPath(owner string) string {
	switch owner {
	case "chore-wheel":
		return filepath.Join(s.calendarDir, "chore-wheel.ics")
	case "maintenance":
		return filepath.Join(s.calendarDir, "maintenance.ics")
	case "routines":
		return filepath.Join(s.calendarDir, "routines.ics")
	default:
		return ""
	}
}

// CommitOwnedFeed is the Calendar-owned transaction for household app output:
// render or remove the dedicated ICS feed, durably save the app payload through
// a narrow callback, rebuild the manifest, and wake the event cache after the
// Calendar lock is released. The callback must not call Calendar again.
type OwnedFeedCommit struct {
	Owner       string
	Name        string
	Events      []Event
	Enabled     bool
	OutputState map[string]bool
	Save        func() error
}

func (s *Service) CommitOwnedFeed(commit OwnedFeedCommit) error {
	if s == nil {
		if commit.Save != nil {
			return commit.Save()
		}
		return nil
	}
	path := s.ownedCalendarPath(commit.Owner)
	if path == "" {
		return fmt.Errorf("unknown app calendar")
	}
	outputs := commit.OutputState
	if outputs == nil {
		outputs = s.outputSnapshot()
	}
	var indexErr error
	if err := func() error {
		s.mu.Lock()
		defer s.mu.Unlock()

		old, readErr := os.ReadFile(path)
		had := readErr == nil
		if readErr != nil && !os.IsNotExist(readErr) {
			return readErr
		}
		if commit.Enabled {
			if err := WriteICSFile(path, commit.Name, commit.Events); err != nil {
				return err
			}
		} else if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		if commit.Save != nil {
			if err := commit.Save(); err != nil {
				if restoreErr := restoreFeed(path, old, had); restoreErr != nil {
					return fmt.Errorf("save %s: %w (calendar rollback: %v)", commit.Owner, err, restoreErr)
				}
				return fmt.Errorf("save %s: %w", commit.Owner, err)
			}
		}
		indexErr = s.generateManifestLocked(outputs)
		return nil
	}(); err != nil {
		return err
	}
	// Calendar callbacks run only after the Calendar transaction unlocks.
	if indexErr != nil {
		if s.indexWarning != nil {
			s.indexWarning(commit.Owner, indexErr)
		}
		return nil
	}
	if s.refreshCacheAsync != nil {
		s.refreshCacheAsync()
	}
	return nil
}

func restoreFeed(path string, old []byte, existed bool) error {
	if existed {
		return fileio.WriteAtomic(path, old, 0644)
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// EscapeICS exposes the stable generated-ICS text escaping used by demo feeds
// and Calendar-owned writers without duplicating formatting logic in core.
func EscapeICS(value string) string { return icsEsc(value) }
