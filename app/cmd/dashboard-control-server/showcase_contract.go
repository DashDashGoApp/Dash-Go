package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	showcaseContractSchema = 1
	showcaseContractName   = "dashgo-showcase/v1"
	showcaseProfileName    = "showcase"
)

var (
	showcaseSourcePattern = regexp.MustCompile(`^calendars/[A-Za-z0-9][A-Za-z0-9._-]*\.ics$`)
	showcaseColorPattern  = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
)

type showcaseContract struct {
	Schema       int             `json:"schema"`
	Contract     string          `json:"contract"`
	Capabilities map[string]bool `json:"capabilities"`
}

type showcaseCacheWindow struct {
	DaysPast   int `json:"daysPast"`
	DaysFuture int `json:"daysFuture"`
}

type showcaseCalendar struct {
	Source         string `json:"source"`
	Name           string `json:"name"`
	Color          string `json:"color"`
	Collection     string `json:"collection"`
	Writable       bool   `json:"writable"`
	Enabled        bool   `json:"enabled"`
	ExpectedEvents int    `json:"expectedEvents"`
}

type showcaseScenarioManifest struct {
	Schema    int                 `json:"schema"`
	Contract  string              `json:"contract"`
	Profile   string              `json:"profile"`
	Scenario  string              `json:"scenario"`
	Cache     showcaseCacheWindow `json:"cache"`
	Calendars []showcaseCalendar  `json:"calendars"`
}

type showcaseRuntime struct {
	contract     showcaseContract
	manifest     showcaseScenarioManifest
	manifestPath string
	dataRoot     string
	homeDir      string
	calendars    map[string]showcaseCalendar
}

func loadShowcaseContract(dash string) (showcaseContract, error) {
	path := filepath.Join(dash, "release", "showcase-contract.json")
	var contract showcaseContract
	if err := readStrictJSON(path, &contract); err != nil {
		return showcaseContract{}, fmt.Errorf("read Showcase contract: %w", err)
	}
	if contract.Schema != showcaseContractSchema || strings.TrimSpace(contract.Contract) != showcaseContractName {
		return showcaseContract{}, errors.New("unsupported Showcase contract")
	}
	for _, capability := range []string{
		"separateDataRoot",
		"scenarioManifest",
		"scenarioCalendars",
		"calendarWritebackAllowlist",
		"cacheRebuildReport",
		"statusEndpoint",
		"staticScenarioAssets",
	} {
		if contract.Capabilities[capability] != true {
			return showcaseContract{}, fmt.Errorf("Showcase contract does not declare required capability %q", capability)
		}
	}
	return contract, nil
}

func readStrictJSON(path string, target any) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 2*1024*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("contains more than one JSON value")
		}
		return err
	}
	return nil
}

func loadShowcaseRuntime(dash string) (*showcaseRuntime, error) {
	profile := strings.TrimSpace(os.Getenv("DASHGO_RUNTIME_PROFILE"))
	manifestPath := strings.TrimSpace(os.Getenv("DASHGO_SHOWCASE_MANIFEST"))
	dataRoot := strings.TrimSpace(os.Getenv("DASHGO_DATA_ROOT"))
	if profile == "" {
		if manifestPath != "" || dataRoot != "" {
			return nil, errors.New("DASHGO_SHOWCASE_MANIFEST and DASHGO_DATA_ROOT require DASHGO_RUNTIME_PROFILE=showcase")
		}
		return nil, nil
	}
	if profile != showcaseProfileName {
		return nil, fmt.Errorf("unsupported DASHGO_RUNTIME_PROFILE %q", profile)
	}
	if manifestPath == "" || dataRoot == "" {
		return nil, errors.New("Showcase runtime requires DASHGO_SHOWCASE_MANIFEST and DASHGO_DATA_ROOT")
	}
	if !filepath.IsAbs(manifestPath) || !filepath.IsAbs(dataRoot) {
		return nil, errors.New("Showcase manifest and data root must be absolute paths")
	}
	dash = filepath.Clean(dash)
	dataRoot = filepath.Clean(dataRoot)
	manifestPath = filepath.Clean(manifestPath)
	if rootsOverlap(dash, dataRoot) {
		return nil, errors.New("Showcase data root must be separate from the immutable Dash-Go app root")
	}
	if !pathInside(dataRoot, manifestPath) {
		return nil, errors.New("Showcase manifest must be stored below the supplied Showcase data root")
	}
	contract, err := loadShowcaseContract(dash)
	if err != nil {
		return nil, err
	}
	var manifest showcaseScenarioManifest
	if err := readStrictJSON(manifestPath, &manifest); err != nil {
		return nil, fmt.Errorf("read Showcase scenario manifest: %w", err)
	}
	runtime := &showcaseRuntime{
		contract:     contract,
		manifest:     manifest,
		manifestPath: manifestPath,
		dataRoot:     dataRoot,
		homeDir:      filepath.Join(dataRoot, "home"),
		calendars:    map[string]showcaseCalendar{},
	}
	if err := runtime.validate(); err != nil {
		return nil, err
	}
	return runtime, nil
}

