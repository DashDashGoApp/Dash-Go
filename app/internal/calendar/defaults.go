package calendar

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var reDefaultCalendarKey = regexp.MustCompile(`^[A-Z0-9_]+$`)

func (s *Service) DefaultConfig() map[string]string {
	values := map[string]string{
		"DEFAULT_US_HOLIDAYS": "1", "HOLIDAY_COUNTRY": "usa", "HOLIDAY_RELIGIONS": "",
		"DEFAULT_MOON_PHASES": "0", "DEFAULT_SEASONS": "0", "DEFAULT_DST_CHANGES": "0", "DEFAULT_ISO_WEEKS": "0",
		"DEFAULT_METEOR_SHOWERS": "0", "DEFAULT_SUPERMOONS": "0", "DEFAULT_ECLIPSES": "0",
		"TRASH_WEEKDAY": "", "RECYCLING_WEEKDAY": "", "RECYCLING_EVERY_WEEKS": "2", "PICKUP_HOLIDAY_SHIFT": "0",
		"PICKUP_SHIFT": "forward", "PICKUP_SHIFT_DAYS": "1", "PAYDAY_MODE": "", "PAYDAY_START": "", "PAYDAY_DAY": "1",
		"DEFAULT_ISS_PASSES": "0", "ISS_N2YO_API_KEY": "", "ISS_LOOKAHEAD_DAYS": "7", "ISS_MIN_VISIBILITY": "180",
	}
	body, err := os.ReadFile(filepath.Join(s.homeDir, ".dashboard-default-calendars"))
	if err != nil {
		return values
	}
	for _, raw := range strings.Split(string(body), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		key := strings.TrimSpace(parts[0])
		if !reDefaultCalendarKey.MatchString(key) {
			continue
		}
		values[key] = strings.Trim(strings.TrimSpace(parts[1]), `"'`)
	}
	return values
}

// GenerateDefaults keeps default feed names, horizon, and output semantics
// unchanged. Weather-generated Moon/Sky calendars and the settings migration
// are called only through narrow core ports.
func (s *Service) GenerateDefaults(refreshIndexes bool) (map[string]any, error) {
	if s == nil {
		return map[string]any{"ok": true, "generator": "go"}, nil
	}
	outputs := s.outputSnapshot()
	s.mu.Lock()
	if err := os.MkdirAll(s.calendarDir, 0755); err != nil {
		s.mu.Unlock()
		return nil, err
	}
	values := s.DefaultConfig()
	today := s.now()
	years := []int{today.Year(), today.Year() + 1, today.Year() + 2}
	rangeStart, rangeEnd := DateOnly(today.Year(), 1, 1), DateOnly(today.Year()+3, 1, 1)
	written, removed := []string{}, []string{}
	var writeErr error
	write := func(file, name string, events []Event) {
		if writeErr != nil {
			return
		}
		if len(events) == 0 {
			if err := os.Remove(filepath.Join(s.calendarDir, file)); err == nil {
				removed = append(removed, file)
			} else if !os.IsNotExist(err) {
				writeErr = err
			}
			return
		}
		if err := WriteICSFile(filepath.Join(s.calendarDir, file), name, events); err != nil {
			writeErr = err
			return
		}
		written = append(written, file)
	}
	remove := func(file string) {
		if RemoveFile(filepath.Join(s.calendarDir, file)) {
			removed = append(removed, file)
		}
	}

	if values["DEFAULT_MOON_PHASES"] == "1" {
		moon := map[string]any{}
		if s.generateMoon != nil {
			moon = s.generateMoon(true)
		}
		if ok, _ := moon["ok"].(bool); ok {
			written = append(written, "moon.violet.ics")
		} else {
			detail := strings.TrimSpace(fmt.Sprint(moon["error"]))
			if detail == "" || detail == "<nil>" {
				detail = "moon generator unavailable"
			}
			s.appendLog("calendar-defaults.log", fmt.Sprintf("%s: moon phases were not generated (%s)\n", s.now().Format(time.ANSIC), detail))
		}
	} else {
		remove("moon.violet.ics")
		// Retire the historical incorrect filename if it exists from an older build.
		remove("moon.slate.ics")
	}
	if values["DEFAULT_SEASONS"] == "1" {
		write("seasons.gold.holiday.ics", "Seasons", SeasonEvents(years))
	} else {
		remove("seasons.gold.holiday.ics")
	}
	if values["DEFAULT_DST_CHANGES"] == "1" {
		write("dst.slate.holiday.ics", "Daylight Saving Time", DSTEvents(years))
	} else {
		remove("dst.slate.holiday.ics")
	}
	remove("weeks.slate.ics")
	if values["DEFAULT_ISO_WEEKS"] == "1" && s.enableISOWeek != nil {
		s.enableISOWeek()
	}
	// Migrate the old installer-only pickup/payday variables on first use. The
	// legacy values stay intact as a safe fallback; the structured config then
	// becomes the Dashboard Control source of truth.
	household, migrated, householdErr := s.loadHouseholdSchedulesLocked(values)
	if householdErr != nil {
		s.mu.Unlock()
		return nil, householdErr
	}
	if migrated {
		if err := s.writeHouseholdSchedulesLocked(household); err != nil {
			s.mu.Unlock()
			return nil, err
		}
	}
	feeds := householdScheduleFeeds(household, rangeStart, rangeEnd, s.HolidayDatesByLayer(nextHolidayLayers(household)))
	write("trash.amber.ics", "Trash Pickup", feeds["trash"])
	write("recycling.teal.ics", "Recycling Pickup", feeds["recycling"])
	write("payday.violet.pay.ics", "Paydays", feeds["payday"])
	celebrations, celebrationNotes := celebrationICSEvents(s.celebrationsFile, years, rangeStart, rangeEnd)
	for _, note := range celebrationNotes {
		s.appendLog("calendar-defaults.log", fmt.Sprintf("%s: celebration %s\n", s.now().Format(time.ANSIC), note))
	}
	write("celebrations.gold.ics", "Celebrations", celebrations)
	if writeErr != nil {
		s.mu.Unlock()
		return nil, writeErr
	}
	sky := map[string]any{}
	if s.generateSky != nil {
		sky = s.generateSky()
	}
	if refreshIndexes {
		_ = s.generateManifestLocked(outputs)
	}
	s.mu.Unlock()
	if refreshIndexes && s.refreshCacheSync != nil {
		_ = s.refreshCacheSync()
	}
	return map[string]any{"ok": true, "generator": "go", "written": written, "removed": removed, "sky": sky}, nil
}

func DSTEvents(years []int) []Event {
	location, err := time.LoadLocation(LocalTimezoneName())
	if err != nil {
		return nil
	}
	out := []Event{}
	for _, year := range years {
		var previous int
		havePrevious := false
		for day := DateOnly(year, 1, 1); day.Year() == year; day = day.AddDate(0, 0, 1) {
			_, offset := time.Date(day.Year(), day.Month(), day.Day(), 12, 0, 0, 0, location).Zone()
			if havePrevious && offset != previous {
				title := "Daylight Saving Time ends"
				if offset > previous {
					title = "Daylight Saving Time begins"
				}
				out = append(out, Event{Date: day, Summary: title, UID: "dst"})
			}
			previous, havePrevious = offset, true
		}
	}
	return out
}
