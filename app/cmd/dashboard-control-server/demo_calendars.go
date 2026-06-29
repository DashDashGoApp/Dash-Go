package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
)

func (a *app) writeDemoCalendars(now time.Time) error {
	loc, err := time.LoadLocation("America/Chicago")
	if err != nil {
		loc = time.Local
	}
	today := demoDateOnly(now.In(loc))
	baseMon := demoMonday(today)
	dense := today.AddDate(0, 0, 1)
	weekend := demoNextWeekday(today, time.Friday, 2)
	retreat := demoNextWeekday(today, time.Tuesday, 7)
	trip := demoNextWeekday(today, time.Friday, 21)
	schoolBreak := demoNextWeekday(today, time.Monday, 28)
	paydayStart := demoNextWeekday(today.AddDate(0, 0, -14), time.Friday, 0)

	family := []demoEvent{
		{UID: "family-dinner", Title: "Family dinner at Navy Pier", Start: demoAt(today, "18:30", loc), End: demoAt(today, "19:45", loc), Location: "Navy Pier, 600 E Grand Ave, Chicago, IL"},
		{UID: "annual-checkup", Title: "Avery and Jordan annual checkup", Start: demoAt(dense, "15:00", loc), End: demoAt(dense, "16:00", loc), Location: "Northwestern Medicine, Chicago, IL", Desc: "Demo appointment with location and overlap."},
		{UID: "soccer-practice", Title: "Soccer practice", Start: demoAt(dense, "15:30", loc), End: demoAt(dense, "16:30", loc), Location: "Grant Park, Chicago, IL"},
		{UID: "museum-day", Title: "Museum of Science and Industry visit", Start: demoAt(today.AddDate(0, 0, 5), "10:00", loc), End: demoAt(today.AddDate(0, 0, 5), "13:00", loc), Location: "Museum of Science and Industry, 5700 S DuSable Lake Shore Dr, Chicago, IL"},
		{UID: "birthday-avery", Title: "Birthday - Avery", Start: today.AddDate(0, 0, 9), AllDay: true},
		{UID: "camping-weekend", Title: "Camping Weekend", Start: demoAt(weekend, "17:00", loc), End: demoAt(weekend.AddDate(0, 0, 2), "11:00", loc), Location: "Indiana Dunes National Park"},
		{UID: "road-trip", Title: "Family Road Trip", Start: demoAt(trip, "18:00", loc), End: demoAt(trip.AddDate(0, 0, 4), "20:00", loc), Location: "Chicago to Door County", Desc: "Demo multi-week event crossing week rows."},
		{UID: "lunch-grandma", Title: "Lunch with Grandma", Start: demoAt(today.AddDate(0, 0, 3), "12:00", loc), End: demoAt(today.AddDate(0, 0, 3), "13:00", loc), Location: "Lou Mitchell's, 565 W Jackson Blvd, Chicago, IL"},
		{UID: "zoo-morning", Title: "Lincoln Park Zoo morning", Start: demoAt(today.AddDate(0, 0, 11), "09:30", loc), End: demoAt(today.AddDate(0, 0, 11), "11:30", loc), Location: "Lincoln Park Zoo, 2001 N Clark St, Chicago, IL"},
		{UID: "family-movie", Title: "Family movie night", Start: demoAt(today.AddDate(0, 0, 12), "19:00", loc), End: demoAt(today.AddDate(0, 0, 12), "21:00", loc)},
		{UID: "weekly-farmers", Title: "Farmers market", Start: demoAt(demoNextWeekday(baseMon, time.Saturday, 0), "07:15", loc), End: demoAt(demoNextWeekday(baseMon, time.Saturday, 0), "09:00", loc), Location: "Green City Market, Chicago, IL", RRule: "FREQ=WEEKLY;COUNT=12;BYDAY=SA"},
	}

	work := []demoEvent{
		{UID: "work-alex", Title: "Work - Alex", Start: demoAt(dense, "08:00", loc), End: demoAt(dense, "17:00", loc), Location: "The Loop, Chicago, IL"},
		{UID: "work-taylor", Title: "Work - Taylor", Start: demoAt(dense, "06:00", loc), End: demoAt(dense, "14:30", loc), Location: "West Loop, Chicago, IL"},
		{UID: "standup", Title: "Daily Standup and Priorities Review", Start: demoAt(baseMon, "09:15", loc), End: demoAt(baseMon, "09:45", loc), RRule: "FREQ=WEEKLY;COUNT=40;BYDAY=MO,TU,WE,TH,FR"},
		{UID: "sprint", Title: "Biweekly Sprint Planning and Roadmap Review", Start: demoAt(dense, "10:00", loc), End: demoAt(dense, "12:00", loc), Location: "Tech conference room, 401 N Michigan Ave, Chicago, IL", RRule: "FREQ=WEEKLY;INTERVAL=2;COUNT=8"},
		{UID: "offsite", Title: "Offsite Strategy Retreat", Start: demoAt(retreat, "09:00", loc), End: demoAt(retreat.AddDate(0, 0, 2), "16:00", loc), Location: "Chicago Cultural Center, 78 E Washington St, Chicago, IL"},
		{UID: "travel", Title: "Client travel day", Start: demoAt(today.AddDate(0, 0, 16), "07:00", loc), End: demoAt(today.AddDate(0, 0, 16), "18:30", loc), Location: "Chicago Union Station"},
		{UID: "presentation", Title: "Quarterly product demo", Start: demoAt(today.AddDate(0, 0, 18), "14:00", loc), End: demoAt(today.AddDate(0, 0, 18), "15:00", loc), Desc: "Demo long-title business event."},
	}

	school := []demoEvent{
		{UID: "dropoff", Title: "School drop-off", Start: demoAt(baseMon, "07:30", loc), End: demoAt(baseMon, "07:50", loc), RRule: "FREQ=WEEKLY;COUNT=40;BYDAY=MO,TU,WE,TH,FR"},
		{UID: "bus-pickup", Title: "Bus pickup", Start: demoAt(dense, "08:10", loc), End: demoAt(dense, "08:25", loc), Location: "Home"},
		{UID: "picture-day", Title: "School picture day", Start: today.AddDate(0, 0, 7), AllDay: true},
		{UID: "conference", Title: "Parent Teacher Conference - Room 204 with Ms. Rivera", Start: demoAt(today.AddDate(0, 0, 8), "17:30", loc), End: demoAt(today.AddDate(0, 0, 8), "18:00", loc), Location: "Demo Elementary School, Chicago, IL"},
		{UID: "piano", Title: "Piano lessons", Start: demoAt(demoNextWeekday(today, time.Wednesday, 0), "17:00", loc), End: demoAt(demoNextWeekday(today, time.Wednesday, 0), "17:45", loc), Location: "Old Town School of Folk Music, Chicago, IL", RRule: "FREQ=WEEKLY;COUNT=12;BYDAY=WE"},
		{UID: "field-trip", Title: "Field trip - Adler Planetarium", Start: demoAt(today.AddDate(0, 0, 15), "09:00", loc), End: demoAt(today.AddDate(0, 0, 15), "14:30", loc), Location: "Adler Planetarium, 1300 S DuSable Lake Shore Dr, Chicago, IL"},
		{UID: "no-school", Title: "No school - staff development", Start: today.AddDate(0, 0, 22), AllDay: true},
		{UID: "school-break", Title: "School Break", Start: schoolBreak, End: schoolBreak.AddDate(0, 0, 5), AllDay: true},
	}

	home := []demoEvent{
		{UID: "trash", Title: "Trash pickup", Start: demoNextWeekday(baseMon, time.Monday, 0), AllDay: true, RRule: "FREQ=WEEKLY;COUNT=14;BYDAY=MO"},
		{UID: "payday", Title: "Payday", Start: paydayStart, AllDay: true, RRule: "FREQ=WEEKLY;INTERVAL=2;COUNT=8;BYDAY=FR"},
		{UID: "library", Title: "Monthly library pickup with a very long title to test wrapping", Start: demoAt(today.AddDate(0, 0, 6), "18:30", loc), End: demoAt(today.AddDate(0, 0, 6), "19:15", loc), Location: "Harold Washington Library Center, 400 S State St, Chicago, IL", RRule: "FREQ=MONTHLY;COUNT=4"},
		{UID: "grocery", Title: "Grocery pickup", Start: demoAt(dense, "17:15", loc), End: demoAt(dense, "17:30", loc), Location: "Jewel-Osco, Chicago, IL"},
		{UID: "dentist-call", Title: "Call dentist", Start: demoAt(today.AddDate(0, 0, 2), "14:45", loc), End: demoAt(today.AddDate(0, 0, 2), "15:00", loc)},
		{UID: "maintenance", Title: "Replace furnace filter", Start: today.AddDate(0, 0, 10), AllDay: true},
		{UID: "volunteer", Title: "Downtown Chicago Community Volunteer Orientation", Start: demoAt(today.AddDate(0, 0, 20), "10:00", loc), End: demoAt(today.AddDate(0, 0, 20), "12:00", loc), Location: "Millennium Park, 201 E Randolph St, Chicago, IL"},
	}

	files := []struct {
		File string
		Name string
		Ev   []demoEvent
	}{
		{"demo-family.green.ics", "Demo Family", family},
		{"demo-work.blue.ics", "Demo Work", work},
		{"demo-school.violet.ics", "Demo School", school},
		{"demo-home.amber.ics", "Demo Home", home},
	}
	for _, f := range files {
		if err := demoWriteICS(filepath.Join(a.calDir, f.File), f.Name, f.Ev); err != nil {
			return err
		}
	}
	return nil
}

