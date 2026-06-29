package weather

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

const synodicDays = 29.530588853

var baseNewMoonUTC = time.Date(2024, 1, 11, 11, 57, 0, 0, time.UTC)

type calendarEvent struct {
	Date        time.Time
	Summary     string
	UID         string
	Description string
}

func icsEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `;`, `\;`)
	s = strings.ReplaceAll(s, `,`, `\,`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}

func dateYMD(t time.Time) string { return t.Format("20060102") }

func atomicWriteText(path string, text string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(text), 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (s *Service) defaultCalendarFlags() map[string]string {
	vals := map[string]string{
		"DEFAULT_MOON_PHASES":    "0",
		"DEFAULT_METEOR_SHOWERS": "0",
		"DEFAULT_SUPERMOONS":     "0",
		"DEFAULT_ECLIPSES":       "0",
	}
	b, err := os.ReadFile(filepath.Join(s.home, ".dashboard-default-calendars"))
	if err != nil {
		return vals
	}
	re := reDefaultCalendarKey
	for line := range strings.SplitSeq(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		key := strings.TrimSpace(parts[0])
		if !re.MatchString(key) {
			continue
		}
		val := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
		vals[key] = val
	}
	return vals
}

func (s *Service) readDashboardLocation() map[string]any {
	out := map[string]any{"lat": nil, "lon": nil, "city": ""}
	b, err := os.ReadFile(s.configLocal)
	if err != nil {
		return out
	}
	txt := string(b)
	if m := reLocationName.FindStringSubmatch(txt); len(m) > 1 {
		out["city"] = m[1]
	}
	if m := reSkyLatitude.FindStringSubmatch(txt); len(m) > 1 {
		if f, err := strconv.ParseFloat(m[1], 64); err == nil {
			out["lat"] = f
		}
	}
	if m := reSkyLongitude.FindStringSubmatch(txt); len(m) > 1 {
		if f, err := strconv.ParseFloat(m[1], 64); err == nil {
			out["lon"] = f
		}
	}
	return out
}

func systemTimezoneName() string {
	if out, err := exec.Command("timedatectl", "show", "-p", "Timezone", "--value").Output(); err == nil {
		if s := strings.TrimSpace(string(out)); s != "" {
			return s
		}
	}
	if b, err := os.ReadFile("/etc/timezone"); err == nil {
		if s := strings.TrimSpace(string(b)); s != "" {
			return s
		}
	}
	return ""
}

func localMoonDate(utc time.Time, lon any) (time.Time, string) {
	if tz := systemTimezoneName(); tz != "" {
		if loc, err := time.LoadLocation(tz); err == nil {
			y, m, d := utc.In(loc).Date()
			return time.Date(y, m, d, 0, 0, 0, 0, time.UTC), tz
		}
	}
	var lf float64
	switch v := lon.(type) {
	case float64:
		lf = v
	case int:
		lf = float64(v)
	case string:
		lf, _ = strconv.ParseFloat(v, 64)
	}
	off := 0.0
	if !math.IsNaN(lf) {
		off = max(-12.0, min(14.0, lf/15.0))
	}
	local := utc.Add(time.Duration(off * float64(time.Hour)))
	y, m, d := local.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC), fmt.Sprintf("longitude-offset:%+.2fh", off)
}

func writeICalendar(path, name string, events []calendarEvent, extra []string) error {
	slices.SortFunc(events, func(a, b calendarEvent) int { return a.Date.Compare(b.Date) })
	now := time.Now().UTC().Format("20060102T150405Z")
	lines := []string{"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//Dash-Go//Go Calendar Generator//EN", "X-WR-CALNAME:" + icsEscape(name)}
	lines = append(lines, extra...)
	for i, ev := range events {
		start := ev.Date
		end := start.AddDate(0, 0, 1)
		uidPrefix := strings.TrimSpace(ev.UID)
		if uidPrefix == "" {
			uidPrefix = "event"
		}
		lines = append(lines,
			"BEGIN:VEVENT",
			fmt.Sprintf("UID:%s-%s-%d@dash-go", uidPrefix, dateYMD(start), i+1),
			"DTSTAMP:"+now,
			"DTSTART;VALUE=DATE:"+dateYMD(start),
			"DTEND;VALUE=DATE:"+dateYMD(end),
			"SUMMARY:"+icsEscape(ev.Summary),
		)
		if strings.TrimSpace(ev.Description) != "" {
			lines = append(lines, "DESCRIPTION:"+icsEscape(ev.Description))
		}
		lines = append(lines, "TRANSP:TRANSPARENT", "END:VEVENT")
	}
	lines = append(lines, "END:VCALENDAR")
	return atomicWriteText(path, strings.Join(lines, "\r\n")+"\r\n")
}