func (runtime *showcaseRuntime) validate() error {
	if runtime == nil {
		return errors.New("Showcase runtime is unavailable")
	}
	manifest := runtime.manifest
	if manifest.Schema != 1 || strings.TrimSpace(manifest.Contract) != runtime.contract.Contract || strings.TrimSpace(manifest.Profile) != showcaseProfileName {
		return errors.New("Showcase scenario manifest does not match Dash-Go Showcase Contract v1")
	}
	if value := strings.TrimSpace(manifest.Scenario); value == "" || len(value) > 96 {
		return errors.New("Showcase scenario manifest has an invalid scenario identifier")
	}
	if manifest.Cache.DaysPast < 0 || manifest.Cache.DaysPast > 366 || manifest.Cache.DaysFuture < 1 || manifest.Cache.DaysFuture > 731 {
		return errors.New("Showcase scenario manifest has an invalid cache window")
	}
	if len(manifest.Calendars) < 1 || len(manifest.Calendars) > 16 {
		return errors.New("Showcase scenario manifest must declare between one and sixteen calendars")
	}
	for _, calendar := range manifest.Calendars {
		source := strings.TrimSpace(calendar.Source)
		if !showcaseSourcePattern.MatchString(source) {
			return fmt.Errorf("Showcase scenario manifest has an invalid calendar source %q", calendar.Source)
		}
		if _, found := runtime.calendars[source]; found {
			return fmt.Errorf("Showcase scenario manifest has a duplicate calendar source %q", source)
		}
		if name := strings.TrimSpace(calendar.Name); name == "" || len(name) > 80 {
			return fmt.Errorf("Showcase scenario manifest has an invalid calendar name for %q", source)
		}
		if color := strings.TrimSpace(calendar.Color); color == "" || !showcaseColorPattern.MatchString(color) {
			return fmt.Errorf("Showcase scenario manifest has an invalid calendar color for %q", source)
		}
		if calendar.ExpectedEvents < 0 || calendar.ExpectedEvents > 10000 {
			return fmt.Errorf("Showcase scenario manifest has an invalid expected event count for %q", source)
		}
		if _, err := runtime.dataPath(calendar.Source); err != nil {
			return fmt.Errorf("Showcase calendar source %q is unsafe: %w", source, err)
		}
		collection, err := runtime.dataPath(calendar.Collection)
		if err != nil {
			return fmt.Errorf("Showcase calendar collection %q is unsafe: %w", calendar.Collection, err)
		}
		collectionsRoot := filepath.Join(runtime.homeDir, ".dashboard-vdirsyncer", "collections")
		if !pathInside(collectionsRoot, collection) || filepath.Clean(collection) == filepath.Clean(collectionsRoot) {
			return fmt.Errorf("Showcase calendar collection for %q must stay below home/.dashboard-vdirsyncer/collections", source)
		}
		runtime.calendars[source] = calendar
	}
	return nil
}

func (runtime *showcaseRuntime) dataPath(relative string) (string, error) {
	if runtime == nil {
		return "", errors.New("Showcase runtime is unavailable")
	}
	relative = strings.TrimSpace(relative)
	if relative == "" || filepath.IsAbs(relative) || strings.Contains(relative, "\\") {
		return "", errors.New("relative path must use a non-empty forward-slash form")
	}
	clean := filepath.Clean(filepath.FromSlash(relative))
	if clean == "." || clean == string(filepath.Separator) || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
		return "", errors.New("relative path escapes the Showcase data root")
	}
	full := filepath.Join(runtime.dataRoot, clean)
	if !pathInside(runtime.dataRoot, full) {
		return "", errors.New("relative path escapes the Showcase data root")
	}
	return full, nil
}

func pathInside(root, candidate string) bool {
	root = filepath.Clean(root)
	candidate = filepath.Clean(candidate)
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func rootsOverlap(left, right string) bool {
	return pathInside(left, right) || pathInside(right, left)
}

func (a *app) showcaseMode() bool {
	return a != nil && a.showcase != nil
}

func (a *app) showcaseStaticDataPath(relative string) (string, bool) {
	if !a.showcaseMode() {
		return "", false
	}
	var valid bool
	relative, valid = staticURLRelativePath(relative)
	if !valid {
		return "", false
	}
	allowed := map[string]bool{
		"config/config.local.js":         true,
		"config/compliments.json":        true,
		"config/message-cache.json":      true,
		"config/temp-messages.json":      true,
		"config/scheduled-messages.json": true,
		"config/settings.json":           true,
		"calendars/calendars.json":       true,
		"cache/events.cache.json":        true,
	}
	if _, found := a.showcase.calendars[relative]; found {
		allowed[relative] = true
	}
	if !allowed[relative] {
		return "", false
	}
	path, err := a.showcase.dataPath(relative)
	if err != nil {
		return "", false
	}
	return path, true
}

func (a *app) runShowcaseContractCLI(args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "usage: dashboard-control-server --showcase-contract")
		return 2
	}
	contract, err := loadShowcaseContract(a.dash)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	body, _ := json.Marshal(contract)
	fmt.Println(string(body))
	return 0
}
