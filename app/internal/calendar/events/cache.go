package events

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func (s *Service) refresh(force bool, daysPast int, daysFuture int) (map[string]any, error) {
	if daysPast < 0 {
		daysPast = 0
	}
	if daysFuture < 1 {
		daysFuture = 1
	}
	_ = os.MkdirAll(s.cacheDir, 0755)
	start, end := cacheWindow(s.now(), daysPast, daysFuture)
	cals := s.loadCalendars()
	sources := s.statSources(cals)
	fp := eventFingerprint(sources, start, end)
	cachePath := filepath.Join(s.cacheDir, "events.cache.json")
	metaPath := filepath.Join(s.cacheDir, ".events-cache.meta.json")
	oldMeta := jsonutil.Map(readJSONDefault(metaPath, map[string]any{}))
	oldCache := jsonutil.Map(readJSONDefault(cachePath, map[string]any{}))
	if !force && jsonutil.StringValue(oldMeta["fingerprint"]) == fp && jsonutil.Int(oldCache["version"], 0) == CacheVersion && anyInt64(oldCache["windowStart"], 0) <= epochMs(start) && anyInt64(oldCache["windowEnd"], 0) >= epochMs(end) {
		oldMeta["lastSuccessAt"] = s.now().UnixMilli()
		oldMeta["generator"] = "go"
		_ = fileio.WriteJSON(metaPath, oldMeta)
		return map[string]any{"ok": true, "unchanged": true, "eventCount": len(jsonutil.List(oldCache["events"])), "issues": jsonutil.List(oldCache["issues"]), "issueCount": len(jsonutil.List(oldCache["issues"])), "generator": "go"}, nil
	}
	out, err := s.buildCache(cals, sources, start, end)
	if err != nil {
		return nil, err
	}
	if err := writeCompactJSON(cachePath, out); err != nil {
		return nil, err
	}
	nowMs := s.now().UnixMilli()
	if err := fileio.WriteJSON(metaPath, map[string]any{"fingerprint": fp, "fingerprintVersion": FingerprintVersion, "updatedAt": nowMs, "lastSuccessAt": nowMs, "generator": "go"}); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "unchanged": false, "eventCount": len(out.Events), "issues": out.Issues, "issueCount": len(out.Issues), "windowStart": out.WindowStart, "windowEnd": out.WindowEnd, "generator": "go"}, nil
}

func cacheWindow(now time.Time, daysPast int, daysFuture int) (time.Time, time.Time) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	start := startOfWeek(today.AddDate(0, 0, -daysPast))
	end := startOfWeek(today.AddDate(0, 0, daysFuture)).AddDate(0, 0, 8)
	return start, end
}

func startOfWeek(t time.Time) time.Time {
	daysSinceSunday := int(t.Weekday())
	base := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	return base.AddDate(0, 0, -daysSinceSunday)
}

func (s *Service) buildCache(cals []CalendarSource, sources []SourceMeta, windowStart, windowEnd time.Time) (*CacheOutput, error) {
	allEvents := []map[string]any{}
	issues := []string{}
	idx := 0
	for _, cal := range cals {
		p := s.eventURLToPath(cal.URL)
		if p == "" || !fileio.Exists(p) {
			label := cal.Name
			if label == "" {
				label = cal.URL
			}
			if label == "" {
				label = "calendar"
			}
			issues = append(issues, label)
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			issues = append(issues, fmt.Sprintf("%s: %v", firstNonEmpty(cal.Name, cal.URL, "calendar"), err))
			continue
		}
		parsed := parseICS(string(b), cal)
		for _, ev := range parsed {
			for _, inst := range expand(ev, windowStart, windowEnd) {
				if eventInWindow(inst, windowStart, windowEnd) {
					allEvents = append(allEvents, serializeEvent(inst, cal, idx))
					idx++
				}
			}
		}
	}
	slices.SortStableFunc(allEvents, func(left, right map[string]any) int {
		leftStart, rightStart := anyInt64(left["start"], 0), anyInt64(right["start"], 0)
		if leftStart != rightStart {
			return compareInt64(leftStart, rightStart)
		}
		leftEnd, rightEnd := anyInt64(left["end"], leftStart), anyInt64(right["end"], rightStart)
		if leftEnd != rightEnd {
			return compareInt64(leftEnd, rightEnd)
		}
		return strings.Compare(fmt.Sprint(left["title"]), fmt.Sprint(right["title"]))
	})
	nowMs := s.now().UnixMilli()
	for i := range sources {
		if sources[i].MtimeMs != nil {
			age := float64(nowMs-*sources[i].MtimeMs) / 3600000.0
			age = float64(int(age*100+0.5)) / 100
			sources[i].AgeHours = &age
		}
	}
	return &CacheOutput{Version: CacheVersion, FingerprintVersion: FingerprintVersion, GeneratedAt: nowMs, WindowStart: epochMs(windowStart), WindowEnd: epochMs(windowEnd), Sources: sources, Issues: issues, Events: allEvents}, nil
}

func serializeEvent(ev ICSEvent, cal CalendarSource, idx int) map[string]any {
	uid := ev.UID
	startMs := epochMs(ev.Start)
	var endAny any = nil
	if ev.End != nil {
		endAny = epochMs(*ev.End)
	}
	h := sha1.Sum([]byte(cal.URL + "|" + uid + "|" + strconv.FormatInt(startMs, 10) + "|" + strconv.Itoa(idx)))
	ident := hex.EncodeToString(h[:])[:16]
	owner := strings.TrimSpace(ev.AppOwner)
	if owner == "" {
		owner = strings.TrimSpace(cal.Owner)
	}
	calMeta := map[string]any{"url": cal.URL, "name": cal.Name, "color": cal.Color, "tag": cal.Tag}
	if owner != "" {
		calMeta["owner"] = owner
	}
	item := map[string]any{"id": ident, "title": defaultString(ev.Title, "(no title)"), "desc": ev.Desc, "location": ev.Location, "start": startMs, "end": endAny, "allDay": ev.AllDay, "uid": uid, "calUrl": cal.URL, "cal": calMeta}
	if owner != "" {
		item["appOwner"] = owner
	}
	if kind := strings.TrimSpace(ev.Meta["X-DASHGO-MANAGED-SCHEDULE"]); kind != "" {
		item["managedSchedule"] = map[string]any{
			"type":        kind,
			"ruleId":      strings.TrimSpace(ev.Meta["X-DASHGO-SCHEDULE-RULE-ID"]),
			"nominalDate": strings.TrimSpace(ev.Meta["X-DASHGO-NOMINAL-DATE"]),
			"actualDate":  strings.TrimSpace(ev.Meta["X-DASHGO-SCHEDULE-ACTUAL-DATE"]),
			"reason":      strings.TrimSpace(ev.Meta["X-DASHGO-SCHEDULE-REASON"]),
		}
	}
	return item
}

func compareInt64(left, right int64) int {
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}