func (s *Service) moonStatusFile() string {
	return filepath.Join(s.cacheDir, "moon-calendar-status.json")
}

func (s *Service) generateMoonCalendar(force bool) map[string]any {
	flags := s.defaultCalendarFlags()
	enabled := flags["DEFAULT_MOON_PHASES"] == "1"
	path := filepath.Join(s.calDir, "moon.violet.ics")
	if !enabled && !force {
		_ = os.Remove(path)
		meta := map[string]any{"ok": true, "enabled": false, "file": path, "eventCount": 0, "updatedAt": time.Now().Unix(), "generator": "go"}
		_ = fileio.WriteJSON(s.moonStatusFile(), meta)
		return meta
	}
	loc := s.readDashboardLocation()
	thisYear := time.Now().Year()
	startUTC := time.Date(thisYear, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, -35)
	endUTC := time.Date(thisYear+3, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, 35)
	n := int(startUTC.Sub(baseNewMoonUTC).Hours()/24.0/synodicDays) - 2
	lo := time.Date(thisYear, 1, 1, 0, 0, 0, 0, time.UTC)
	hi := time.Date(thisYear+3, 1, 1, 0, 0, 0, 0, time.UTC)
	phases := []struct {
		off   float64
		title string
		uid   string
	}{
		{0.0, "New Moon", "newmoon"},
		{synodicDays / 4.0, "First Quarter Moon", "firstquarter"},
		{synodicDays / 2.0, "Full Moon", "fullmoon"},
		{3.0 * synodicDays / 4.0, "Last Quarter Moon", "lastquarter"},
	}
	events := []calendarEvent{}
	tzUsed := ""
	for {
		newMoon := baseNewMoonUTC.Add(time.Duration(float64(n) * synodicDays * float64(24*time.Hour)))
		if newMoon.After(endUTC) {
			break
		}
		for _, phase := range phases {
			phaseUTC := newMoon.Add(time.Duration(phase.off * float64(24*time.Hour)))
			localDay, tzLabel := localMoonDate(phaseUTC, loc["lon"])
			if tzUsed == "" {
				tzUsed = tzLabel
			}
			if !localDay.Before(lo) && localDay.Before(hi) {
				city := jsonutil.StringValue(loc["city"])
				if city == "" {
					city = "dashboard location"
				}
				events = append(events, calendarEvent{Date: localDay, Summary: phase.title, UID: phase.uid, Description: fmt.Sprintf("Moon phase date localized for %s using %s.", city, tzLabel)})
			}
		}
		n++
	}
	extra := []string{}
	city := jsonutil.StringValue(loc["city"])
	coord := ""
	lat, latOK := loc["lat"].(float64)
	lon, lonOK := loc["lon"].(float64)
	if latOK && lonOK {
		coord = fmt.Sprintf("%.4f,%.4f", lat, lon)
	}
	extra = append(extra, "X-FD-MOON-LOCATION:"+icsEscape(city), "X-FD-MOON-COORD:"+icsEscape(coord))
	if err := writeICalendar(path, "Moon Phases", events, extra); err != nil {
		return map[string]any{"ok": false, "enabled": enabled || force, "error": err.Error(), "file": path, "generator": "go"}
	}
	meta := map[string]any{"ok": true, "enabled": enabled || force, "file": path, "eventCount": len(events), "lat": loc["lat"], "lon": loc["lon"], "city": city, "timezone": tzUsed, "updatedAt": time.Now().Unix(), "yearsAhead": 3, "generator": "go"}
	_ = fileio.WriteJSON(s.moonStatusFile(), meta)
	return meta
}

