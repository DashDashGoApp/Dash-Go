package weather

import (
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// weatherSourcesPayloadGo publishes normalized provider records to the browser.
// The browser owns the one authoritative blend because it already has the
// rendered-source context, physical clamps, robust per-field rejection, and
// timestamp-union hourly merge used by the dashboard. Keeping a second Go
// blend here would create a divergent forecast that no dashboard view uses.
//
// The top-level current/daily/hourly fields remain a compatibility mirror of
// the first successful source for cache validation and older consumers. They
// are intentionally not labelled or computed as a blend.
func weatherSourcesPayloadGo(sources []any, status []any, selected []string) map[string]any {
	out := map[string]any{
		"sources":            sources,
		"status":             status,
		"sourceHealth":       status,
		"selected":           selected,
		"cache":              map[string]any{"hit": false, "generatedAt": time.Now().Unix()},
		"generator":          "go",
		"weatherBlend":       map[string]any{"ok": len(sources) > 0, "method": "browser-authoritative normalized sources", "sourceCount": len(sources), "generator": "go"},
		"goWeatherProviders": []string{"openmeteo", "openmeteo-custom", "nws", "weatherapi", "openweather", "googleweather", "tomorrow", "visualcrossing", "weatherbit", "pirateweather", "accuweather", "xweather"},
		"keysInServedConfig": false,
	}
	if len(sources) == 0 {
		out["current"] = nil
		out["daily"] = map[string][]any{}
		out["hourly"] = map[string][]any{}
		out["alerts"] = []any{}
		return out
	}

	primary := anyMap(sources[0])
	out["current"] = primary["current"]
	out["daily"] = primary["daily"]
	out["hourly"] = primary["hourly"]
	out["_source"] = primary["_source"]
	out["_sourceLabel"] = primary["_sourceLabel"]
	out["alerts"] = mergeAlertsGo(sources)
	return out
}

func mergeAlertsGo(sources []any) []any {
	out := []any{}
	seen := map[string]bool{}
	for _, src := range sources {
		for _, raw := range jsonutil.List(anyMap(src)["alerts"]) {
			key := jsonutil.TextValue(raw)
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, raw)
		}
	}
	return out
}
