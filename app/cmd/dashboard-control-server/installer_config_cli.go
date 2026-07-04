package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
)

// Installer configuration files are small, but these expressions run on every
// installer helper invocation. Keep literal patterns compiled once at startup.
var (
	reConfigLocalBirthdaysAnchor = regexp.MustCompile(`(?m)^\s*birthdays\s*:`)
	reConfigLocalClosingBrace    = regexp.MustCompile(`(?m)^\s*};\s*$`)
	reWeatherProvidersList       = regexp.MustCompile(`(?s)\bweatherProviders\s*:\s*\[([^\]]*)\]`)
	reQuotedWeatherProvider      = regexp.MustCompile(`"([^"]+)"`)
)

func jsString(value string) string { return strconv.Quote(strings.TrimSpace(value)) }

func configLocalBody(path string) ([]byte, error) {
	body, err := os.ReadFile(path)
	if err == nil {
		return body, nil
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	return []byte("window.DASHBOARD_LOCAL = {\n  lat: 0,\n  lon: 0,\n  birthdays: []\n};\n"), nil
}

func setConfigLocalField(body []byte, key, value string) []byte {
	pattern := `(?m)^(\s*)` + regexp.QuoteMeta(key) + `\s*:\s*.*?(,?)\s*$`
	re := regexp.MustCompile(pattern)
	if re.Match(body) {
		return re.ReplaceAll(body, []byte("${1}"+key+": "+value+","))
	}
	anchor := reConfigLocalBirthdaysAnchor.FindIndex(body)
	if anchor == nil {
		anchor = reConfigLocalClosingBrace.FindIndex(body)
	}
	insert := []byte("  " + key + ": " + value + ",\n")
	if anchor == nil {
		return append(append(bytesTrimRightSpace(body), '\n'), insert...)
	}
	out := make([]byte, 0, len(body)+len(insert))
	out = append(out, body[:anchor[0]]...)
	out = append(out, insert...)
	out = append(out, body[anchor[0]:]...)
	return out
}

func bytesTrimRightSpace(value []byte) []byte {
	return []byte(strings.TrimRight(string(value), " \t\r\n"))
}

func removeConfigLocalFields(body []byte, keys ...string) []byte {
	pattern := `(?m)^\s*(?:` + strings.Join(keys, "|") + `)\s*:\s*.*?,?\s*\n`
	return regexp.MustCompile(pattern).ReplaceAll(body, nil)
}

func (a *app) runInstallerConfigLocalCLI(args []string) int {
	fs := flag.NewFlagSet("installer-config-local", flag.ContinueOnError)
	file := fs.String("file", "", "config.local.js")
	mode := fs.String("mode", "", "radar, weather, or display")
	provider := fs.String("provider", "", "radar provider")
	customTiles := fs.String("custom-tiles", "", "custom radar tiles")
	providers := fs.String("providers", "", "space-separated weather providers")
	wxAPI := fs.String("wx-api", "", "weather API base")
	aqAPI := fs.String("aq-api", "", "air quality API base")
	tempUnit := fs.String("temp-unit", "", "temperature unit")
	windUnit := fs.String("wind-unit", "", "wind unit")
	weatherDays := fs.Int("weather-days", 14, "weather days")
	refreshWX := fs.Int("refresh-wx", 30, "weather refresh minutes")
	alertRefresh := fs.Int("alert-refresh", 5, "alert refresh minutes")
	alertMin := fs.String("alert-min", "moderate", "alert severity")
	if err := fs.Parse(args); err != nil || *file == "" {
		fmt.Fprintln(os.Stderr, "usage: --installer-config-local --file PATH --mode radar|weather|display [values]")
		return 64
	}
	body, err := configLocalBody(*file)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	switch *mode {
	case "radar":
		value := strings.TrimSpace(*provider)
		if value == "" {
			value = "rainviewer"
		}
		body = setConfigLocalField(body, "radarProvider", jsString(value))
		if strings.TrimSpace(*customTiles) != "" {
			body = setConfigLocalField(body, "radarCustomTiles", jsString(*customTiles))
		} else {
			body = removeConfigLocalFields(body, "radarCustomTiles")
		}
	case "weather":
		list := []string{}
		for _, item := range strings.Fields(*providers) {
			if item = strings.TrimSpace(item); item != "" {
				list = append(list, item)
			}
		}
		if len(list) == 0 {
			list = []string{"openmeteo"}
		}
		quoted := make([]string, 0, len(list))
		for _, item := range list {
			quoted = append(quoted, jsString(item))
		}
		body = setConfigLocalField(body, "weatherProviders", "["+strings.Join(quoted, ", ")+"]")
		body = removeConfigLocalFields(body, "weatherProviderKeys", "apiKey")
		if v := strings.TrimSuffix(strings.TrimSpace(*wxAPI), "/"); v != "" {
			body = setConfigLocalField(body, "wxApi", jsString(v))
		}
		if v := strings.TrimSuffix(strings.TrimSpace(*aqAPI), "/"); v != "" {
			body = setConfigLocalField(body, "aqApi", jsString(v))
		}
	case "display":
		unit := strings.TrimSpace(*tempUnit)
		if unit == "" {
			unit = "fahrenheit"
		}
		wind := strings.TrimSpace(*windUnit)
		if wind == "" {
			wind = "mph"
		}
		body = setConfigLocalField(body, "tempUnit", jsString(unit))
		body = setConfigLocalField(body, "windUnit", jsString(wind))
		body = setConfigLocalField(body, "weatherDays", strconv.Itoa(max(1, *weatherDays)))
		body = setConfigLocalField(body, "refreshWxMinutes", strconv.Itoa(max(1, *refreshWX)))
		body = setConfigLocalField(body, "weatherAlerts", "{ enabled: true, refreshMinutes: "+strconv.Itoa(max(1, *alertRefresh))+", minSeverity: "+jsString(*alertMin)+" }")
	default:
		fmt.Fprintln(os.Stderr, "--mode must be radar, weather, or display")
		return 64
	}
	if err := fileio.WriteAtomic(*file, body, 0644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func (a *app) runInstallerWeatherProvidersCLI(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: --installer-weather-providers CONFIG_LOCAL")
		return 64
	}
	body, err := os.ReadFile(args[0])
	if os.IsNotExist(err) {
		fmt.Println("openmeteo")
		return 0
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	match := reWeatherProvidersList.FindSubmatch(body)
	if len(match) < 2 {
		fmt.Println("openmeteo")
		return 0
	}
	supported := map[string]string{"metno": "weatherbit", "meteosource": "weatherbit", "openmeteo": "openmeteo", "nws": "nws", "weatherapi": "weatherapi", "openweather": "openweather", "googleweather": "googleweather", "tomorrow": "tomorrow", "visualcrossing": "visualcrossing", "weatherbit": "weatherbit", "pirateweather": "pirateweather", "accuweather": "accuweather", "xweather": "xweather", "openmeteo-custom": "openmeteo-custom"}
	values := reQuotedWeatherProvider.FindAllStringSubmatch(string(match[1]), -1)
	out := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		key := strings.ToLower(strings.TrimSpace(value[1]))
		if key = supported[key]; key != "" && !seen[key] {
			seen[key] = true
			out = append(out, key)
		}
	}
	if len(out) == 0 {
		out = []string{"openmeteo"}
	}
	fmt.Println(strings.Join(out, " "))
	return 0
}
