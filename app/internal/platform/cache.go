package platform

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func platformReadJSON(path string, fallback any) any {
	b, e := os.ReadFile(path)
	if e != nil {
		return fallback
	}
	var v any
	if json.Unmarshal(b, &v) != nil {
		return fallback
	}
	return v
}
func platformInt64(v any, def int64) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	case json.Number:
		if x, e := n.Int64(); e == nil {
			return x
		}
	}
	return int64(jsonutil.Int(v, int(def)))
}
func platformText(v any) string { return strings.TrimSpace(jsonutil.TextValue(v)) }
func platformFirst(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
func (s *Service) CacheStatus() map[string]any {
	cachePath := filepath.Join(s.cacheDir, "events.cache.json")
	metaPath := filepath.Join(s.cacheDir, ".events-cache.meta.json")
	cache := jsonutil.Map(platformReadJSON(cachePath, map[string]any{}))
	meta := jsonutil.Map(platformReadJSON(metaPath, map[string]any{}))
	st, statErr := os.Stat(cachePath)
	exists := statErr == nil && !st.IsDir()
	valid := exists && jsonutil.Int(cache["version"], 0) == s.eventCacheVersion && len(jsonutil.List(cache["events"])) >= 0
	generated := platformInt64(cache["generatedAt"], 0)
	sources := jsonutil.List(cache["sources"])
	latest := int64(0)
	for _, raw := range sources {
		m := jsonutil.Map(raw)
		if mt := platformInt64(m["mtimeMs"], 0); mt > latest {
			latest = mt
		}
	}
	stale := valid && generated > 0 && latest > generated+1000
	eventCounts := map[string]int{}
	for _, raw := range jsonutil.List(cache["events"]) {
		ev := jsonutil.Map(raw)
		cal := jsonutil.Map(ev["cal"])
		key := strings.ToLower(strings.TrimSpace(platformFirst(platformText(cal["url"]), platformText(cal["name"]))))
		if key != "" {
			eventCounts[key]++
		}
	}
	sourceByURL := map[string]map[string]any{}
	for _, raw := range sources {
		m := jsonutil.Map(raw)
		key := strings.ToLower(strings.TrimSpace(platformText(m["url"])))
		if key != "" {
			sourceByURL[key] = m
		}
	}
	issues := []any{}
	if valid {
		issues = jsonutil.List(cache["issues"])
	}
	rows := []map[string]any{}
	summary := map[string]any{"total": 0, "enabled": 0, "hidden": 0, "healthy": 0, "check": 0, "action": 0, "info": 0}
	nowMs := s.nowTime().UnixMilli()
	for _, raw := range s.getCalendarEntries() {
		c := jsonutil.Map(raw)
		urlv := platformText(c["url"])
		p := ""
		if s.eventURLToPath != nil {
			p = s.eventURLToPath(urlv)
		} else if urlv != "" {
			p = filepath.Join(s.dashDir, strings.TrimPrefix(urlv, "/"))
		}
		mtime, size := int64(0), int64(0)
		existsFile := false
		if p != "" {
			if st, e := os.Stat(p); e == nil && !st.IsDir() {
				existsFile = true
				mtime = st.ModTime().UnixMilli()
				size = st.Size()
			}
		}
		source := sourceByURL[strings.ToLower(urlv)]
		if source == nil {
			source = map[string]any{}
		}
		realPath := platformText(source["realPath"])
		if realPath == "" && p != "" {
			realPath = p
			if rp, e := filepath.EvalSymlinks(p); e == nil {
				realPath = rp
			}
		}
		isSymlink := jsonutil.Truthy(source["isSymlink"])
		if p != "" && realPath != "" && filepath.Clean(p) != filepath.Clean(realPath) {
			isSymlink = true
		}
		keyURL, keyName := strings.ToLower(urlv), strings.ToLower(platformText(c["name"]))
		cnt := eventCounts[keyURL]
		if cnt == 0 {
			cnt = eventCounts[keyName]
		}
		matching := []any{}
		for _, issue := range issues {
			txt := platformText(issue)
			low := strings.ToLower(txt)
			if (keyName != "" && strings.HasPrefix(low, keyName)) || (keyURL != "" && strings.Contains(low, keyURL)) {
				matching = append(matching, txt)
			}
		}
		fresh := generated > 0 && (mtime == 0 || mtime <= generated+1000)
		problems := []any{}
		level, label := "healthy", "Healthy"
		enabled := true
		if v, ok := c["enabled"]; ok {
			enabled = jsonutil.Truthy(v)
		}
		if !enabled {
			level, label = "info", "Hidden"
			problems = append(problems, "Hidden in Dashboard Control")
		}
		if !existsFile {
			level, label = "action", "Missing file"
			problems = append(problems, "The .ics file is missing")
		} else if size == 0 {
			level, label = "action", "Empty file"
			problems = append(problems, "The .ics file is empty")
		} else if valid && !fresh {
			if level != "action" {
				level, label = "check", "Cache stale"
			}
			problems = append(problems, "File changed after the event cache was generated")
		} else if !valid {
			if level != "action" {
				level, label = "check", "Cache unavailable"
			}
			problems = append(problems, "Event cache is missing or invalid")
		}
		if len(matching) > 0 {
			level, label = "action", "Parse issue"
			end := len(matching)
			if end > 3 {
				end = 3
			}
			problems = append(problems, matching[:end]...)
		}
		if level == "healthy" && cnt == 0 && enabled {
			label = "No visible events"
		}
		row := make(map[string]any, len(c))
		maps.Copy(row, c)
		row["mtimeMs"] = mtime
		row["size"] = size
		row["events"] = cnt
		row["exists"] = existsFile
		row["cacheFresh"] = fresh
		row["source"] = source
		row["ageHours"] = nil
		if mtime > 0 {
			row["ageHours"] = float64(nowMs-mtime) / 3600000.0
		}
		row["realPath"] = realPath
		row["isSymlink"] = isSymlink
		row["contentSha256"] = source["sha256"]
		row["level"] = level
		row["label"] = label
		row["problems"] = problems
		rows = append(rows, row)
		summary["total"] = jsonutil.Int(summary["total"], 0) + 1
		if enabled {
			summary["enabled"] = jsonutil.Int(summary["enabled"], 0) + 1
		} else {
			summary["hidden"] = jsonutil.Int(summary["hidden"], 0) + 1
		}
		summary[level] = jsonutil.Int(summary[level], 0) + 1
	}
	if jsonutil.Int(summary["action"], 0) > 0 {
		summary["level"], summary["label"] = "action", "Action needed"
	} else if jsonutil.Int(summary["check"], 0) > 0 {
		summary["level"], summary["label"] = "check", "Check soon"
	} else if jsonutil.Int(summary["total"], 0) > 0 {
		summary["level"], summary["label"] = "healthy", "Healthy"
	} else {
		summary["level"], summary["label"] = "check", "No calendars"
	}
	return map[string]any{"exists": exists, "valid": valid, "using": valid && !stale, "stale": stale, "generatedAt": ternaryBool(valid, generated, int64(0)), "windowStart": cache["windowStart"], "windowEnd": cache["windowEnd"], "eventCount": len(jsonutil.List(cache["events"])), "issues": issues, "sources": sources, "calendars": rows, "calendarSummary": summary, "metaUpdatedAt": meta["updatedAt"], "path": cachePath}
}
func ternaryBool[T any](condition bool, yes, no T) T {
	if condition {
		return yes
	}
	return no
}
