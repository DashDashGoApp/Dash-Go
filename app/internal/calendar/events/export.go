package events

import "time"

// The exported surface is intentionally compact. Core keeps route/CLI and
// calendar-management orchestration; this package owns the event domain.
func (s *Service) LoadCalendars() []CalendarSource { return s.loadCalendars() }
func (s *Service) URLToPath(url string) string     { return s.eventURLToPath(url) }

func (s *Service) Refresh(force bool, past, future int) (map[string]any, error) {
	return s.refresh(force, past, future)
}

func ParseICS(text string, cal CalendarSource) []ICSEvent { return parseICS(text, cal) }
func ParseICSDate(raw string, params map[string]string) (time.Time, bool, bool) {
	return parseICSDate(raw, params)
}
func Expand(ev ICSEvent, start, end time.Time) []ICSEvent { return expand(ev, start, end) }
