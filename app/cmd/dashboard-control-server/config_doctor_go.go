package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func sanitizeConfigCity(v any) string {
	s := fmt.Sprint(v)
	s = strings.Map(func(r rune) rune {
		if r == '\\' || r == '"' || r < 32 {
			return -1
		}
		return r
	}, s)
	if len(s) > 80 {
		s = s[:80]
	}
	return s
}

func repairCommonConfigSyntaxText(text string) string {
	re := rePauseBeforeProfile
	return re.ReplaceAllString(text, `$1,$2`)
}

func configHasKnownSyntaxIssue(text string) bool {
	re := rePauseProfilePair
	return re.MatchString(text)
}

func valuePrefixPattern(key, value string) *regexp.Regexp {
	// Use Go regexp syntax without lookbehind or backreferences.
	k := regexp.QuoteMeta(key)
	obj := `(?:window\s*\.\s*)?(?:DASHBOARD_LOCAL|CONFIG|DASH_CONFIG)`
	prop := `(?:\.\s*` + k + `|\[\s*["']` + k + `["']\s*\])`
	assign := obj + `\s*` + prop + `\s*=\s*`
	objectKey := `(?:["']` + k + `["']|` + k + `)\s*:\s*`
	return regexp.MustCompile(`(?s)(` + objectKey + `|` + assign + `)(` + value + `)`)
}

func firstSubmatch(m []string) string {
	for i := len(m) - 1; i >= 0; i-- {
		if m[i] != "" {
			return m[i]
		}
	}
	return ""
}

func readConfigLocation(path string) map[string]any {
	b, err := os.ReadFile(path)
	if err != nil {
		return map[string]any{"ok": false, "lat": nil, "lon": nil, "city": ""}
	}
	txt := repairCommonConfigSyntaxText(string(b))
	num := func(key string) any {
		re := valuePrefixPattern(key, `([-+]?\d+(?:\.\d+)?)`)
		m := re.FindStringSubmatch(txt)
		if len(m) == 0 {
			return nil
		}
		f, err := strconv.ParseFloat(firstSubmatch(m[2:]), 64)
		if err != nil {
			return nil
		}
		return f
	}
	city := ""
	reCity := valuePrefixPattern("locationName", `(?:"([^"]*)"|'([^']*)')`)
	if m := reCity.FindStringSubmatch(txt); len(m) > 0 {
		city = firstSubmatch(m[3:])
	}
	return map[string]any{"ok": true, "lat": num("lat"), "lon": num("lon"), "city": city}
}

func writeConfigLocation(path string, lat, lon float64, city string) (map[string]any, error) {
	if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return nil, fmt.Errorf("lat/lon out of range")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config.local.js not found")
	}
	txt := repairCommonConfigSyntaxText(string(b))
	city = sanitizeConfigCity(city)
	replaceNum := func(src, key string, value float64) (string, bool) {
		re := valuePrefixPattern(key, `[-+]?\d+(?:\.\d+)?`)
		changed := false
		out := re.ReplaceAllStringFunc(src, func(s string) string {
			if changed {
				return s
			}
			m := re.FindStringSubmatch(s)
			if len(m) == 0 {
				return s
			}
			changed = true
			return m[1] + fmt.Sprintf("%.4f", value)
		})
		return out, changed
	}
	n1, n2, n3 := false, false, false
	txt, n1 = replaceNum(txt, "lat", lat)
	txt, n2 = replaceNum(txt, "lon", lon)
	if city != "" {
		re := valuePrefixPattern("locationName", `(?:"[^"]*"|'[^']*')`)
		changed := false
		js, _ := json.Marshal(city)
		txt = re.ReplaceAllStringFunc(txt, func(s string) string {
			if changed {
				return s
			}
			m := re.FindStringSubmatch(s)
			if len(m) == 0 {
				return s
			}
			changed = true
			return m[1] + string(js)
		})
		n3 = changed
	}
	if !n1 || !n2 || (city != "" && !n3) {
		insert := ""
		if !n1 {
			insert += fmt.Sprintf("  lat: %.4f,\n", lat)
		}
		if !n2 {
			insert += fmt.Sprintf("  lon: %.4f,\n", lon)
		}
		if city != "" && !n3 {
			js, _ := json.Marshal(city)
			insert += "  locationName: " + string(js) + ",\n"
		}
		objRe := reDashboardLocalObject
		if objRe.MatchString(txt) {
			txt = objRe.ReplaceAllString(txt, `${1}`+insert)
		} else {
			txt = strings.TrimRight(txt, "\r\n") + "\n\nwindow.DASHBOARD_LOCAL = window.DASHBOARD_LOCAL || {};\n"
			if !n1 {
				txt += fmt.Sprintf("window.DASHBOARD_LOCAL.lat = %.4f;\n", lat)
			}
			if !n2 {
				txt += fmt.Sprintf("window.DASHBOARD_LOCAL.lon = %.4f;\n", lon)
			}
			if city != "" && !n3 {
				js, _ := json.Marshal(city)
				txt += "window.DASHBOARD_LOCAL.locationName = " + string(js) + ";\n"
			}
		}
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(txt), 0644); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp, path); err != nil {
		return nil, err
	}
	return map[string]any{"lat": lat, "lon": lon, "city": city, "locationNameUpdated": city != ""}, nil
}

