package weather

import "time"

// weatherSourcesPayloadGo publishes normalized provider records to the browser.
// The browser owns the one authoritative blend because it already has the
// rendered-source context, physical clamps, robust per-field rejection, and
// timestamp-union hourly merge used by the dashboard. Keeping a second Go
// blend here would create a divergent forecast that no dashboard view uses.
//
// The top-level current/daily/hourly fields remain a compatibility mirror of
// the first successful source for cache validation and older consumers. They
// are intentionally not labelled or computed as a blend.
//
// There is deliberately no alerts field here. Dash-Go's severe-weather banners
// come from the browser's own NWS request; no Go provider ever populated this
// object, so publishing it only advertised data that could never arrive.
func weatherSourcesPayloadGo(sources []any, status []any, selected []string, cfg Config) map[string]any {
	fillDerivedSunTimesGo(sources, cfg)
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
		return out
	}

	primary := anyMap(sources[0])
	out["current"] = primary["current"]
	out["daily"] = primary["daily"]
	out["hourly"] = primary["hourly"]
	out["_source"] = primary["_source"]
	out["_sourceLabel"] = primary["_sourceLabel"]
	return out
}
