package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	controlauth "github.com/DashDashGoApp/Dash-Go/app/internal/auth"
	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func (a *app) runPinHashCLI(args []string) int {
	fs := flag.NewFlagSet("pin-hash", flag.ContinueOnError)
	pin := fs.String("pin", os.Getenv("PIN_VALUE"), "4-8 digit PIN")
	timeout := fs.String("timeout", "", "unlock timeout")
	if err := fs.Parse(args); err != nil {
		return 64
	}
	payload, err := controlauth.NewPINPayload(strings.TrimSpace(*pin), *timeout)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	order := []string{"DASH_CONTROL_PIN_ENABLED", "DASH_CONTROL_PIN_ITERATIONS", "DASH_CONTROL_PIN_SALT", "DASH_CONTROL_PIN_HASH"}
	for _, key := range order {
		fmt.Printf("%s=%s\n", key, payload[key])
	}
	return 0
}

type jsonSetValueMode uint8

const (
	jsonSetAuto jsonSetValueMode = iota
	jsonSetString
	jsonSetJSON
)

// parseCLIValue preserves the legacy --json-set behavior. New installer-owned
// calls use explicit string or JSON modes so text such as "0420", "true", or
// "null" cannot silently change type.
func parseCLIValue(raw string) any {
	var value any
	if json.Unmarshal([]byte(raw), &value) == nil {
		return value
	}
	return raw
}

func parseJSONSetValue(raw string, mode jsonSetValueMode) (any, error) {
	switch mode {
	case jsonSetString:
		return raw, nil
	case jsonSetJSON:
		var value any
		if err := json.Unmarshal([]byte(raw), &value); err != nil {
			return nil, fmt.Errorf("VALUE must be valid JSON for --json-set-json: %w", err)
		}
		return value, nil
	default:
		return parseCLIValue(raw), nil
	}
}

func jsonObjectFile(path string) (map[string]any, error) {
	body, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		return nil, err
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("JSON root must be an object")
	}
	return object, nil
}

func jsonSetPath(root map[string]any, dotted string, value any) error {
	parts := strings.FieldsFunc(dotted, func(r rune) bool { return r == '.' })
	if len(parts) == 0 {
		return errors.New("field must not be empty")
	}
	current := root
	for _, part := range parts[:len(parts)-1] {
		value, exists := current[part]
		if !exists {
			child := map[string]any{}
			current[part] = child
			current = child
			continue
		}
		child, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("field %q is not an object; refusing to overwrite it", part)
		}
		current = child
	}
	current[parts[len(parts)-1]] = value
	return nil
}

