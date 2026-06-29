package calendar

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	reISSLatitude  = regexp.MustCompile(`lat\s*:\s*([-0-9.]+)`)
	reISSLongitude = regexp.MustCompile(`lon\s*:\s*([-0-9.]+)`)
)

func (s *Service) UpdateISSPasses() map[string]any {
	if s == nil {
		return map[string]any{"ok": true, "enabled": false, "removed": true, "generator": "go"}
	}
	s.mu.Lock()
	values := s.DefaultConfig()
	destination := filepath.Join(s.calendarDir, "iss.slate.ics")
	if values["DEFAULT_ISS_PASSES"] != "1" {
		RemoveFile(destination)
		s.appendLog("iss-passes.log", fmt.Sprintf("%s: ISS passes disabled by %s\n", s.now().Format(time.ANSIC), filepath.Join(s.homeDir, ".dashboard-default-calendars")))
		s.mu.Unlock()
		return map[string]any{"ok": true, "enabled": false, "removed": true, "generator": "go"}
	}
	key := values["ISS_N2YO_API_KEY"]
	if key == "" {
		message := "ISS passes enabled but ISS_N2YO_API_KEY is blank; kept previous file"
		s.appendLog("iss-passes.log", s.now().Format(time.ANSIC)+": "+message+"\n")
		s.mu.Unlock()
		return map[string]any{"ok": true, "enabled": true, "skipped": true, "message": message, "generator": "go"}
	}
	latitude, longitude, err := ReadLatLon(s.configLocal)
	if err != nil {
		message := "ISS pass update skipped; location missing (" + err.Error() + ")"
		s.appendLog("iss-passes.log", s.now().Format(time.ANSIC)+": "+message+"\n")
		s.mu.Unlock()
		return map[string]any{"ok": true, "enabled": true, "skipped": true, "message": message, "generator": "go"}
	}
	days := atoiClamp(values["ISS_LOOKAHEAD_DAYS"], 7, 1, 10)
	minVisibility := atoiClamp(values["ISS_MIN_VISIBILITY"], 180, 1, 300)
	api := fmt.Sprintf("https://api.n2yo.com/rest/v1/satellite/visualpasses/25544/%.6f/%.6f/0/%d/%d/&apiKey=%s", latitude, longitude, days, minVisibility, url.QueryEscape(key))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, api, nil)
	client := &http.Client{Timeout: 30 * time.Second}
	if s.httpClient != nil && s.httpClient() != nil {
		client = s.httpClient()
	}
	response, err := client.Do(request)
	if err != nil {
		s.appendLog("iss-passes.log", s.now().Format(time.ANSIC)+": ISS pass fetch FAILED, kept previous file ("+err.Error()+")\n")
		s.mu.Unlock()
		return map[string]any{"ok": false, "error": err.Error(), "generator": "go"}
	}
	defer response.Body.Close()
	var data struct {
		Passes []map[string]any `json:"passes"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&data); err != nil {
		s.appendLog("iss-passes.log", s.now().Format(time.ANSIC)+": ISS pass fetch FAILED, bad payload\n")
		s.mu.Unlock()
		return map[string]any{"ok": false, "error": err.Error(), "generator": "go"}
	}
	events := []Event{}
	for _, pass := range data.Passes {
		start := int64(asFloat(pass["startUTC"]))
		if start <= 0 {
			continue
		}
		duration := int64(asFloat(pass["duration"]))
		if duration == 0 {
			duration = 60
		}
		if duration < 60 {
			duration = 60
		}
		begins, ends := time.Unix(start, 0).UTC(), time.Unix(start+duration, 0).UTC()
		bits := []string{}
		for _, item := range []struct{ key, label string }{{"maxEl", "Max elevation"}, {"mag", "Magnitude"}, {"startAzCompass", "Starts"}, {"endAzCompass", "Ends"}} {
			if value, ok := pass[item.key]; ok && fmt.Sprint(value) != "" {
				bits = append(bits, item.label+": "+fmt.Sprint(value))
			}
		}
		events = append(events, Event{Date: begins, Start: &begins, End: &ends, Summary: "ISS visible pass", Description: strings.Join(bits, "; "), UID: "iss"})
	}
	_ = WriteICSFile(destination, "ISS Visible Passes", events)
	s.appendLog("iss-passes.log", fmt.Sprintf("%s: ISS passes updated (%d events)\n", s.now().Format(time.ANSIC), len(events)))
	s.mu.Unlock()
	return map[string]any{"ok": true, "enabled": true, "eventCount": len(events), "generator": "go"}
}

func ReadLatLon(path string) (float64, float64, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, err
	}
	text := string(body)
	lat, lon := reISSLatitude.FindStringSubmatch(text), reISSLongitude.FindStringSubmatch(text)
	if len(lat) < 2 || len(lon) < 2 {
		return 0, 0, fmt.Errorf("lat/lon missing")
	}
	latitude, firstErr := strconv.ParseFloat(lat[1], 64)
	longitude, secondErr := strconv.ParseFloat(lon[1], 64)
	if firstErr != nil {
		return 0, 0, firstErr
	}
	if secondErr != nil {
		return 0, 0, secondErr
	}
	return latitude, longitude, nil
}

func asFloat(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		number, _ := typed.Float64()
		return number
	case string:
		number, _ := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return number
	default:
		return 0
	}
}