func (s *Service) moonCalendarStatus() map[string]any {
	flags := s.defaultCalendarFlags()
	enabled := flags["DEFAULT_MOON_PHASES"] == "1"
	loc := s.readDashboardLocation()
	meta := map[string]any{}
	if raw := s.readJSONDefault(s.moonStatusFile(), map[string]any{}); raw != nil {
		if m, ok := raw.(map[string]any); ok {
			for k, v := range m {
				meta[k] = v
			}
		}
	}
	stale := false
	if enabled {
		if fmt.Sprint(meta["lat"]) != fmt.Sprint(loc["lat"]) || fmt.Sprint(meta["lon"]) != fmt.Sprint(loc["lon"]) {
			stale = true
		}
	}
	meta["ok"] = true
	meta["enabled"] = enabled
	meta["stale"] = stale
	meta["lat"] = loc["lat"]
	meta["lon"] = loc["lon"]
	meta["city"] = loc["city"]
	meta["generator"] = "go"
	return meta
}

func parseMMDD(year int, md string) time.Time {
	parts := strings.Split(md, "-")
	if len(parts) != 2 {
		return time.Time{}
	}
	m, _ := strconv.Atoi(parts[0])
	d, _ := strconv.Atoi(parts[1])
	return time.Date(year, time.Month(m), d, 0, 0, 0, 0, time.UTC)
}

func (s *Service) removeCalendar(name string) { _ = os.Remove(filepath.Join(s.calDir, name)) }

