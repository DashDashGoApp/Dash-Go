package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	writebackpkg "github.com/DashDashGoApp/Dash-Go/app/internal/calendar/writeback"
	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func (a *app) initializeShowcaseRuntime() error {
	if !a.showcaseMode() {
		return nil
	}
	if err := a.syncShowcaseCalendarManifest(); err != nil {
		return err
	}
	if err := a.syncShowcaseCalendarWritebackRegistry(); err != nil {
		return err
	}
	_, err := a.refreshEventCache(true, a.showcase.manifest.Cache.DaysPast, a.showcase.manifest.Cache.DaysFuture)
	if err != nil {
		return fmt.Errorf("rebuild Showcase event cache: %w", err)
	}
	return nil
}

func (a *app) syncShowcaseCalendarManifest() error {
	if !a.showcaseMode() {
		return a.calendarService().GenerateManifest()
	}
	entries := make([]any, 0, len(a.showcase.manifest.Calendars))
	for _, calendar := range a.showcase.manifest.Calendars {
		sourcePath, err := a.showcase.dataPath(calendar.Source)
		if err != nil {
			return err
		}
		if info, err := os.Stat(sourcePath); err != nil || info.IsDir() {
			if err == nil {
				err = fmt.Errorf("is a directory")
			}
			return fmt.Errorf("Showcase calendar fixture %s is unavailable: %w", calendar.Source, err)
		}
		entries = append(entries, map[string]any{
			"url":     calendar.Source,
			"name":    calendar.Name,
			"color":   calendar.Color,
			"enabled": calendar.Enabled,
		})
	}
	return fileio.WriteJSON(filepath.Join(a.calDir, "calendars.json"), entries)
}

func (a *app) syncShowcaseCalendarWritebackRegistry() error {
	if !a.showcaseMode() {
		return nil
	}
	registry := writebackpkg.Registry{
		Version:    writebackpkg.RegistryVersion,
		Enabled:    true,
		RequirePIN: false,
		Calendars:  make([]writebackpkg.Calendar, 0, len(a.showcase.manifest.Calendars)),
	}
	for _, calendar := range a.showcase.manifest.Calendars {
		collection, err := a.showcase.dataPath(calendar.Collection)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(collection, 0700); err != nil {
			return fmt.Errorf("create Showcase session collection for %s: %w", calendar.Source, err)
		}
		registry.Calendars = append(registry.Calendars, writebackpkg.Calendar{
			Source:     calendar.Source,
			Collection: collection,
			Writable:   calendar.Writable,
			Enabled:    calendar.Enabled,
			Name:       calendar.Name,
		})
	}
	return fileio.WriteJSON(filepath.Join(a.configDir, calendarWritebackRegistryFile), registry)
}

func (a *app) showcaseStatus() map[string]any {
	if !a.showcaseMode() {
		return map[string]any{"enabled": false, "error": "Showcase runtime profile is not active."}
	}
	cache := jsonutil.Map(a.readJSONDefault(filepath.Join(a.cacheDir, "events.cache.json"), map[string]any{}))
	writeback := a.calendarWritebackStatus()
	registered := map[string]map[string]any{}
	for _, raw := range jsonutil.List(writeback["calendars"]) {
		row := jsonutil.Map(raw)
		registered[strings.TrimSpace(jsonutil.StringValue(row["source"]))] = row
	}
	eventCountBySource := map[string]int{}
	candidateCountBySource := map[string]int{}
	for _, raw := range jsonutil.List(cache["events"]) {
		row := jsonutil.Map(raw)
		source := strings.TrimSpace(jsonutil.StringValue(row["calUrl"]))
		eventCountBySource[source]++
		if jsonutil.Truthy(jsonutil.Map(row["writeback"])["candidate"]) {
			candidateCountBySource[source]++
		}
	}
	rows := make([]map[string]any, 0, len(a.showcase.manifest.Calendars))
	problems := []string{}
	for _, calendar := range a.showcase.manifest.Calendars {
		path, _ := a.showcase.dataPath(calendar.Source)
		fixturePresent := false
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			fixturePresent = true
		}
		row := map[string]any{
			"source":              calendar.Source,
			"name":                calendar.Name,
			"writable":            calendar.Writable,
			"enabled":             calendar.Enabled,
			"fixturePresent":      fixturePresent,
			"events":              eventCountBySource[calendar.Source],
			"writebackCandidates": candidateCountBySource[calendar.Source],
			"expectedEvents":      calendar.ExpectedEvents,
		}
		if registry := registered[calendar.Source]; registry != nil {
			row["writebackRegistered"] = registry["writable"] == true && registry["enabled"] == true
			row["deleteAllowed"] = registry["deleteAllowed"] == true
		} else {
			row["writebackRegistered"] = false
			row["deleteAllowed"] = false
		}
		if !fixturePresent {
			problems = append(problems, "missing fixture "+calendar.Source)
		}
		if eventCountBySource[calendar.Source] < calendar.ExpectedEvents {
			problems = append(problems, fmt.Sprintf("%s has %d cached event(s), expected at least %d", calendar.Source, eventCountBySource[calendar.Source], calendar.ExpectedEvents))
		}
		if calendar.Writable && candidateCountBySource[calendar.Source] == 0 {
			problems = append(problems, "missing writeback candidate for "+calendar.Source)
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(left, right int) bool { return rows[left]["source"].(string) < rows[right]["source"].(string) })
	cacheEvents := len(jsonutil.List(cache["events"]))
	candidates := 0
	for _, value := range candidateCountBySource {
		candidates += value
	}
	return map[string]any{
		"schema":   1,
		"contract": a.showcase.contract.Contract,
		"profile":  showcaseProfileName,
		"ready":    len(problems) == 0,
		"roots": map[string]any{
			"app":  a.dash,
			"data": a.showcase.dataRoot,
		},
		"scenario":  a.showcase.manifest.Scenario,
		"calendars": rows,
		"cache": map[string]any{
			"rebuilt":             jsonutil.Int(cache["version"], 0) == eventCacheVersion,
			"events":              cacheEvents,
			"writebackCandidates": candidates,
			"generatedAt":         cache["generatedAt"],
		},
		"problems": problems,
	}
}
