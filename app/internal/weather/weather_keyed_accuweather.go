package weather

import (
	"context"
	"fmt"
	"net/url"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func fetchAccuWeatherGo(ctx context.Context, cfg Config) (map[string]any, error) {
	key := weatherProviderKeyGo("accuweather", cfg)
	locationKey, cached := weatherLocationCacheRead(cfg, "accuweather")
	if !cached {
		locationURL := "https://dataservice.accuweather.com/locations/v1/cities/geoposition/search?" + weatherURLValues(map[string]string{"apikey": key, "q": fmt.Sprintf("%s,%s", trimFloat(cfg.Lat), trimFloat(cfg.Lon))})
		location, err := fetchJSONGo(ctx, locationURL)
		if err != nil {
			return nil, err
		}
		locationKey = jsonutil.StringValue(location["Key"])
		if locationKey == "" {
			return nil, fmt.Errorf("AccuWeather location lookup failed")
		}
		weatherLocationCacheWrite(cfg, "accuweather", locationKey)
	}
	metric := "false"
	if cfg.TempUnit == "celsius" {
		metric = "true"
	}
	currentURL := fmt.Sprintf("https://dataservice.accuweather.com/currentconditions/v1/%s?%s", url.PathEscape(locationKey), weatherURLValues(map[string]string{"apikey": key, "details": "true"}))
	dailyURL := fmt.Sprintf("https://dataservice.accuweather.com/forecasts/v1/daily/5day/%s?%s", url.PathEscape(locationKey), weatherURLValues(map[string]string{"apikey": key, "details": "true", "metric": metric}))
	results := weatherParallelCalls(
		func() (any, error) { return fetchJSONAnyGo(ctx, currentURL) },
		func() (any, error) { return fetchJSONGo(ctx, dailyURL) },
	)
	current := map[string]any{}
	if currentList := jsonutil.List(results[0].Value); len(currentList) > 0 {
		c := anyMap(currentList[0])
		unitKey := "Imperial"
		windUnit := "mph"
		if cfg.TempUnit == "celsius" {
			unitKey = "Metric"
		}
		if cfg.WindUnit == "ms" || cfg.WindUnit == "kmh" {
			windUnit = "kmh"
		}
		temp := anyMap(c["Temperature"])
		real := anyMap(c["RealFeelTemperature"])
		wind := anyMap(c["Wind"])
		windProviderUnit := "Imperial"
		if windUnit == "kmh" {
			windProviderUnit = "Metric"
		}
		current = map[string]any{"temperature_2m": anyMap(temp[unitKey])["Value"], "apparent_temperature": anyMap(real[unitKey])["Value"], "weather_code": textCodeGo(c["WeatherText"]), "wind_speed_10m": toWindGo(anyMap(anyMap(wind["Speed"])[windProviderUnit])["Value"], windUnit, cfg.WindUnit), "relative_humidity_2m": c["RelativeHumidity"]}
	}
	d := emptyDailyGo()
	if daily, ok := results[1].Value.(map[string]any); ok {
		for _, raw := range jsonutil.List(daily["DailyForecasts"]) {
			x := anyMap(raw)
			temp := anyMap(x["Temperature"])
			real := anyMap(x["RealFeelTemperature"])
			day := anyMap(x["Day"])
			d["time"] = append(d["time"], firstN(fmt.Sprint(x["Date"]), 10))
			d["weather_code"] = append(d["weather_code"], textCodeGo(day["IconPhrase"]))
			d["temperature_2m_max"] = append(d["temperature_2m_max"], anyMap(temp["Maximum"])["Value"])
			d["temperature_2m_min"] = append(d["temperature_2m_min"], anyMap(temp["Minimum"])["Value"])
			d["apparent_temperature_max"] = append(d["apparent_temperature_max"], anyMap(real["Maximum"])["Value"])
			d["precipitation_sum"] = append(d["precipitation_sum"], nil)
			d["precipitation_probability_max"] = append(d["precipitation_probability_max"], day["PrecipitationProbability"])
			dailyWindUnit := "mph"
			if metric == "true" {
				dailyWindUnit = "kmh"
			}
			d["wind_speed_10m_max"] = append(d["wind_speed_10m_max"], toWindGo(anyMap(anyMap(day["Wind"])["Speed"])["Value"], dailyWindUnit, cfg.WindUnit))
			d["uv_index_max"] = append(d["uv_index_max"], nil)
			d["sunrise"] = append(d["sunrise"], nil)
			d["sunset"] = append(d["sunset"], nil)
		}
	}
	return weatherPartialSourceGo("accuweather", current, d, nil, results[0].Err, results[1].Err)
}
