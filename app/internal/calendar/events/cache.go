package events

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

type cacheMetadata struct {
	Fingerprint        string       `json:"fingerprint"`
	FingerprintVersion int          `json:"fingerprintVersion"`
	UpdatedAt          int64        `json:"updatedAt"`
	LastSuccessAt      int64        `json:"lastSuccessAt"`
	Generator          string       `json:"generator"`
	CacheVersion       int          `json:"cacheVersion,omitempty"`
	WindowStart        int64        `json:"windowStart,omitempty"`
	WindowEnd          int64        `json:"windowEnd,omitempty"`
	EventCount         int          `json:"eventCount,omitempty"`
	Issues             []string     `json:"issues,omitempty"`
	CacheSize          int64        `json:"cacheSize,omitempty"`
	CacheMtimeNs       int64        `json:"cacheMtimeNs,omitempty"`
	Sources            []SourceMeta `json:"sources,omitempty"`
}

func readCacheMetadata(path string) (cacheMetadata, bool) {
	body, err := os.ReadFile(path)
	if err != nil {
		return cacheMetadata{}, false
	}
	var meta cacheMetadata
	if err := json.Unmarshal(body, &meta); err != nil {
		return cacheMetadata{}, false
	}
	return meta, true
}

func cacheMetadataMatchesFile(path string, meta cacheMetadata) bool {
	if meta.CacheVersion != CacheVersion || meta.CacheSize <= 0 || meta.CacheMtimeNs <= 0 {
		return false
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	return info.Size() == meta.CacheSize && info.ModTime().UnixNano() == meta.CacheMtimeNs
}

func cacheMetadataCovers(meta cacheMetadata, fingerprint string, windowStart, windowEnd time.Time) bool {
	return meta.Fingerprint == fingerprint &&
		meta.FingerprintVersion == FingerprintVersion &&
		meta.CacheVersion == CacheVersion &&
		meta.WindowStart <= epochMs(windowStart) &&
		meta.WindowEnd >= epochMs(windowEnd)
}

func unchangedCacheResult(eventCount int, issues []string) map[string]any {
	if issues == nil {
		issues = []string{}
	}
	return map[string]any{
		"ok":         true,
		"unchanged":  true,
		"eventCount": eventCount,
		"issues":     issues,
		"issueCount": len(issues),
		"generator":  "go",
	}
}

func cacheMetadataFromOutput(fingerprint string, out *CacheOutput, info os.FileInfo, nowMs int64) cacheMetadata {
	return cacheMetadata{
		Fingerprint:        fingerprint,
		FingerprintVersion: FingerprintVersion,
		UpdatedAt:          nowMs,
		LastSuccessAt:      nowMs,
		Generator:          "go",
		CacheVersion:       out.Version,
		WindowStart:        out.WindowStart,
		WindowEnd:          out.WindowEnd,
		EventCount:         len(out.Events),
		Issues:             append([]string(nil), out.Issues...),
		CacheSize:          info.Size(),
		CacheMtimeNs:       info.ModTime().UnixNano(),
		Sources:            append([]SourceMeta(nil), out.Sources...),
	}
}

func cacheMetadataFromExisting(fingerprint string, oldMeta cacheMetadata, oldCache map[string]any, info os.FileInfo, nowMs int64) cacheMetadata {
	updatedAt := oldMeta.UpdatedAt
	if updatedAt <= 0 {
		updatedAt = nowMs
	}
	issuesAny := jsonutil.List(oldCache["issues"])
	issues := make([]string, 0, len(issuesAny))
	for _, issue := range issuesAny {
		issues = append(issues, fmt.Sprint(issue))
	}
	var sources []SourceMeta
	if body, err := json.Marshal(oldCache["sources"]); err == nil {
		_ = json.Unmarshal(body, &sources)
	}
	return cacheMetadata{
		Fingerprint:        fingerprint,
		FingerprintVersion: FingerprintVersion,
		UpdatedAt:          updatedAt,
		LastSuccessAt:      nowMs,
		Generator:          "go",
		CacheVersion:       jsonutil.Int(oldCache["version"], 0),
		WindowStart:        anyInt64(oldCache["windowStart"], 0),
		WindowEnd:          anyInt64(oldCache["windowEnd"], 0),
		EventCount:         len(jsonutil.List(oldCache["events"])),
		Issues:             issues,
		CacheSize:          info.Size(),
		CacheMtimeNs:       info.ModTime().UnixNano(),
		Sources:            sources,
	}
}

func (s *Service) refresh(force bool, daysPast int, daysFuture int) (map[string]any, error) {
	waitedInProcess := !s.refreshMu.TryLock()
	if waitedInProcess {
		s.refreshMu.Lock()
		force = false
	}
	defer s.refreshMu.Unlock()
	if daysPast < 0 {
		daysPast = 0
	}
	if daysFuture < 1 {
		daysFuture = 1
	}
	_ = os.MkdirAll(s.cacheDir, 0755)
	releaseLock, waitedForProcess, err := acquireEventCacheLock(filepath.Join(s.cacheDir, ".events-cache.lock"))
	if err != nil {
		return nil, err
	}
	defer releaseLock()
	if waitedForProcess {
		force = false
	}
	start, end := cacheWindow(s.now(), daysPast, daysFuture, s.firstDayOfWeek())
	cachePath := filepath.Join(s.cacheDir, "events.cache.json")
	metaPath := filepath.Join(s.cacheDir, ".events-cache.meta.json")
	oldMeta, metaOK := readCacheMetadata(metaPath)
	cals := s.loadCalendars()
	sources := s.statSources(cals, oldMeta.Sources)
	fp := eventFingerprint(sources, start, end, s.capabilityFingerprint())

	if !force && metaOK && cacheMetadataCovers(oldMeta, fp, start, end) && cacheMetadataMatchesFile(cachePath, oldMeta) {
		oldMeta.LastSuccessAt = s.now().UnixMilli()
		oldMeta.Generator = "go"
		if err := fileio.WriteJSON(metaPath, oldMeta); err != nil {
			return nil, err
		}
		return unchangedCacheResult(oldMeta.EventCount, oldMeta.Issues), nil
	}

	// Metadata written before beta.5 has no cache summary/stat fields. Parse the
	// existing cache once, preserve the established unchanged result, and upgrade
	// the metadata so later no-op generations need only a stat check.
	oldCache := jsonutil.Map(readJSONDefault(cachePath, map[string]any{}))
	if !force && metaOK && oldMeta.Fingerprint == fp &&
		jsonutil.Int(oldCache["version"], 0) == CacheVersion &&
		anyInt64(oldCache["windowStart"], 0) <= epochMs(start) &&
		anyInt64(oldCache["windowEnd"], 0) >= epochMs(end) {
		info, err := os.Stat(cachePath)
		if err == nil && info.Mode().IsRegular() {
			nowMs := s.now().UnixMilli()
			upgraded := cacheMetadataFromExisting(fp, oldMeta, oldCache, info, nowMs)
			if err := fileio.WriteJSON(metaPath, upgraded); err != nil {
				return nil, err
			}
			return unchangedCacheResult(upgraded.EventCount, upgraded.Issues), nil
		}
	}

	out, err := s.buildCache(cals, sources, start, end)
	if err != nil {
		return nil, err
	}
	if err := fileio.WriteCompactJSON(cachePath, out); err != nil {
		return nil, err
	}
	info, err := os.Stat(cachePath)
	if err != nil {
		return nil, err
	}
	nowMs := s.now().UnixMilli()
	if err := fileio.WriteJSON(metaPath, cacheMetadataFromOutput(fp, out, info, nowMs)); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "unchanged": false, "eventCount": len(out.Events), "issues": out.Issues, "issueCount": len(out.Issues), "windowStart": out.WindowStart, "windowEnd": out.WindowEnd, "generator": "go"}, nil
}

