package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
)

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
	set := func(key, value string) {
		if err == nil {
			body, err = setConfigLocalField(body, key, value)
		}
	}
	remove := func(keys ...string) {
		if err == nil {
			body, err = removeConfigLocalFields(body, keys...)
		}
	}
	switch *mode {
	case "radar":
		value := strings.TrimSpace(*provider)
		if value == "" {
			value = "rainviewer"
		}
		set("radarProvider", jsString(value))
		if strings.TrimSpace(*customTiles) != "" {
			set("radarCustomTiles", jsString(*customTiles))
		} else {
			remove("radarCustomTiles")
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
		set("weatherProviders", "["+strings.Join(quoted, ", ")+"]")
		remove("weatherProviderKeys", "apiKey")
		if v := strings.TrimSuffix(strings.TrimSpace(*wxAPI), "/"); v != "" {
			set("wxApi", jsString(v))
		}
		if v := strings.TrimSuffix(strings.TrimSpace(*aqAPI), "/"); v != "" {
			set("aqApi", jsString(v))
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
		set("tempUnit", jsString(unit))
		set("windUnit", jsString(wind))
		set("weatherDays", strconv.Itoa(max(1, *weatherDays)))
		set("refreshWxMinutes", strconv.Itoa(max(1, *refreshWX)))
		set("weatherAlerts", "{ enabled: true, refreshMinutes: "+strconv.Itoa(max(1, *alertRefresh))+", minSeverity: "+jsString(*alertMin)+" }")
	default:
		fmt.Fprintln(os.Stderr, "--mode must be radar, weather, or display")
		return 64
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "refusing to edit config.local.js:", err)
		return 1
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
