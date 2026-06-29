package maps

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func (s *Service) startMapPrewarm(body map[string]any) map[string]any {
	limit := clamp(jsonutil.Int(body["limit"], 24), 1, 100)
	res := s.prewarmEventMaps(limit)
	return res
}

func (s *Service) runMapPrewarmCLI(args []string) int {
	limit := 24
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--limit":
			if i+1 < len(args) {
				if n, err := strconv.Atoi(args[i+1]); err == nil {
					limit = n
				}
				i++
			}
		case "-h", "--help":
			fmt.Println("usage: dashboard-control-server --maps-prewarm [--limit N]")
			return 0
		}
	}
	res := s.prewarmEventMaps(limit)
	if res["ok"] != true {
		fmt.Fprintln(os.Stderr, "map prewarm failed")
		return 1
	}
	fmt.Printf("map prewarm resolved %v/%v locations and wrote %v images\n", res["resolved"], res["candidateCount"], res["imagesWritten"])
	return 0
}

func (s *Service) prewarmEventMaps(limit int) map[string]any {
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}
	_ = os.MkdirAll(s.mapImageDir(), 0755)
	_ = os.MkdirAll(s.logDir, 0755)
	started := time.Now()
	statePath := filepath.Join(s.cacheDir, "map-prewarm-state.json")
	_ = fileio.WriteJSON(statePath, map[string]any{"running": true, "lastStart": started.Unix(), "limit": limit, "goServer": true})

	candidates := s.mapPrewarmCandidates(limit)
	resolved := 0
	imagesWritten := 0
	errors := []string{}
	for _, cand := range candidates {
		lat, lon := anyFloat(cand["lat"]), anyFloat(cand["lon"])
		if lat == 0 && lon == 0 {
			q := jsonutil.StringValue(cand["query"])
			if q == "" {
				continue
			}
			geo := s.eventMapLookup(q)
			if geo["ok"] != true {
				errors = append(errors, q+": "+fmt.Sprint(geo["error"]))
				continue
			}
			lat, lon = anyFloat(geo["lat"]), anyFloat(geo["lon"])
		}
		if lat < -90 || lat > 90 || lon < -180 || lon > 180 || (lat == 0 && lon == 0) {
			continue
		}
		resolved++
		for _, style := range []string{"standard", "hybrid"} {
			for _, z := range []int{13, 15, 17} {
				if s.ensureStaticMapImage(lat, lon, z, style) {
					imagesWritten++
				}
			}
		}
	}
	state := map[string]any{"ok": true, "running": false, "started": false, "lastStart": started.Unix(), "lastFinish": time.Now().Unix(), "limit": limit, "candidateCount": len(candidates), "resolved": resolved, "imagesWritten": imagesWritten, "errors": errors, "errorCount": len(errors), "goServer": true, "generator": "go"}
	_ = fileio.WriteJSON(statePath, state)
	var log strings.Builder
	log.WriteString(started.Format(time.RFC3339) + " Go map prewarm scanned event-cache locations\n")
	log.WriteString(fmt.Sprintf("candidateCount=%d resolved=%d imagesWritten=%d errors=%d\n", len(candidates), resolved, imagesWritten, len(errors)))
	for _, e := range errors {
		log.WriteString("WARN " + e + "\n")
	}
	_ = os.WriteFile(filepath.Join(s.logDir, "map-prewarm.log"), []byte(log.String()), 0644)
	state["status"] = s.mapCacheStatusWithCleanup(true)
	return state
}

func (s *Service) mapPrewarmCandidates(limit int) []map[string]any {
	seen := map[string]bool{}
	out := []map[string]any{}
	cache := map[string]any{}
	if m, ok := s.readJSONDefault(s.mapCacheFile(), map[string]any{}).(map[string]any); ok {
		cache = m
	}
	add := func(query, label string, lat, lon float64) {
		query = strings.TrimSpace(query)
		label = strings.TrimSpace(label)
		if query == "" || len(query) < 3 || len(out) >= limit {
			return
		}
		key := mapQueryKey(query)
		if key == "" || seen[key] {
			return
		}
		seen[key] = true
		item := map[string]any{"query": query, "label": defaultString(label, query)}
		if lat != 0 || lon != 0 {
			item["lat"] = lat
			item["lon"] = lon
		} else if old, ok := cache[key].(map[string]any); ok {
			data := jsonutil.Map(old["data"])
			if len(data) == 0 {
				data = old
			}
			item["lat"] = anyFloat(data["lat"])
			item["lon"] = anyFloat(data["lon"])
			item["cached"] = true
		}
		out = append(out, item)
	}

	settings := s.loadSettings()
	lat, lon := anyFloat(settings["lat"]), anyFloat(settings["lon"])
	if lat != 0 || lon != 0 {
		label := strOr(settings["locationName"], "Dashboard location")
		add(label, label, lat, lon)
	}

	cacheData := jsonutil.Map(s.readJSONDefault(filepath.Join(s.cacheDir, "events.cache.json"), map[string]any{}))
	for _, raw := range jsonutil.List(cacheData["events"]) {
		ev := jsonutil.Map(raw)
		loc := jsonutil.StringValue(ev["location"])
		if loc == "" {
			continue
		}
		// Avoid trying to geocode video links or call-in details that commonly live in LOCATION fields.
		low := strings.ToLower(loc)
		if strings.Contains(low, "http://") || strings.Contains(low, "https://") || strings.Contains(low, "zoom") || strings.Contains(low, "meet.google") || strings.Contains(low, "teams.microsoft") {
			continue
		}
		add(loc, strOr(ev["title"], loc), 0, 0)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func (s *Service) ensureStaticMapImage(lat, lon float64, z int, style string) bool {
	style = normMapStyle(style)
	if z < 1 || z > 18 {
		z = mapDefaultZoom
	}
	if p, _ := s.cachedMapImagePath(lat, lon, z, style); p != "" {
		return false
	}
	p, _ := s.fetchMapImage(lat, lon, z, style, false)
	return p != ""
}
