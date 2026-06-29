package platform

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func DoctorDataFinding(code, detail string) string {
	detail = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(detail, "\r", " "), "\n", " "))
	if detail == "" {
		return code
	}
	return code + ":" + detail
}
func DoctorDataJSON(path string) (any, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	var value any
	if e := json.Unmarshal(b, &value); e != nil {
		return nil, e
	}
	return value, nil
}
func DoctorDataNumber(value any) bool {
	switch value.(type) {
	case float64, float32, int, int64, int32, uint, uint64, uint32:
		return true
	default:
		return false
	}
}

func containsPrefix(findings []string, prefix string) bool {
	for _, finding := range findings {
		if strings.HasPrefix(finding, prefix) {
			return true
		}
	}
	return false
}
func (s *Service) DoctorDataFindings() []string {
	findings := []string{}
	status := map[string]any{}
	if s.fontStatus != nil {
		status = s.fontStatus()
	}
	if status["present"] != true {
		missing := []string{}
		if raw, ok := status["missing"].([]string); ok {
			missing = raw
		}
		findings = append(findings, DoctorDataFinding("DASHBOARD_FONTS_MISSING", strings.Join(missing, ",")))
	} else {
		findings = append(findings, DoctorDataFinding("DASHBOARD_FONTS_OK", ""))
	}
	calendarPath := filepath.Join(s.calendarDir, "calendars.json")
	calendarRaw, e := DoctorDataJSON(calendarPath)
	if e != nil {
		if os.IsNotExist(e) {
			findings = append(findings, DoctorDataFinding("CALENDAR_MANIFEST_ABSENT", ""))
		} else {
			findings = append(findings, DoctorDataFinding("CALENDAR_MANIFEST_INVALID", e.Error()))
		}
	} else if calendars, ok := calendarRaw.([]any); !ok {
		findings = append(findings, DoctorDataFinding("CALENDAR_MANIFEST_INVALID", "top level must be an array"))
	} else {
		missing := 0
		for _, raw := range calendars {
			entry := jsonutil.Map(raw)
			url := strings.TrimSpace(jsonutil.TextValue(entry["url"]))
			if url == "" {
				findings = append(findings, DoctorDataFinding("CALENDAR_MANIFEST_INVALID", "calendar entry has no url"))
				continue
			}
			path := ""
			if s.eventURLToPath != nil {
				path = s.eventURLToPath(url)
			}
			if path == "" {
				continue
			}
			st, statErr := os.Stat(path)
			if statErr != nil || st.IsDir() {
				missing++
				findings = append(findings, DoctorDataFinding("CALENDAR_SOURCE_MISSING", url))
			}
		}
		if missing == 0 {
			findings = append(findings, DoctorDataFinding("CALENDAR_MANIFEST_OK", fmt.Sprintf("%d entries", len(calendars))))
		}
	}
	eventPath := filepath.Join(s.cacheDir, "events.cache.json")
	eventRaw, e := DoctorDataJSON(eventPath)
	if e != nil {
		if os.IsNotExist(e) {
			findings = append(findings, DoctorDataFinding("EVENT_CACHE_ABSENT", ""))
		} else {
			findings = append(findings, DoctorDataFinding("EVENT_CACHE_INVALID", e.Error()))
		}
	} else {
		cache, ok := eventRaw.(map[string]any)
		valid := ok && jsonutil.Int(cache["version"], 0) == s.eventCacheVersion
		if valid {
			_, eventsOK := cache["events"].([]any)
			generatedAt, generatedOK := cache["generatedAt"]
			start, startOK := cache["windowStart"]
			end, endOK := cache["windowEnd"]
			valid = eventsOK && generatedOK && DoctorDataNumber(generatedAt) && startOK && DoctorDataNumber(start) && endOK && DoctorDataNumber(end)
		}
		if valid {
			findings = append(findings, DoctorDataFinding("EVENT_CACHE_OK", ""))
		} else {
			findings = append(findings, DoctorDataFinding("EVENT_CACHE_INVALID", "schema or cache version does not match"))
		}
	}
	weatherPath := filepath.Join(s.cacheDir, "weather-cache.json")
	weatherRaw, e := DoctorDataJSON(weatherPath)
	if e != nil {
		if os.IsNotExist(e) {
			findings = append(findings, DoctorDataFinding("WEATHER_CACHE_ABSENT", ""))
		} else {
			findings = append(findings, DoctorDataFinding("WEATHER_CACHE_INVALID", e.Error()))
		}
	} else {
		payload, ok := weatherRaw.(map[string]any)
		location := jsonutil.Map(payload["location"])
		cache := jsonutil.Map(payload["cache"])
		_, latOK := location["lat"]
		_, lonOK := location["lon"]
		if !ok || location == nil || cache == nil || !latOK || !lonOK {
			findings = append(findings, DoctorDataFinding("WEATHER_CACHE_INVALID", "missing payload location or cache metadata"))
		} else if jsonutil.TextValue(payload["error"]) != "" {
			findings = append(findings, DoctorDataFinding("WEATHER_CACHE_ERROR_PAYLOAD", "last refresh reported a provider error"))
		} else if s.weatherDoctorFindings != nil {
			findings = append(findings, s.weatherDoctorFindings(payload, s.nowTime())...)
		}
		if !containsPrefix(findings, "WEATHER_CACHE_") {
			findings = append(findings, DoctorDataFinding("WEATHER_CACHE_OK", ""))
		}
	}
	messagePath := filepath.Join(s.configDir, "message-cache.json")
	messageRaw, e := DoctorDataJSON(messagePath)
	if e != nil {
		if os.IsNotExist(e) {
			findings = append(findings, DoctorDataFinding("MESSAGE_CACHE_ABSENT", ""))
		} else {
			findings = append(findings, DoctorDataFinding("MESSAGE_CACHE_INVALID", e.Error()))
		}
	} else {
		cache, ok := messageRaw.(map[string]any)
		_, itemsOK := cache["items"].([]any)
		generated, generatedOK := cache["generatedAt"]
		_, sourcesOK := cache["sources"].([]any)
		statusRaw, statusPresent := cache["sourceStatus"]
		statusOK := !statusPresent
		if statusPresent {
			_, statusOK = statusRaw.([]any)
		}
		if !ok || !itemsOK || !generatedOK || !DoctorDataNumber(generated) || !sourcesOK || !statusOK {
			findings = append(findings, DoctorDataFinding("MESSAGE_CACHE_INVALID", "schema does not match generated message cache"))
		} else {
			findings = append(findings, DoctorDataFinding("MESSAGE_CACHE_OK", ""))
		}
	}
	return findings
}
func (s *Service) RunDoctorDataCLI(args []string) int {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			fmt.Println("usage: dashboard-control-server --doctor-data")
			return 0
		}
	}
	for _, finding := range s.DoctorDataFindings() {
		fmt.Println(finding)
	}
	return 0
}
