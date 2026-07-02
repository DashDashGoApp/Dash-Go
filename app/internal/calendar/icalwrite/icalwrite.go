// Package icalwrite authors and safely edits vdir-style iCalendar items for
// Dash-Go calendar write-back. It deliberately owns only simple single-event
// fields; all provider-specific lines remain untouched during edits.
package icalwrite

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Event struct {
	UID      string
	Title    string
	Desc     string
	Location string
	Start    time.Time
	End      time.Time
	AllDay   bool
}

const (
	prodID       = "-//Dash-Go//calendar-writeback//EN"
	maxTextLen   = 2000
	maxTitleLen  = 300
	maxOctetLine = 75
)

func NewUID(now time.Time) string {
	buf := make([]byte, 9)
	_, _ = rand.Read(buf)
	return fmt.Sprintf("dashgo-%s-%s", now.UTC().Format("20060102T150405Z"), hex.EncodeToString(buf))
}

func (e Event) Validate() error {
	title := strings.TrimSpace(e.Title)
	switch {
	case strings.TrimSpace(e.UID) == "":
		return errors.New("event UID required")
	case strings.ContainsAny(e.UID, "/\\\r\n\" '"):
		return errors.New("event UID contains unsafe characters")
	case title == "":
		return errors.New("event title required")
	case len(title) > maxTitleLen:
		return errors.New("event title too long")
	case len(e.Desc) > maxTextLen || len(e.Location) > maxTextLen:
		return errors.New("event text too long")
	case e.Start.IsZero():
		return errors.New("event start required")
	case e.End.IsZero() || !e.End.After(e.Start):
		if e.AllDay {
			return errors.New("all-day events need an exclusive end date after the start")
		}
		return errors.New("event end must be after start")
	}
	return nil
}

func (e Event) Serialize(now time.Time) (string, error) {
	if err := e.Validate(); err != nil {
		return "", err
	}
	stamp := utcStamp(now)
	lines := []string{
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:" + prodID, "CALSCALE:GREGORIAN",
		"BEGIN:VEVENT", prop("UID", e.UID), prop("DTSTAMP", stamp), prop("CREATED", stamp),
		prop("LAST-MODIFIED", stamp), "SEQUENCE:0",
	}
	if e.AllDay {
		lines = append(lines, prop("DTSTART;VALUE=DATE", dateStamp(e.Start)), prop("DTEND;VALUE=DATE", dateStamp(e.End)))
	} else {
		lines = append(lines, prop("DTSTART", utcStamp(e.Start)), prop("DTEND", utcStamp(e.End)))
	}
	lines = append(lines, prop("SUMMARY", escapeText(strings.TrimSpace(e.Title))))
	if strings.TrimSpace(e.Desc) != "" {
		lines = append(lines, prop("DESCRIPTION", escapeText(e.Desc)))
	}
	if strings.TrimSpace(e.Location) != "" {
		lines = append(lines, prop("LOCATION", escapeText(e.Location)))
	}
	lines = append(lines, "END:VEVENT", "END:VCALENDAR")
	return refold(lines), nil
}

func utcStamp(t time.Time) string    { return t.UTC().Format("20060102T150405Z") }
func dateStamp(t time.Time) string   { return t.Format("20060102") }
func prop(name, value string) string { return foldLine(name + ":" + value) }

func escapeText(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case ';':
			b.WriteString(`\;`)
		case ',':
			b.WriteString(`\,`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func foldLine(line string) string {
	if len(line) <= maxOctetLine {
		return line
	}
	var b strings.Builder
	budget, count := maxOctetLine, 0
	for _, r := range line {
		size := len(string(r))
		if count+size > budget {
			b.WriteString("\r\n ")
			count, budget = 0, maxOctetLine-1
		}
		b.WriteRune(r)
		count += size
	}
	return b.String()
}

func refold(lines []string) string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, foldLine(line))
	}
	return strings.Join(out, "\r\n") + "\r\n"
}
