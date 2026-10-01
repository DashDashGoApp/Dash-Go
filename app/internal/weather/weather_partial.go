package weather

import (
	"errors"
	"sync"
)

type weatherCallResult struct {
	Value any
	Err   error
}

func weatherParallelCalls(calls ...func() (any, error)) []weatherCallResult {
	results := make([]weatherCallResult, len(calls))
	var wg sync.WaitGroup
	for index, call := range calls {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[index].Value, results[index].Err = call()
		}()
	}
	wg.Wait()
	return results
}

func weatherPartialSourceGo(id string, current map[string]any, daily map[string][]any, hourly any, currentErr, dailyErr, hourlyErr error) (map[string]any, error) {
	if current == nil {
		current = map[string]any{}
	}
	if daily == nil {
		daily = emptyDailyGo()
	}
	available := []string{}
	if len(current) > 0 {
		available = append(available, "current")
	}
	if len(daily["time"]) > 0 {
		available = append(available, "daily")
	}
	if hourly != nil {
		available = append(available, "hourly")
	}
	if len(available) == 0 {
		return nil, errors.Join(currentErr, dailyErr, hourlyErr)
	}
	if len(current) == 0 && len(daily["time"]) > 0 {
		first := func(key string) any {
			values := daily[key]
			if len(values) == 0 {
				return nil
			}
			return values[0]
		}
		current = map[string]any{
			"temperature_2m":       first("temperature_2m_max"),
			"apparent_temperature": first("apparent_temperature_max"),
			"weather_code":         first("weather_code"),
			"wind_speed_10m":       first("wind_speed_10m_max"),
			"relative_humidity_2m": nil,
		}
		available = append(available, "current-derived")
	}
	out := weatherOKGo(id, map[string]any{"current": current, "daily": daily, "hourly": hourly})
	out["_available"] = available
	if currentErr != nil || dailyErr != nil || hourlyErr != nil {
		out["_partial"] = true
		errorsByPart := map[string]any{}
		if currentErr != nil {
			errorsByPart["current"] = currentErr.Error()
		}
		if dailyErr != nil {
			errorsByPart["daily"] = dailyErr.Error()
		}
		if hourlyErr != nil {
			errorsByPart["hourly"] = hourlyErr.Error()
		}
		out["_partialErrors"] = errorsByPart
	}
	return out, nil
}
