package weather

import "encoding/json"

// Provider caches use canonical Celsius, metres/second, millimetres, and
// provider-local date/time values. Display conversion happens only on a copy
// returned to the browser, so changing units never causes another provider
// request and remains usable from the last-good provider cache while offline.
func weatherCanonicalFetchConfigGo(cfg Config) Config {
	out := cfg
	out.TempUnit = "celsius"
	out.WindUnit = "ms"
	return out
}

func weatherDisplaySourceGo(source map[string]any, display Config) map[string]any {
	if source == nil {
		return nil
	}
	body, _ := json.Marshal(source)
	out := map[string]any{}
	_ = json.Unmarshal(body, &out)
	convertTemp := func(value any) any { return toTempGo(value, "c", display.TempUnit) }
	convertWind := func(value any) any { return toWindGo(value, "ms", display.WindUnit) }
	current := anyMap(out["current"])
	for _, key := range []string{"temperature_2m", "apparent_temperature"} {
		if _, ok := current[key]; ok {
			current[key] = convertTemp(current[key])
		}
	}
	if _, ok := current["wind_speed_10m"]; ok {
		current["wind_speed_10m"] = convertWind(current["wind_speed_10m"])
	}
	out["current"] = current
	for _, sectionName := range []string{"daily", "hourly"} {
		section := anyMap(out[sectionName])
		for _, key := range []string{"temperature_2m", "temperature_2m_max", "temperature_2m_min", "apparent_temperature_max"} {
			if values, ok := section[key].([]any); ok {
				for index := range values {
					values[index] = convertTemp(values[index])
				}
				section[key] = values
			}
		}
		for _, key := range []string{"wind_speed_10m", "wind_speed_10m_max"} {
			if values, ok := section[key].([]any); ok {
				for index := range values {
					values[index] = convertWind(values[index])
				}
				section[key] = values
			}
		}
		out[sectionName] = section
	}
	out["_canonicalUnits"] = map[string]any{"temperature": "celsius", "wind": "m/s", "precipitation": "mm"}
	return out
}
