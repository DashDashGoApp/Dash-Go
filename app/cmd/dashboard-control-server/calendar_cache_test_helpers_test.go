package main

import "testing"

// disableCalendarCacheRefreshForTest replaces the lazy Calendar service with a
// test-local instance whose post-commit refresh port is inert. Calendar holds
// only immutable path/callback configuration plus its own lock, so replacing a
// fixture service does not change persisted data or production behavior.
func disableCalendarCacheRefreshForTest(t *testing.T, a *app) {
	t.Helper()
	a.calendarInitMu.Lock()
	defer a.calendarInitMu.Unlock()
	a.calendar = a.newCalendarService(func() {})
}
