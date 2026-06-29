package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
	messagespkg "github.com/DashDashGoApp/Dash-Go/app/internal/messages"
)

var demoCompliments = []string{
	"Demo mode is showing sample Chicago calendars, weather, and messages.",
	"Tap a busy day to preview the day timeline popup.",
	"Triple-tap the moon phase to open Dashboard Control and explore display and calendar settings.",
	"Long events, recurring events, and multi-day trips are included in this demo.",
	"Run the installer again to leave Demo Mode and configure your real dashboard.",
	"Chicago weather is live when the network is available; sample calendars are local.",
}

var demoTextMarkers = []string{
	"Demo mode", "Demo Mode", "Demo tip:", "sample Chicago calendars",
	"Triple-tap the moon phase to open Dashboard Control", "Run the installer again to leave Demo Mode",
	"Re-run the installer to reset Demo Mode",
}

func (a *app) runUpdateMessageFeedsCLI(args []string) int {
	fs := flag.NewFlagSet("update-message-feeds", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	noNetwork := fs.Bool("no-network", false, "use local fallback pools only")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	payload := a.refreshMessages(context.Background(), !*noNetwork, false)
	out := map[string]any{"ok": true, "items": len(jsonutil.List(payload["items"])), "sources": payload["sources"], "sourceStatus": payload["sourceStatus"], "generator": "go"}
	b, _ := json.Marshal(out)
	fmt.Println(string(b))
	return 0
}

func (a *app) runMigrateComplimentsCLI(args []string) int {
	fs := flag.NewFlagSet("migrate-compliments", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	check := fs.Bool("check", false, "verify only; exit 1 if migration is needed")
	path := fs.String("path", a.complimentsPath(), "optional compliments.json path")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	raw := jsonutil.Map(a.readJSONDefault(*path, map[string]any{}))
	canonical, changed := messagespkg.CanonicalCompliments(raw)
	if *check {
		if changed {
			fmt.Println("compliments.json needs migration to canonical v4")
			return 1
		}
		fmt.Println("compliments.json is canonical v4")
		return 0
	}
	if err := fileio.WriteJSON(*path, canonical); err != nil {
		fmt.Fprintf(os.Stderr, "migrate compliments failed: %v\n", err)
		return 1
	}
	fmt.Printf("compliments.json migrated to canonical v4 (%d messages)\n", len(jsonutil.List(canonical["messages"])))
	return 0
}

func (a *app) runSetupDemoModeCLI(args []string) int {
	fs := flag.NewFlagSet("setup-demo-mode", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	cleanMessages := fs.Bool("clean-messages", false, "remove demo messages from message stores")
	hasLeaks := fs.Bool("has-message-leaks", false, "print leaked demo messages and exit")
	reset := fs.Bool("reset", false, "reset demo files before seeding")
	clear := fs.Bool("clear", false, "remove demo files and flags without re-seeding")
	wipeCalendars := fs.Bool("wipe-calendars", false, "remove all calendars while clearing demo mode")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *hasLeaks {
		leaks := a.demoMessageLeaks()
		for _, leak := range leaks {
			fmt.Println(leak)
		}
		return 0
	}
	if *cleanMessages {
		n := a.cleanDemoMessages()
		fmt.Printf("removed %d demo message entries\n", n)
		return 0
	}
	if *reset || *clear {
		if err := a.clearDemoMode(*wipeCalendars); err != nil {
			fmt.Fprintf(os.Stderr, "clear demo mode failed: %v\n", err)
			return 1
		}
	}
	if *clear {
		fmt.Println("demo mode cleared")
		return 0
	}
	if err := a.seedDemoMode(); err != nil {
		fmt.Fprintf(os.Stderr, "setup demo mode failed: %v\n", err)
		return 1
	}
	fmt.Println("demo mode seeded with Go-generated calendars, Chicago weather, and sample messages")
	return 0
}

func (a *app) clearDemoMode(wipeCalendars bool) error {
	a.cleanDemoMessages()
	for _, f := range []string{"demo-family.green.ics", "demo-work.blue.ics", "demo-school.violet.ics", "demo-home.amber.ics"} {
		_ = os.Remove(filepath.Join(a.calDir, f))
	}
	if wipeCalendars {
		if err := os.RemoveAll(a.calDir); err != nil {
			return err
		}
		if err := os.MkdirAll(a.calDir, 0755); err != nil {
			return err
		}
	}
	_ = os.Remove(filepath.Join(a.cacheDir, "demo-mode.json"))
	if _, err := a.updateSettings(func(settings map[string]any) {
		delete(settings, "demoMode")
	}); err != nil {
		return err
	}
	return a.clearDemoConfigLocalFlag()
}

func (a *app) clearDemoConfigLocalFlag() error {
	path := filepath.Join(a.configDir, "config.local.js")
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	// Preserve non-demo preferences if a user has already edited this file. The
	// demo writer puts this on its own line, but the expression also handles a
	// compact one-line object without deleting latitude, theme, or other fields.
	demoProperty := reDemoMode
	cleaned := demoProperty.ReplaceAllString(string(b), "")
	return fileio.WriteAtomic(path, []byte(cleaned), 0644)
}

func (a *app) seedDemoMode() error {
	_ = os.MkdirAll(a.calDir, 0755)
	_ = os.MkdirAll(a.configDir, 0755)

	if err := a.writeDemoCalendars(time.Now()); err != nil {
		return err
	}
	if err := a.writeDemoConfigLocal(); err != nil {
		return err
	}
	if err := a.writeDemoMessages(); err != nil {
		return err
	}

	if _, err := a.updateSettings(func(settings map[string]any) {
		settings["lat"] = 41.8781
		settings["lon"] = -87.6298
		settings["locationName"] = "Chicago, IL"
		settings["weatherLocationName"] = "Chicago, IL"
		settings["demoMode"] = true
		settings["tempUnit"] = "fahrenheit"
		settings["clock24"] = false
		settings["showUV"] = true
		settings["showAQI"] = true
		settings["weatherDays"] = 7
		settings["agendaDays"] = 10
		settings["weeksAbove"] = 1
		settings["weeksBelow"] = 8
		settings["showIsoWeekNumbers"] = true
		settings["fontPreset"] = "default"
		settings["weatherIconStyle"] = "soft"
		settings["seasonalDecor"] = "subtle"
		settings["showEventMaps"] = true
		settings["showInteractiveMaps"] = false
	}); err != nil {
		return err
	}

	_ = fileio.WriteJSON(filepath.Join(a.cacheDir, "demo-mode.json"), map[string]any{"enabled": true, "location": "Chicago, IL", "generatedAt": nowMillis(), "generator": "go", "dataset": "built-in-demo"})
	if _, err := a.generateDefaultCalendars(true); err != nil {
		return err
	}
	_, _ = a.refreshEventCache(true, 90, 365)
	return nil
}