func (s *Service) generateStaticSkyCalendars() map[string]any {
	flags := s.defaultCalendarFlags()
	thisYear := time.Now().Year()
	lo := time.Date(thisYear, 1, 1, 0, 0, 0, 0, time.UTC)
	hi := time.Date(thisYear+3, 1, 1, 0, 0, 0, 0, time.UTC)
	counts := map[string]int{"meteorShowers": 0, "supermoons": 0, "eclipses": 0}

	if flags["DEFAULT_METEOR_SHOWERS"] == "1" {
		rows := []struct{ md, title, uid, desc string }{
			{"01-03", "Quadrantids meteor shower peak", "quadrantids", "Strong annual shower; best after midnight from dark skies."},
			{"04-22", "Lyrids meteor shower peak", "lyrids", "Spring shower; best before dawn from dark skies."},
			{"05-06", "Eta Aquariids meteor shower peak", "eta-aquariids", "Halley debris stream; often better from lower latitudes."},
			{"07-30", "Southern Delta Aquariids meteor shower peak", "delta-aquariids", "Best after midnight; modest but reliable."},
			{"07-30", "Alpha Capricornids meteor shower peak", "alpha-capricornids", "Usually low rates, but can produce bright fireballs."},
			{"08-12", "Perseids meteor shower peak", "perseids", "Popular summer shower; best late night through dawn."},
			{"10-08", "Draconids meteor shower peak", "draconids", "Evening-friendly shower; rates vary by year."},
			{"10-21", "Orionids meteor shower peak", "orionids", "Halley debris stream; best after midnight."},
			{"11-05", "Southern Taurids meteor shower peak", "southern-taurids", "Slow meteors; occasional fireballs."},
			{"11-12", "Northern Taurids meteor shower peak", "northern-taurids", "Slow meteors; occasional fireballs."},
			{"11-17", "Leonids meteor shower peak", "leonids", "Rates vary by year; best before dawn."},
			{"12-14", "Geminids meteor shower peak", "geminids", "Often one of the strongest annual showers."},
			{"12-22", "Ursids meteor shower peak", "ursids", "Modest northern shower near the December solstice."},
		}
		events := []calendarEvent{}
		for y := thisYear; y < thisYear+3; y++ {
			for _, row := range rows {
				day := parseMMDD(y, row.md)
				if !day.Before(lo) && day.Before(hi) {
					events = append(events, calendarEvent{Date: day, Summary: row.title, UID: row.uid, Description: row.desc})
				}
			}
		}
		_ = writeICalendar(filepath.Join(s.calDir, "meteor-showers.indigo.sky.ics"), "Meteor Showers", events, nil)
		counts["meteorShowers"] = len(events)
	} else {
		s.removeCalendar("meteor-showers.indigo.sky.ics")
	}

	if flags["DEFAULT_SUPERMOONS"] == "1" {
		rows := map[int][][2]string{
			2026: {{"01-03", "Wolf Supermoon"}, {"11-24", "Beaver Supermoon"}, {"12-24", "Closest Full Supermoon"}},
			2027: {{"01-22", "Closest Full Supermoon"}, {"02-20", "Snow Supermoon"}},
			2028: {{"01-12", "Wolf Supermoon"}, {"02-10", "Closest Full Supermoon"}, {"03-11", "Worm Supermoon"}, {"04-09", "Pink Supermoon"}},
			2029: {{"01-30", "Wolf Supermoon"}, {"02-28", "Snow Supermoon"}, {"03-30", "Closest Full Supermoon"}, {"04-28", "Pink Supermoon"}, {"05-27", "Flower Supermoon"}},
		}
		events := []calendarEvent{}
		for y, set := range rows {
			for _, row := range set {
				day := parseMMDD(y, row[0])
				if !day.Before(lo) && day.Before(hi) {
					events = append(events, calendarEvent{Date: day, Summary: row[1], UID: "supermoon", Description: "Full Moon near perigee. Local date may vary by time zone."})
				}
			}
		}
		_ = writeICalendar(filepath.Join(s.calDir, "supermoons.violet.sky.ics"), "Supermoons", events, nil)
		counts["supermoons"] = len(events)
	} else {
		s.removeCalendar("supermoons.violet.sky.ics")
	}

	if flags["DEFAULT_ECLIPSES"] == "1" {
		rows := []struct{ date, title, uid, desc string }{
			{"2026-02-17", "Annular solar eclipse", "solar-eclipse", "Global event; local visibility varies. Use proper solar eye protection."},
			{"2026-03-03", "Total lunar eclipse", "lunar-eclipse", "Global event; local visibility varies."},
			{"2026-08-12", "Total solar eclipse", "solar-eclipse", "Global event; local visibility varies. Use proper solar eye protection."},
			{"2026-08-28", "Partial lunar eclipse", "lunar-eclipse", "Global event; local visibility varies."},
			{"2027-02-06", "Annular solar eclipse", "solar-eclipse", "Global event; local visibility varies. Use proper solar eye protection."},
			{"2027-02-21", "Penumbral lunar eclipse", "lunar-eclipse", "Global event; local visibility varies."},
			{"2027-07-18", "Penumbral lunar eclipse", "lunar-eclipse", "Global event; local visibility varies."},
			{"2027-08-02", "Total solar eclipse", "solar-eclipse", "Global event; local visibility varies. Use proper solar eye protection."},
			{"2027-08-17", "Penumbral lunar eclipse", "lunar-eclipse", "Global event; local visibility varies."},
			{"2028-01-12", "Partial lunar eclipse", "lunar-eclipse", "Global event; local visibility varies."},
			{"2028-01-26", "Annular solar eclipse", "solar-eclipse", "Global event; local visibility varies. Use proper solar eye protection."},
			{"2028-07-06", "Partial lunar eclipse", "lunar-eclipse", "Global event; local visibility varies."},
			{"2028-07-22", "Total solar eclipse", "solar-eclipse", "Global event; local visibility varies. Use proper solar eye protection."},
			{"2028-12-31", "Total lunar eclipse", "lunar-eclipse", "Global event; local visibility varies."},
		}
		events := []calendarEvent{}
		for _, row := range rows {
			day, err := time.Parse("2006-01-02", row.date)
			if err == nil && !day.Before(lo) && day.Before(hi) {
				events = append(events, calendarEvent{Date: day, Summary: row.title, UID: row.uid, Description: row.desc})
			}
		}
		_ = writeICalendar(filepath.Join(s.calDir, "eclipses.slate.sky.ics"), "Eclipses", events, nil)
		counts["eclipses"] = len(events)
	} else {
		s.removeCalendar("eclipses.slate.sky.ics")
	}
	return map[string]any{"ok": true, "generator": "go", "counts": counts}
}

func (s *Service) writeSkyStatus(meta map[string]any) {
	_ = os.MkdirAll(s.cacheDir, 0755)
	b, _ := json.MarshalIndent(meta, "", "  ")
	_ = os.WriteFile(filepath.Join(s.cacheDir, "sky-calendar-status.json"), append(b, '\n'), 0644)
}
