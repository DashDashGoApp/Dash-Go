package maps

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// eventMapLookup coordinates local cache lookup, deterministic query variants,
// and provider selection. Cache/schema presentation details live beside their
// storage helpers; input cleanup and external providers have their own files.
func mapQueryKey(q string) string {
	q = strings.ToLower(strings.TrimSpace(q))
	q = reWhitespace.ReplaceAllString(q, " ")
	if len(q) > 160 {
		q = q[:160]
	}
	return q
}

func anyFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f
	}
	return 0
}

func (s *Service) eventMapLookup(q string) map[string]any {
	rawQ := strings.TrimSpace(q)
	key := mapQueryKey(rawQ)
	if len(key) < 3 {
		return map[string]any{"ok": false, "error": "missing location"}
	}
	if hit := s.geocodeMemoGet(key); hit != nil {
		return hit
	}
	cache := map[string]any{}
	if m, ok := s.readJSONDefault(s.mapCacheFile(), map[string]any{}).(map[string]any); ok {
		cache = m
	}
	if hit, ok := s.cachedEventMap(cache, key); ok {
		s.geocodeMemoPut(key, hit)
		return hit
	}
	variants := eventMapQueryVariants(rawQ)
	rows, usedQuery, geocoder, errors := s.geocodeEventLocationVariants(variants)
	if len(rows) == 0 {
		data := map[string]any{"ok": false, "error": "location not found", "tried": variants, "geocoderErrors": tailStrings(errors, 6)}
		cache[key] = map[string]any{"ts": time.Now().Unix(), "version": mapLookupVersion, "data": data}
		_ = fileio.WriteJSON(s.mapCacheFile(), cache)
		return data
	}
	m := jsonutil.Map(rows[0])
	lat, lon := anyFloat(m["lat"]), anyFloat(m["lon"])
	if lat == 0 && lon == 0 {
		return map[string]any{"ok": false, "error": "bad map result"}
	}
	label := strOr(m["display_name"], "")
	if label == "" {
		label = strOr(m["label"], "")
	}
	if label == "" {
		label = defaultString(usedQuery, rawQ)
	}
	osm := fmt.Sprintf("https://www.openstreetmap.org/?mlat=%.6f&mlon=%.6f#map=16/%.6f/%.6f", lat, lon, lat, lon)
	data := map[string]any{"ok": true, "lat": lat, "lon": lon, "label": label, "queryUsed": defaultString(usedQuery, variants[0]), "geocoder": geocoder, "osmUrl": osm, "cached": false}
	data = s.decorateMapData(data)
	cache[key] = map[string]any{"ts": time.Now().Unix(), "version": mapLookupVersion, "data": data}
	mapCachePrune(cache)
	_ = fileio.WriteJSON(s.mapCacheFile(), cache)
	s.geocodeMemoPut(key, data)
	return data
}

// mapCachePrune keeps the geocode cache at 200 entries, evicting the oldest
// first. Split out so every write path shares one policy.
func mapCachePrune(cache map[string]any) {
	if len(cache) <= 200 {
		return
	}
	type kv struct {
		k  string
		ts int
	}
	items := []kv{}
	for k, v := range cache {
		items = append(items, kv{k: k, ts: jsonutil.Int(jsonutil.Map(v)["ts"], 0)})
	}
	slices.SortFunc(items, func(left, right kv) int { return compareInts(left.ts, right.ts) })
	keep := map[string]bool{}
	for _, it := range items[len(items)-200:] {
		keep[it.k] = true
	}
	for k := range cache {
		if !keep[k] {
			delete(cache, k)
		}
	}
}

// geocodeMemoGet returns an in-memory hit only when the on-disk cache file has
// not changed since the memo was filled; a stale memo is dropped eagerly.
func (s *Service) geocodeMemoGet(key string) map[string]any {
	s.geocodeMemoMu.Lock()
	defer s.geocodeMemoMu.Unlock()
	if s.geocodeMemo == nil || len(s.geocodeMemo) == 0 {
		return nil
	}
	if info, err := os.Stat(s.mapCacheFile()); err != nil || !info.ModTime().Equal(s.geocodeMemoMtime) {
		s.geocodeMemo = nil
		s.geocodeMemoMtime = time.Time{}
		return nil
	}
	hit := s.geocodeMemo[key]
	if hit == nil {
		return nil
	}
	out := jsonutil.CloneMap(hit)
	out["cached"] = true
	return out
}

// geocodeMemoPut records a successful lookup only; negative results stay on
// their on-disk TTL so their 24h retry window is unchanged.
func (s *Service) geocodeMemoPut(key string, data map[string]any) {
	if data == nil || data["ok"] != true {
		return
	}
	s.geocodeMemoMu.Lock()
	defer s.geocodeMemoMu.Unlock()
	info, err := os.Stat(s.mapCacheFile())
	if err != nil {
		return
	}
	if s.geocodeMemo == nil || !info.ModTime().Equal(s.geocodeMemoMtime) {
		s.geocodeMemo = map[string]map[string]any{}
		s.geocodeMemoMtime = info.ModTime()
	}
	s.geocodeMemo[key] = jsonutil.CloneMap(data)
	if len(s.geocodeMemo) > 200 {
		for k := range s.geocodeMemo {
			delete(s.geocodeMemo, k)
			if len(s.geocodeMemo) <= 200 {
				break
			}
		}
	}
}

func tailStrings(in []string, n int) []string {
	if len(in) <= n {
		return in
	}
	return in[len(in)-n:]
}
