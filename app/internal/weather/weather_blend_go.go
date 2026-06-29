package weather

import (
	"maps"
	"math"
	"slices"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// blendWeatherSourcesGo creates the server-side blended weather view that the
// browser expects. It intentionally keeps each raw provider in sources[] while
// exposing blended current/daily/hourly fields at the top level so the dashboard
// no longer has to do browser-side provider selection work.
func blendWeatherSourcesGo(sources []any, status []any, selected []string, cfg Config) map[string]any {
	out := map[string]any{
		"sources":            sources,
		"status":             status,
		"sourceHealth":       status,
		"selected":           selected,
		"cache":              map[string]any{"hit": false, "generatedAt": time.Now().Unix()},
		"generator":          "go",
		"weatherBlend":       map[string]any{"ok": len(sources) > 0, "method": "median-trimmed mean", "sourceCount": len(sources), "generator": "go"},
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
	out["current"] = blendCurrentGo(sources)
	out["daily"] = blendDailyGo(sources, cfg.Days)
	out["hourly"] = blendHourlyGo(sources)
	out["alerts"] = mergeAlertsGo(sources)
	out["_source"] = "blend"
	out["_sourceLabel"] = "Blended forecast"
	return out
}

func blendCurrentGo(sources []any) map[string]any {
	keys := []string{"temperature_2m", "apparent_temperature", "wind_speed_10m", "relative_humidity_2m"}
	out := map[string]any{"_source": "blend", "_sourceLabel": "Blended current"}
	for _, k := range keys {
		vals := []float64{}
		for _, src := range sources {
			cur := anyMap(anyMap(src)["current"])
			if len(cur) == 0 {
				continue
			}
			if v, ok := toFloatGo(cur[k]); ok {
				vals = append(vals, v)
			}
		}
		out[k] = robustAverageGo(vals)
	}
	out["weather_code"] = modalWeatherCodeGo(sources, -1, "current")
	return out
}

func blendDailyGo(sources []any, days int) map[string][]any {
	if days <= 0 {
		days = 14
	}
	dateOrder := []string{}
	seen := map[string]bool{}
	for _, src := range sources {
		d := anyMap(anyMap(src)["daily"])
		for _, t := range jsonutil.List(d["time"]) {
			ds := jsonutil.TextValue(t)
			if ds == "" || seen[ds] {
				continue
			}
			seen[ds] = true
			dateOrder = append(dateOrder, ds)
		}
	}
	slices.Sort(dateOrder)
	if len(dateOrder) > days {
		dateOrder = dateOrder[:days]
	}
	fields := []string{"temperature_2m_max", "temperature_2m_min", "apparent_temperature_max", "precipitation_sum", "precipitation_probability_max", "wind_speed_10m_max", "uv_index_max"}
	out := emptyDailyGo()
	for _, date := range dateOrder {
		out["time"] = append(out["time"], date)
		for _, f := range fields {
			vals := []float64{}
			for _, src := range sources {
				d := anyMap(anyMap(src)["daily"])
				if idx := indexOfAnyStringGo(jsonutil.List(d["time"]), date); idx >= 0 {
					if v, ok := listFloatAtGo(d[f], idx); ok {
						vals = append(vals, v)
					}
				}
			}
			out[f] = append(out[f], robustAverageGo(vals))
		}
		out["weather_code"] = append(out["weather_code"], modalWeatherCodeByDateGo(sources, date))
		out["sunrise"] = append(out["sunrise"], firstDailyValueByDateGo(sources, date, "sunrise"))
		out["sunset"] = append(out["sunset"], firstDailyValueByDateGo(sources, date, "sunset"))
	}
	return out
}

type hourlyBlendSourceGo struct {
	hourly map[string]any
	byTime map[string]int
}

func buildHourlyBlendSourcesGo(sources []any) ([]hourlyBlendSourceGo, map[string]any) {
	indexed := make([]hourlyBlendSourceGo, 0, len(sources))
	first := map[string]any{}
	bestCount := -1
	for _, src := range sources {
		h := anyMap(anyMap(src)["hourly"])
		times := jsonutil.List(h["time"])
		byTime := make(map[string]int, len(times))
		for i, raw := range times {
			ts := jsonutil.TextValue(raw)
			if ts == "" {
				continue
			}
			if _, exists := byTime[ts]; !exists {
				byTime[ts] = i
			}
		}
		indexed = append(indexed, hourlyBlendSourceGo{hourly: h, byTime: byTime})
		if len(times) > bestCount {
			bestCount = len(times)
			first = h
		}
	}
	return indexed, first
}

func blendHourlyGo(sources []any) map[string][]any {
	indexed, first := buildHourlyBlendSourcesGo(sources)
	out := map[string][]any{"time": {}, "temperature_2m": {}, "weather_code": {}, "precipitation_probability": {}}
	for _, t := range jsonutil.List(first["time"]) {
		ts := jsonutil.TextValue(t)
		if ts == "" {
			continue
		}
		out["time"] = append(out["time"], ts)
		for _, f := range []string{"temperature_2m", "precipitation_probability"} {
			vals := []float64{}
			for _, src := range indexed {
				if idx, ok := src.byTime[ts]; ok {
					if v, ok := listFloatAtGo(src.hourly[f], idx); ok {
						vals = append(vals, v)
					}
				}
			}
			out[f] = append(out[f], robustAverageGo(vals))
		}
		out["weather_code"] = append(out["weather_code"], modalHourlyCodeByTimeIndexedGo(indexed, ts))
	}
	return out
}

func modalHourlyCodeByTimeIndexedGo(sources []hourlyBlendSourceGo, ts string) any {
	counts := map[string]int{}
	values := map[string]any{}
	for _, src := range sources {
		idx, ok := src.byTime[ts]
		if !ok {
			continue
		}
		v := listValueAtGo(src.hourly["weather_code"], idx)
		k := jsonutil.TextValue(v)
		if k != "" {
			counts[k]++
			values[k] = v
		}
	}
	return bestModalGo(counts, values)
}

func robustAverageGo(vals []float64) any {
	cleaned := []float64{}
	for _, v := range vals {
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			cleaned = append(cleaned, v)
		}
	}
	if len(cleaned) == 0 {
		return nil
	}
	slices.Sort(cleaned)
	if len(cleaned) >= 5 {
		cleaned = cleaned[1 : len(cleaned)-1]
	}
	sum := 0.0
	for _, v := range cleaned {
		sum += v
	}
	return math.Round((sum/float64(len(cleaned)))*10) / 10
}

func indexOfAnyStringGo(xs []any, want string) int {
	for i, v := range xs {
		if jsonutil.TextValue(v) == want {
			return i
		}
	}
	return -1
}

func listFloatAtGo(v any, idx int) (float64, bool) {
	xs := jsonutil.List(v)
	if idx < 0 || idx >= len(xs) {
		return 0, false
	}
	return toFloatGo(xs[idx])
}

func listValueAtGo(v any, idx int) any {
	xs := jsonutil.List(v)
	if idx < 0 || idx >= len(xs) {
		return nil
	}
	return xs[idx]
}

func firstDailyValueByDateGo(sources []any, date, field string) any {
	for _, src := range sources {
		d := anyMap(anyMap(src)["daily"])
		if idx := indexOfAnyStringGo(jsonutil.List(d["time"]), date); idx >= 0 {
			if v := listValueAtGo(d[field], idx); v != nil {
				return v
			}
		}
	}
	return nil
}

func modalWeatherCodeByDateGo(sources []any, date string) any {
	counts := map[string]int{}
	values := map[string]any{}
	for _, src := range sources {
		d := anyMap(anyMap(src)["daily"])
		if idx := indexOfAnyStringGo(jsonutil.List(d["time"]), date); idx >= 0 {
			v := listValueAtGo(d["weather_code"], idx)
			k := jsonutil.TextValue(v)
			if k != "" {
				counts[k]++
				values[k] = v
			}
		}
	}
	return bestModalGo(counts, values)
}

func modalWeatherCodeGo(sources []any, _ int, scope string) any {
	counts := map[string]int{}
	values := map[string]any{}
	for _, src := range sources {
		cur := anyMap(anyMap(src)["current"])
		v := cur["weather_code"]
		k := jsonutil.TextValue(v)
		if k != "" {
			counts[k]++
			values[k] = v
		}
	}
	return bestModalGo(counts, values)
}

func bestModalGo(counts map[string]int, values map[string]any) any {
	best := ""
	bestN := -1
	keys := slices.Sorted(maps.Keys(counts))
	for _, k := range keys {
		if counts[k] > bestN {
			bestN = counts[k]
			best = k
		}
	}
	if best == "" {
		return nil
	}
	return values[best]
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