func (a *app) runDoctorConfigCLI(args []string) int {
	mode := "check"
	query := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--repair":
			mode = "repair"
		case "--location-check":
			mode = "location-check"
		case "--set-location":
			mode = "set-location"
			if i+1 < len(args) {
				query = args[i+1]
				i++
			}
		case "-h", "--help":
			fmt.Println("usage: dashboard-control-server --doctor-config [--repair|--location-check|--set-location QUERY]")
			return 0
		}
	}
	switch mode {
	case "repair":
		b, err := os.ReadFile(a.configLocal)
		if err != nil {
			fmt.Println("config.local.js not found:", a.configLocal)
			return 1
		}
		fixed := repairCommonConfigSyntaxText(string(b))
		if fixed == string(b) {
			fmt.Println("No known config syntax issue found.")
			return 0
		}
		bak := a.configLocal + ".doctor-bak-" + time.Now().Format("20060102-150405")
		_ = os.WriteFile(bak, b, 0644)
		if err := os.WriteFile(a.configLocal+".tmp", []byte(fixed), 0644); err != nil {
			fmt.Println(err)
			return 1
		}
		if err := os.Rename(a.configLocal+".tmp", a.configLocal); err != nil {
			fmt.Println(err)
			return 1
		}
		fmt.Println("Repaired config.local.js syntax; backup:", bak)
		return 0
	case "location-check":
		b, err := os.ReadFile(a.configLocal)
		if err != nil {
			fmt.Println("CONFIG_MISSING")
			return 0
		}
		if configHasKnownSyntaxIssue(string(b)) {
			fmt.Println("SYNTAX_KNOWN:missing comma between pauseWhileOpen and profile")
		}
		loc := readConfigLocation(a.configLocal)
		lat, lok := loc["lat"].(float64)
		lon, ook := loc["lon"].(float64)
		if !lok || !ook {
			fmt.Println("LOCATION_MISSING")
		} else {
			fmt.Printf("LOCATION_OK:%.6f,%.6f:%s\n", lat, lon, loc["city"])
			if lat > -0.000001 && lat < 0.000001 && lon > -0.000001 && lon < 0.000001 {
				fmt.Println("LOCATION_ZERO")
			}
		}
		cache := jsonutil.Map(a.readJSONDefault(filepath.Join(a.cacheDir, "weather-cache.json"), map[string]any{}))
		payload := jsonutil.Map(cache["payload"])
		cloc := jsonutil.Map(payload["location"])
		if cloc != nil && anyFloat(cloc["lat"]) > -0.000001 && anyFloat(cloc["lat"]) < 0.000001 && anyFloat(cloc["lon"]) > -0.000001 && anyFloat(cloc["lon"]) < 0.000001 {
			fmt.Println("WEATHERAPI_ZERO_CACHE")
		}
		return 0
	case "set-location":
		if strings.TrimSpace(query) == "" {
			fmt.Println("No location entered. Skipped.")
			return 0
		}
		res := a.eventMapLookup(query)
		if res["ok"] != true {
			fmt.Println("Geocode failed:", res["error"])
			return 1
		}
		loc, err := writeConfigLocation(a.configLocal, anyFloat(res["lat"]), anyFloat(res["lon"]), strOr(res["city"], strOr(res["label"], query)))
		if err != nil {
			fmt.Println("Write failed:", err)
			return 1
		}
		fmt.Printf("Location saved: %.4f, %.4f %s\n", loc["lat"], loc["lon"], loc["city"])
		return 0
	}
	return 0
}