func (a *app) runJSONSetModeCLI(args []string, mode jsonSetValueMode, usage string) int {
	if len(args) != 3 {
		fmt.Fprintf(os.Stderr, "usage: %s FILE FIELD VALUE\n", usage)
		return 64
	}
	root, err := jsonObjectFile(args[0])
	if err == nil {
		var value any
		value, err = parseJSONSetValue(args[2], mode)
		if err == nil {
			err = jsonSetPath(root, args[1], value)
		}
	}
	if err == nil {
		err = fileio.WriteJSON(args[0], root)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// runJSONSetCLI remains for compatibility with existing external scripts.
// Prefer --json-set-string for user-entered text and --json-set-json for
// deliberate structured values.
func (a *app) runJSONSetCLI(args []string) int {
	return a.runJSONSetModeCLI(args, jsonSetAuto, "--json-set")
}

func (a *app) runJSONSetStringCLI(args []string) int {
	return a.runJSONSetModeCLI(args, jsonSetString, "--json-set-string")
}

func (a *app) runJSONSetJSONCLI(args []string) int {
	return a.runJSONSetModeCLI(args, jsonSetJSON, "--json-set-json")
}

func (a *app) runInstallerTodoSettingsCLI(args []string) int {
	fs := flag.NewFlagSet("installer-todo-settings", flag.ContinueOnError)
	path := fs.String("file", "", "settings JSON path")
	mode := fs.String("mode", "local", "local or microsoft")
	clientID := fs.String("client-id", "", "optional Microsoft public client ID")
	if err := fs.Parse(args); err != nil || *path == "" {
		fmt.Fprintln(os.Stderr, "usage: --installer-todo-settings --file PATH --mode local|microsoft [--client-id ID]")
		return 64
	}
	root, err := jsonObjectFile(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "settings.json is not valid JSON; resolve it before App Setup can safely change Lists settings")
		return 1
	}
	todo := jsonutil.Map(root["todo"])
	mapping := jsonutil.Map(todo["map"])
	todo["source"] = "local"
	todo["syncMode"] = *mode
	if *mode == "local" {
		mapping["todo"] = "local-todo"
		mapping["grocery"] = "local-grocery"
	} else {
		if strings.TrimSpace(jsonutil.StringValue(mapping["todo"])) == "" {
			mapping["todo"] = "local-todo"
		}
		if strings.TrimSpace(jsonutil.StringValue(mapping["grocery"])) == "" {
			mapping["grocery"] = "local-grocery"
		}
	}
	if strings.TrimSpace(*clientID) != "" {
		todo["clientId"] = strings.TrimSpace(*clientID)
	}
	todo["map"] = mapping
	root["todo"] = todo
	if err := fileio.WriteJSON(*path, root); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func removeConfigLocalLines(path string, keys ...string) error {
	body, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	updated, err := removeConfigLocalFields(body, keys...)
	if err != nil {
		return fmt.Errorf("refusing to edit config.local.js: %w", err)
	}
	if string(updated) == string(body) {
		return nil
	}
	return fileio.WriteAtomic(path, updated, 0644)
}

func (a *app) runInstallerNormalizeAppVisibilityCLI(args []string) int {
	fs := flag.NewFlagSet("installer-normalize-app-visibility", flag.ContinueOnError)
	settingsPath := fs.String("settings", "", "settings JSON path")
	configPath := fs.String("config-local", "", "config.local.js path")
	if err := fs.Parse(args); err != nil || *settingsPath == "" || *configPath == "" {
		fmt.Fprintln(os.Stderr, "usage: --installer-normalize-app-visibility --settings PATH --config-local PATH")
		return 64
	}
	if _, err := os.Stat(*settingsPath); err == nil {
		root, err := jsonObjectFile(*settingsPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "settings.json is not valid JSON; leaving legacy app visibility fields untouched")
			return 1
		}
		changed := false
		for _, key := range []string{"showChalkboard", "radarEnabled"} {
			if _, ok := root[key]; ok {
				delete(root, key)
				changed = true
			}
		}
		if todo, ok := root["todo"].(map[string]any); ok {
			if _, exists := todo["enabled"]; exists {
				delete(todo, "enabled")
				changed = true
			}
			mapping := jsonutil.Map(todo["map"])
			if _, ok := todo["map"].(map[string]any); !ok {
				todo["map"] = mapping
				changed = true
			}
			for slot, dflt := range map[string]string{"todo": "local-todo", "grocery": "local-grocery"} {
				if strings.TrimSpace(jsonutil.StringValue(mapping[slot])) == "" {
					mapping[slot] = dflt
					changed = true
				}
			}
		}
		if changed {
			if err := fileio.WriteJSON(*settingsPath, root); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
		}
	} else if !os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := removeConfigLocalLines(*configPath, "showChalkboard", "radarEnabled"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func (a *app) runGeocodeCLI(args []string) int {
	fs := flag.NewFlagSet("geocode", flag.ContinueOnError)
	name := fs.String("name", "", "city or town")
	if err := fs.Parse(args); err != nil || strings.TrimSpace(*name) == "" {
		fmt.Fprintln(os.Stderr, "usage: --geocode --name PLACE")
		return 64
	}
	query := strings.TrimSpace(strings.SplitN(*name, ",", 2)[0])
	base := strings.TrimSpace(os.Getenv("DASH_GO_GEOCODE_ENDPOINT"))
	if base == "" {
		base = "https://geocoding-api.open-meteo.com/v1/search"
	}
	endpointURL, err := url.Parse(base)
	if err != nil || endpointURL.Scheme == "" || endpointURL.Host == "" {
		fmt.Fprintln(os.Stderr, "invalid geocoding endpoint")
		return 1
	}
	values := endpointURL.Query()
	values.Set("name", query)
	values.Set("count", "5")
	values.Set("language", "en")
	values.Set("format", "json")
	endpointURL.RawQuery = values.Encode()
	endpoint := endpointURL.String()
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 20 * time.Second}
	client := &http.Client{
		Timeout: 20 * time.Second,
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           dialer.DialContext,
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: 15 * time.Second,
		},
	}
	response, err := client.Get(endpoint)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "geocoding returned HTTP %d\n", response.StatusCode)
		return 1
	}
	var payload struct {
		Results []struct {
			Name      string  `json:"name"`
			Admin1    string  `json:"admin1"`
			Country   string  `json:"country"`
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		} `json:"results"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, result := range payload.Results {
		label := strings.Join(compactStrings(result.Name, result.Admin1, result.Country), ", ")
		if label == "" {
			continue
		}
		fmt.Printf("%s|%s|%s\n", strconv.FormatFloat(result.Latitude, 'f', -1, 64), strconv.FormatFloat(result.Longitude, 'f', -1, 64), label)
	}
	return 0
}

func compactStrings(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func (a *app) runInstallerMessageFilesCLI(args []string) int {
	fs := flag.NewFlagSet("installer-message-files", flag.ContinueOnError)
	dir := fs.String("dir", "", "configuration directory")
	if err := fs.Parse(args); err != nil || *dir == "" {
		fmt.Fprintln(os.Stderr, "usage: --installer-message-files --dir PATH")
		return 64
	}
	files := map[string]any{
		"compliments.json":     map[string]any{"messages": []any{}, "defaultsCleared": false, "defaultsSeeded": false, "removedDefaults": []any{}, "defaultEdits": map[string]any{}, "version": 4},
		"message-sources.json": map[string]any{"enabled": []any{}, "updatedAt": 0},
		"message-cache.json": map[string]any{"items": []any{
			map[string]any{"id": "example-welcome-01", "text": "Example: Pick Message sources to add quotes, jokes, and facts here.", "source": "example", "nsfw": false, "weight": 1, "edited": false},
			map[string]any{"id": "example-welcome-02", "text": "Example: Deleted or edited pulled items will stay that way after refresh.", "source": "example", "nsfw": false, "weight": 1, "edited": false},
			map[string]any{"id": "example-welcome-03", "text": "Example: Enable sources, then tap Refresh now to replace examples with fresh items.", "source": "example", "nsfw": false, "weight": 1, "edited": false},
		}, "generatedAt": 0, "sources": []any{"example"}, "enabled": []any{}},
		"message-cache-overrides.json": map[string]any{"removed": []any{}, "edits": map[string]any{}},
		"temp-messages.json":           []any{},
		"scheduled-messages.json":      []any{},
	}
	for name, fallback := range files {
		path := filepath.Join(*dir, name)
		valid := false
		if raw, err := os.ReadFile(path); err == nil {
			var value any
			if json.Unmarshal(raw, &value) == nil {
				switch fallback.(type) {
				case map[string]any:
					_, valid = value.(map[string]any)
				case []any:
					_, valid = value.([]any)
				}
			}
		}
		if !valid {
			if err := fileio.WriteJSON(path, fallback); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
		}
	}
	return 0
}
