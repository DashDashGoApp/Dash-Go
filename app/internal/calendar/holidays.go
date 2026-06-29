package calendar

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
)

var reHolidayDate = regexp.MustCompile(`^DTSTART(?:;[^:]*)?:(\d{8})`)

func (s *Service) CivilHolidayDates() map[string]bool {
	out := map[string]bool{}
	body, err := os.ReadFile(filepath.Join(s.calendarDir, "holidays.blue.holiday.ics"))
	if err != nil {
		return out
	}
	for _, raw := range strings.Split(string(body), "\n") {
		if matches := reHolidayDate.FindStringSubmatch(strings.TrimSpace(raw)); len(matches) == 2 {
			out[matches[1]] = true
		}
	}
	return out
}

func (s *Service) UpdateHolidays() map[string]any {
	if s == nil {
		return map[string]any{"ok": true, "changed": 0, "errors": []string{}, "generator": "go"}
	}
	s.mu.Lock()
	values := s.DefaultConfig()
	changed, errorsOut := 0, []string{}
	civilDest := filepath.Join(s.calendarDir, "holidays.blue.holiday.ics")
	if values["DEFAULT_US_HOLIDAYS"] == "1" {
		if ok, err := s.fetchFirstCalendarLocked(civilDest, "civil holidays ("+values["HOLIDAY_COUNTRY"]+")", CountryCalendarIDs(values["HOLIDAY_COUNTRY"])); ok {
			changed++
		} else if err != nil {
			errorsOut = append(errorsOut, err.Error())
		}
	} else {
		if RemoveFile(civilDest) {
			changed++
		}
		s.appendLog("holidays.log", fmt.Sprintf("%s: civil holidays disabled by %s\n", s.now().Format(time.ANSIC), filepath.Join(s.homeDir, ".dashboard-default-calendars")))
	}
	selected := map[string]bool{}
	for _, religion := range strings.Fields(strings.NewReplacer(",", " ", ";", " ").Replace(strings.ToLower(values["HOLIDAY_RELIGIONS"]))) {
		selected[religion] = true
	}
	known := []string{"jewish-holidays.violet.holiday.ics", "islamic-holidays.teal.holiday.ics", "christian-holidays.gold.holiday.ics", "orthodox-holidays.slate.holiday.ics", "hindu-holidays.amber.holiday.ics"}
	keep := map[string]bool{}
	for religion := range selected {
		file, label, ids, ok := ReligionCalendarSpec(religion)
		if !ok {
			s.appendLog("holidays.log", fmt.Sprintf("%s: unknown observance layer ignored: %s\n", s.now().Format(time.ANSIC), religion))
			continue
		}
		keep[file] = true
		if ok, err := s.fetchFirstCalendarLocked(filepath.Join(s.calendarDir, file), label, ids); ok {
			changed++
		} else if err != nil {
			errorsOut = append(errorsOut, err.Error())
		}
	}
	for _, file := range known {
		if !keep[file] {
			RemoveFile(filepath.Join(s.calendarDir, file))
		}
	}
	s.mu.Unlock()
	return map[string]any{"ok": len(errorsOut) == 0, "changed": changed, "errors": errorsOut, "generator": "go"}
}

func (s *Service) fetchFirstCalendarLocked(destination, label string, ids []string) (bool, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	if s.httpClient != nil && s.httpClient() != nil {
		client = s.httpClient()
	}
	var lastErr error
	for _, id := range ids {
		url := "https://calendar.google.com/calendar/ical/" + id + "%40group.v.calendar.google.com/public/basic.ics"
		response, err := client.Get(url)
		if err != nil {
			lastErr = err
			continue
		}
		body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
		response.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		if response.StatusCode < 200 || response.StatusCode > 299 || !BytesHasCalendar(body) {
			lastErr = fmt.Errorf("%s fetch failed from %s: HTTP %d", label, id, response.StatusCode)
			continue
		}
		if err := fileio.WriteAtomic(destination, body, 0644); err != nil {
			return false, err
		}
		s.appendLog("holidays.log", fmt.Sprintf("%s: %s updated from %s\n", s.now().Format(time.ANSIC), label, id))
		return true, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("%s fetch failed", label)
	}
	s.appendLog("holidays.log", fmt.Sprintf("%s: %s FAILED, kept previous file (%v)\n", s.now().Format(time.ANSIC), label, lastErr))
	return false, lastErr
}

func BytesHasCalendar(body []byte) bool {
	limit := len(body)
	if limit > 1024 {
		limit = 1024
	}
	return strings.Contains(string(body[:limit]), "BEGIN:VCALENDAR")
}

func CountryCalendarIDs(country string) []string {
	switch strings.NewReplacer(" ", "", "_", "", "-", "").Replace(strings.ToLower(country)) {
	case "uk", "gb", "unitedkingdom":
		return []string{"en.uk%23holiday", "en.uk.official%23holiday"}
	case "canada", "ca":
		return []string{"en.canadian%23holiday", "en.canadian.official%23holiday"}
	case "australia", "au":
		return []string{"en.australian%23holiday", "en.australian.official%23holiday"}
	case "germany", "de":
		return []string{"de.german%23holiday", "en.german%23holiday", "en.german.official%23holiday"}
	case "france", "fr":
		return []string{"fr.french%23holiday", "en.french%23holiday", "en.french.official%23holiday"}
	case "spain", "es":
		return []string{"en.spain%23holiday", "en.spain.official%23holiday"}
	case "italy", "it":
		return []string{"en.italian%23holiday", "en.italian.official%23holiday"}
	case "japan", "jp":
		return []string{"en.japanese%23holiday"}
	case "netherlands", "nl":
		return []string{"en.dutch%23holiday", "en.dutch.official%23holiday"}
	case "newzealand", "nz":
		return []string{"en.new_zealand%23holiday", "en.new_zealand.official%23holiday"}
	case "mexico", "mx":
		return []string{"en.mexican%23holiday", "en.mexican.official%23holiday"}
	default:
		return []string{"en.usa%23holiday", "en.usa.official%23holiday"}
	}
}

func ReligionCalendarSpec(religion string) (string, string, []string, bool) {
	switch strings.NewReplacer(" ", "", "_", "", "-", "").Replace(strings.ToLower(religion)) {
	case "jewish", "judaism":
		return "jewish-holidays.violet.holiday.ics", "Jewish holidays", []string{"en.judaism%23holiday", "iw.jewish%23holiday", "en.jewish%23holiday"}, true
	case "islamic", "islam", "muslim":
		return "islamic-holidays.teal.holiday.ics", "Islamic holidays", []string{"en.islamic%23holiday"}, true
	case "christian", "christianity":
		return "christian-holidays.gold.holiday.ics", "Christian holidays", []string{"en.christian%23holiday"}, true
	case "orthodox", "orthodoxchristian":
		return "orthodox-holidays.slate.holiday.ics", "Orthodox holidays", []string{"en.orthodox_christian%23holiday"}, true
	case "hindu", "hinduism":
		return "hindu-holidays.amber.holiday.ics", "Hindu holidays", []string{"en.hinduism%23holiday"}, true
	default:
		return "", "", nil, false
	}
}
