package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	eventspkg "github.com/DashDashGoApp/Dash-Go/app/internal/calendar/events"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// Calendar-event route/CLI adaptation remains in core. Calendar management,
// source identity, owned-feed metadata, and visibility now live in
// internal/calendar; internal/calendar/events stays focused on ICS parsing,
// recurrence, source inspection, fingerprints, and durable cache construction.
type (
	calendarSource   = eventspkg.CalendarSource
	sourceMeta       = eventspkg.SourceMeta
	icsEvent         = eventspkg.ICSEvent
	eventCacheOutput = eventspkg.CacheOutput
)

const (
	eventCacheVersion       = eventspkg.CacheVersion
	eventFingerprintVersion = eventspkg.FingerprintVersion
)

func (a *app) eventService() *eventspkg.Service {
	a.eventsInitMu.Lock()
	defer a.eventsInitMu.Unlock()
	if a.events == nil {
		a.events = eventspkg.New(eventspkg.ServiceConfig{
			DashDir:        a.dash,
			CalendarDir:    a.calDir,
			CacheDir:       a.cacheDir,
			OutputEnabled:  a.calendarOutputEnabledForURL,
			SourceIdentity: calendarSourceIdentity,
			OwnedSource:    ownedCalendarSource,
			Now:            time.Now,
		})
	}
	return a.events
}

func parseICSGo(text string, cal calendarSource) []icsEvent {
	return eventspkg.ParseICS(text, cal)
}
func parseICSDateGo(raw string, params map[string]string) (time.Time, bool, bool) {
	return eventspkg.ParseICSDate(raw, params)
}
func expandEventGo(ev icsEvent, start, end time.Time) []icsEvent {
	return eventspkg.Expand(ev, start, end)
}
func (a *app) loadEventCalendars() []calendarSource { return a.eventService().LoadCalendars() }
func (a *app) eventURLToPath(url string) string     { return a.eventService().URLToPath(url) }

func (a *app) runEventCacheCLI(args []string) int {
	force := false
	daysPast, daysFuture := 90, 365
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--force":
			force = true
		case "--days-past":
			if i+1 < len(args) {
				if n, err := strconv.Atoi(args[i+1]); err == nil {
					daysPast = n
				}
				i++
			}
		case "--days-future":
			if i+1 < len(args) {
				if n, err := strconv.Atoi(args[i+1]); err == nil {
					daysFuture = n
				}
				i++
			}
		case "-h", "--help":
			fmt.Println("usage: dashboard-control-server --gen-events-cache [--force] [--days-past N] [--days-future N]")
			return 0
		}
	}
	res, err := a.refreshEventCache(force, daysPast, daysFuture)
	if err != nil {
		fmt.Fprintln(os.Stderr, "events.cache.json failed:", err)
		return 1
	}
	if res["unchanged"] == true {
		fmt.Println("events.cache.json unchanged")
	} else {
		fmt.Printf("events.cache.json wrote %v events", res["eventCount"])
		if jsonutil.Int(res["issueCount"], 0) > 0 {
			fmt.Print(" with issues")
		}
		fmt.Println()
	}
	for _, raw := range jsonutil.List(res["issues"]) {
		fmt.Fprintln(os.Stderr, "WARN", fmt.Sprint(raw))
	}
	return 0
}

func (a *app) refreshEventCache(force bool, daysPast int, daysFuture int) (map[string]any, error) {
	// Calendar owns ownership/visibility and the manifest write. The event child
	// receives the already-established source policy through this façade.
	_ = a.generateCalendarManifest()
	return a.eventService().Refresh(force, daysPast, daysFuture)
}
