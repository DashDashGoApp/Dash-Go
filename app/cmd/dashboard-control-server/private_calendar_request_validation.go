package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// These checks deliberately run before the expensive-operation lease is
// acquired. Invalid local API input must never consume the cooldown that
// protects a legitimate touchscreen action.
func (a *app) validatePrivateCalendarDiscovery() error {
	script := filepath.Join(a.binDir, "private-calendar-discovery.sh")
	info, err := os.Stat(script)
	if err != nil || info.Mode()&0111 == 0 {
		return errors.New("private calendar discovery is unavailable")
	}
	return nil
}

func (a *app) validatePrivateCalendarSync(body map[string]any) error {
	source := strings.TrimSpace(jsonutil.BodyString(body, "source"))
	for _, selection := range a.privateCalendarSelections() {
		if selection.Source != source {
			continue
		}
		return a.calendarWritebackSourceBlocked(selection.Source)
	}
	return errors.New("selected private calendar is no longer available")
}

func (a *app) validatePrivateCalendarRepair(body map[string]any) error {
	source := strings.TrimSpace(jsonutil.BodyString(body, "source"))
	selection, err := a.selectedPrivateCalendar(source)
	if err != nil {
		return err
	}
	if _, err := a.calendarWritebackService().RegisteredCalendar(selection.Source); err != nil {
		return err
	}
	script := filepath.Join(a.binDir, "private-calendar-selection.sh")
	info, err := os.Stat(script)
	if err != nil || info.Mode()&0111 == 0 {
		return errors.New("private calendar repair is unavailable")
	}
	return nil
}