func acquireEventCacheLock(path string) (func(), bool, error) {
	waited := false
	deadline := time.Now().Add(30 * time.Second)
	for {
		file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err == nil {
			_, _ = fmt.Fprintf(file, "%d %d\n", os.Getpid(), time.Now().Unix())
			_ = file.Close()
			return func() { _ = os.Remove(path) }, waited, nil
		}
		if !os.IsExist(err) {
			return func() {}, waited, err
		}
		waited = true
		if info, statErr := os.Stat(path); statErr == nil && time.Since(info.ModTime()) > 2*time.Minute {
			_ = os.Remove(path)
			continue
		}
		if time.Now().After(deadline) {
			return func() {}, waited, fmt.Errorf("timed out waiting for event-cache generator lock")
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func cacheWindow(now time.Time, daysPast int, daysFuture int, firstDay int) (time.Time, time.Time) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	start := startOfWeek(today.AddDate(0, 0, -daysPast), firstDay)
	end := startOfWeek(today.AddDate(0, 0, daysFuture), firstDay).AddDate(0, 0, 8)
	return start, end
}

func startOfWeek(t time.Time, firstDay int) time.Time {
	firstDay = ((firstDay % 7) + 7) % 7
	daysSince := (int(t.Weekday()) - firstDay + 7) % 7
	base := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	return base.AddDate(0, 0, -daysSince)
}

func (s *Service) buildCache(cals []CalendarSource, sources []SourceMeta, windowStart, windowEnd time.Time) (*CacheOutput, error) {
	allEvents := []map[string]any{}
	issues := []string{}
	diagnostics := []SourceDiagnostics{}
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
			diagnostics = append(diagnostics, SourceDiagnostics{URL: cal.URL, Name: cal.Name, Error: "calendar source is unavailable"})
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			issues = append(issues, fmt.Sprintf("%s: %v", firstNonEmpty(cal.Name, cal.URL, "calendar"), err))
			diagnostics = append(diagnostics, SourceDiagnostics{URL: cal.URL, Name: cal.Name, Error: err.Error()})
			continue
		}
		parseStarted := time.Now()
		parsed, parsedDiagnostics := parseICSWithDiagnostics(string(b), cal)
		diagnostic := SourceDiagnostics{URL: cal.URL, Name: cal.Name, ComponentsFound: parsedDiagnostics.ComponentsFound, EventsAccepted: parsedDiagnostics.EventsAccepted, InvalidEventsDropped: parsedDiagnostics.InvalidEventsDropped, CancelledEvents: parsedDiagnostics.CancelledEvents, SupersededRevisions: parsedDiagnostics.SupersededRevisions}
		for _, ev := range parsed {
			for _, inst := range expand(ev, windowStart, windowEnd) {
				if eventInWindow(inst, windowStart, windowEnd) {
					allEvents = append(allEvents, s.serializeEvent(inst, cal))
					diagnostic.ExpandedOccurrences++
				}
			}
		}
		diagnostic.ParseDurationMs = time.Since(parseStarted).Milliseconds()
		diagnostics = append(diagnostics, diagnostic)
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
	return &CacheOutput{Version: CacheVersion, FingerprintVersion: FingerprintVersion, GeneratedAt: nowMs, WindowStart: epochMs(windowStart), WindowEnd: epochMs(windowEnd), Sources: sources, Issues: issues, Diagnostics: diagnostics, Events: allEvents}, nil
}
