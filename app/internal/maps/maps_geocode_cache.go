package maps

import (
	"fmt"
	"strconv"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// decorateMapData keeps map responses stable regardless of whether their
// coordinates came from a fresh provider lookup, a current cache entry, or a
// legacy flat cache entry.
func (s *Service) decorateMapData(data map[string]any) map[string]any {
	if data == nil || data["ok"] != true {
		return data
	}
	data["defaultZoom"] = mapDefaultZoom
	data["defaultStyle"] = "standard"
	data["zoomLevels"] = []int{13, 15, 17}
	data["mapStyles"] = []map[string]string{{"name": "standard", "label": "Standard"}, {"name": "hybrid", "label": "Hybrid"}}
	s.ensureMapURLs(data)
	return data
}

func (s *Service) cachedEventMap(cache map[string]any, key string) (map[string]any, bool) {
	raw, ok := cache[key]
	if !ok {
		return nil, false
	}
	now := time.Now().Unix()
	entry := jsonutil.Map(raw)
	if data := jsonutil.Map(entry["data"]); len(data) > 0 {
		age := now - int64(jsonutil.Int(entry["ts"], 0))
		ver := jsonutil.Int(entry["version"], 0)
		if data["ok"] == true && age < 180*86400 {
			out := jsonutil.CloneMap(data)
			out["cached"] = true
			return s.decorateMapData(out), true
		}
		if data["ok"] != true && ver >= mapLookupVersion && age < 24*3600 {
			out := jsonutil.CloneMap(data)
			out["cached"] = true
			return out, true
		}
		return nil, false
	}
	// beta.21/22 Go cache entries were written flat. Preserve them, but
	// decorate so old callers get the same styleStaticUrls/default metadata.
	if entry["ok"] == true && (anyFloat(entry["lat"]) != 0 || anyFloat(entry["lon"]) != 0) {
		out := jsonutil.CloneMap(entry)
		out["cached"] = true
		return s.decorateMapData(out), true
	}
	return nil, false
}
func (s *Service) ensureMapURLs(m map[string]any) {
	lat, lon := anyFloat(m["lat"]), anyFloat(m["lon"])
	styles := map[string]any{}
	for _, st := range []string{"standard", "hybrid"} {
		by := map[string]string{}
		for _, z := range []int{13, 15, 17} {
			by[strconv.Itoa(z)] = fmt.Sprintf("/api/event-map-img?lat=%.6f&lon=%.6f&z=%d&style=%s", lat, lon, z, st)
		}
		styles[st] = by
	}
	m["styleStaticUrls"] = styles
	m["staticUrl"] = fmt.Sprintf("/api/event-map-img?lat=%.6f&lon=%.6f&z=15&style=standard", lat, lon)
}