type demoEvent struct {
	UID      string
	Title    string
	Start    time.Time
	End      time.Time
	AllDay   bool
	Location string
	Desc     string
	RRule    string
}

func demoDateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func demoMonday(t time.Time) time.Time {
	return t.AddDate(0, 0, -int(t.Weekday()-time.Monday+7)%7)
}

func demoNextWeekday(base time.Time, weekday time.Weekday, minDays int) time.Time {
	delta := (int(weekday) - int(base.Weekday()) + 7) % 7
	if delta < minDays {
		delta += 7
	}
	return base.AddDate(0, 0, delta)
}

func demoAt(day time.Time, hhmm string, loc *time.Location) time.Time {
	parts := strings.SplitN(hhmm, ":", 2)
	h, mi := 0, 0
	if len(parts) == 2 {
		fmt.Sscanf(parts[0], "%d", &h)
		fmt.Sscanf(parts[1], "%d", &mi)
	}
	return time.Date(day.Year(), day.Month(), day.Day(), h, mi, 0, 0, loc)
}

func demoWriteICS(path, name string, events []demoEvent) error {
	now := time.Now().UTC().Format("20060102T150405Z")
	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Dash-Go//Demo Mode//EN\r\nCALSCALE:GREGORIAN\r\n")
	b.WriteString("X-WR-CALNAME:" + icsEsc(name) + "\r\n")
	b.WriteString("X-WR-TIMEZONE:America/Chicago\r\n")
	for _, ev := range events {
		b.WriteString("BEGIN:VEVENT\r\n")
		uid := strings.TrimSpace(ev.UID)
		if uid == "" {
			uid = strings.ToLower(strings.ReplaceAll(ev.Title, " ", "-"))
		}
		fmt.Fprintf(&b, "UID:%s@dash-go-demo\r\nDTSTAMP:%s\r\n", icsEsc(uid), now)
		if ev.AllDay {
			end := ev.End
			if end.IsZero() {
				end = ev.Start.AddDate(0, 0, 1)
			}
			fmt.Fprintf(&b, "DTSTART;VALUE=DATE:%s\r\nDTEND;VALUE=DATE:%s\r\n", ev.Start.Format("20060102"), end.Format("20060102"))
		} else {
			end := ev.End
			if end.IsZero() {
				end = ev.Start.Add(time.Hour)
			}
			fmt.Fprintf(&b, "DTSTART;TZID=America/Chicago:%s\r\nDTEND;TZID=America/Chicago:%s\r\n", ev.Start.Format("20060102T150405"), end.Format("20060102T150405"))
		}
		b.WriteString("SUMMARY:" + icsEsc(ev.Title) + "\r\n")
		if ev.Location != "" {
			b.WriteString("LOCATION:" + icsEsc(ev.Location) + "\r\n")
		}
		if ev.Desc != "" {
			b.WriteString("DESCRIPTION:" + icsEsc(ev.Desc) + "\r\n")
		}
		if ev.RRule != "" {
			b.WriteString("RRULE:" + ev.RRule + "\r\n")
		}
		b.WriteString("END:VEVENT\r\n")
	}
	b.WriteString("END:VCALENDAR\r\n")
	return fileio.WriteAtomic(path, []byte(b.String()), 0644)
}
