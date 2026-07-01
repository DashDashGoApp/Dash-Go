package weather

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func fetchWeatherAPIGo(ctx context.Context, cfg Config) (map[string]any, error) {
	k := weatherProviderKeyGo("weatherapi", cfg)
	days := clamp(cfg.Days, 1, 14)
	q := fmt.Sprintf("%s,%s", trimFloat(cfg.Lat), trimFloat(cfg.Lon))
	u := "https://api.weatherapi.com/v1/forecast.json?" + weatherURLValues(map[string]string{"key": k, "days": strconv.Itoa(days), "aqi": "no", "alerts": "no"}) + "&q=" + url.QueryEscape(q)
	j, err := fetchJSONGo(ctx, u)
	if err != nil {
		return nil, err
	}
	d := emptyDailyGo()
	forecast := anyMap(j["forecast"])
	for _, raw := range jsonutil.List(forecast["forecastday"]) {
		x := anyMap(raw)
		day := anyMap(x["day"])
		cond := anyMap(day["condition"])
		d["time"] = append(d["time"], x["date"])
		d["weather_code"] = append(d["weather_code"], textCodeGo(cond["text"]))
		unit := "f"
		if cfg.TempUnit == "celsius" {
			unit = "c"
		}
		d["temperature_2m_max"] = append(d["temperature_2m_max"], toTempGo(choice(unit == "c", day["maxtemp_c"], day["maxtemp_f"]), unit, cfg.TempUnit))
		d["temperature_2m_min"] = append(d["temperature_2m_min"], toTempGo(choice(unit == "c", day["mintemp_c"], day["mintemp_f"]), unit, cfg.TempUnit))
		d["apparent_temperature_max"] = append(d["apparent_temperature_max"], nil)
		d["precipitation_sum"] = append(d["precipitation_sum"], precipitationMMGo(day["totalprecip_mm"], "mm"))
		d["precipitation_probability_max"] = append(d["precipitation_probability_max"], day["daily_chance_of_rain"])
		d["wind_speed_10m_max"] = append(d["wind_speed_10m_max"], toWindGo(day["maxwind_mph"], "mph", cfg.WindUnit))
		d["uv_index_max"] = append(d["uv_index_max"], day["uv"])
		d["sunrise"] = append(d["sunrise"], nil)
		d["sunset"] = append(d["sunset"], nil)
	}
	c := anyMap(j["current"])
	cond := anyMap(c["condition"])
	unit := "f"
	if cfg.TempUnit == "celsius" {
		unit = "c"
	}
	return weatherOKGo("weatherapi", map[string]any{"current": map[string]any{"temperature_2m": toTempGo(choice(unit == "c", c["temp_c"], c["temp_f"]), unit, cfg.TempUnit), "apparent_temperature": toTempGo(choice(unit == "c", c["feelslike_c"], c["feelslike_f"]), unit, cfg.TempUnit), "weather_code": textCodeGo(cond["text"]), "wind_speed_10m": toWindGo(c["wind_mph"], "mph", cfg.WindUnit), "relative_humidity_2m": c["humidity"]}, "daily": d, "hourly": nil}), nil
}

func fetchOpenWeatherGo(ctx context.Context, cfg Config) (map[string]any, error) {
	k := weatherProviderKeyGo("openweather", cfg)
	units := "imperial"
	if cfg.TempUnit == "celsius" {
		units = "metric"
	}
	u := "https://api.openweathermap.org/data/3.0/onecall?" + weatherURLValues(map[string]string{"lat": trimFloat(cfg.Lat), "lon": trimFloat(cfg.Lon), "appid": k, "units": units, "exclude": "minutely,alerts"})
	j, err := fetchJSONGo(ctx, u)
	if err != nil {
		return nil, err
	}
	d := emptyDailyGo()
	for _, raw := range jsonutil.List(j["daily"])[:min(len(jsonutil.List(j["daily"])), cfg.Days)] {
		x := anyMap(raw)
		temp := anyMap(x["temp"])
		feels := anyMap(x["feels_like"])
		w := firstMap(jsonutil.List(x["weather"]))
		d["time"] = append(d["time"], tsDateGo(x["dt"]))
		d["weather_code"] = append(d["weather_code"], owCodeGo(xOr(w["id"], 0)))
		d["temperature_2m_max"] = append(d["temperature_2m_max"], temp["max"])
		d["temperature_2m_min"] = append(d["temperature_2m_min"], temp["min"])
		d["apparent_temperature_max"] = append(d["apparent_temperature_max"], xOr(feels["day"], feels["max"]))
		d["precipitation_sum"] = append(d["precipitation_sum"], precipitationSumMMGo(x["rain"], x["snow"]))
		d["precipitation_probability_max"] = append(d["precipitation_probability_max"], mult100(x["pop"]))
		d["wind_speed_10m_max"] = append(d["wind_speed_10m_max"], toWindGo(x["wind_speed"], choice(units == "metric", "ms", "mph").(string), cfg.WindUnit))
		d["uv_index_max"] = append(d["uv_index_max"], x["uvi"])
		d["sunrise"] = append(d["sunrise"], tsISOGo(x["sunrise"]))
		d["sunset"] = append(d["sunset"], tsISOGo(x["sunset"]))
	}
	c := anyMap(j["current"])
	w := firstMap(jsonutil.List(c["weather"]))
	return weatherOKGo("openweather", map[string]any{"current": map[string]any{"temperature_2m": c["temp"], "apparent_temperature": c["feels_like"], "weather_code": owCodeGo(w["id"]), "wind_speed_10m": toWindGo(c["wind_speed"], choice(units == "metric", "ms", "mph").(string), cfg.WindUnit), "relative_humidity_2m": c["humidity"]}, "daily": d, "hourly": nil}), nil
}

func fetchGoogleWeatherGo(ctx context.Context, cfg Config) (map[string]any, error) {
	k := weatherProviderKeyGo("googleweather", cfg)
	days := clamp(cfg.Days, 1, 10)
	units := "IMPERIAL"
	if cfg.TempUnit == "celsius" {
		units = "METRIC"
	}
	baseVals := map[string]string{"key": k, "location.latitude": trimFloat(cfg.Lat), "location.longitude": trimFloat(cfg.Lon), "unitsSystem": units}
	cur, err := fetchJSONGo(ctx, "https://weather.googleapis.com/v1/currentConditions:lookup?"+weatherURLValues(baseVals))
	if err != nil {
		return nil, err
	}
	vals := mapCopy(baseVals)
	vals["days"] = strconv.Itoa(days)
	vals["pageSize"] = strconv.Itoa(days)
	daily, err := fetchJSONGo(ctx, "https://weather.googleapis.com/v1/forecast/days:lookup?"+weatherURLValues(vals))
	if err != nil {
		return nil, err
	}
	d := emptyDailyGo()
	for _, raw := range jsonutil.List(daily["forecastDays"]) {
		x := anyMap(raw)
		day := anyMap(x["daytimeForecast"])
		sun := anyMap(x["sunEvents"])
		d["time"] = append(d["time"], googleDateGo(x))
		d["weather_code"] = append(d["weather_code"], textCodeGo(conditionTextGo(day)))
		d["temperature_2m_max"] = append(d["temperature_2m_max"], degreesGo(x["maxTemperature"]))
		d["temperature_2m_min"] = append(d["temperature_2m_min"], degreesGo(x["minTemperature"]))
		d["apparent_temperature_max"] = append(d["apparent_temperature_max"], degreesGo(x["feelsLikeMaxTemperature"]))
		d["precipitation_sum"] = append(d["precipitation_sum"], googlePrecipitationMMGo(anyMap(day["precipitation"])["qpf"], choice(units == "METRIC", "mm", "in").(string)))
		d["precipitation_probability_max"] = append(d["precipitation_probability_max"], anyMap(anyMap(day["precipitation"])["probability"])["percent"])
		d["wind_speed_10m_max"] = append(d["wind_speed_10m_max"], googleWindGo(day["wind"], cfg))
		d["uv_index_max"] = append(d["uv_index_max"], day["uvIndex"])
		d["sunrise"] = append(d["sunrise"], sun["sunriseTime"])
		d["sunset"] = append(d["sunset"], sun["sunsetTime"])
	}
	return weatherOKGo("googleweather", map[string]any{"current": map[string]any{"temperature_2m": degreesGo(cur["temperature"]), "apparent_temperature": degreesGo(cur["feelsLikeTemperature"]), "weather_code": textCodeGo(conditionTextGo(cur)), "wind_speed_10m": googleWindGo(cur["wind"], cfg), "relative_humidity_2m": cur["relativeHumidity"]}, "daily": d, "hourly": nil}), nil
}

func fetchTomorrowGo(ctx context.Context, cfg Config) (map[string]any, error) {
	k := weatherProviderKeyGo("tomorrow", cfg)
	units := "imperial"
	if cfg.TempUnit == "celsius" {
		units = "metric"
	}
	u := "https://api.tomorrow.io/v4/weather/forecast?" + weatherURLValues(map[string]string{"location": fmt.Sprintf("%s,%s", trimFloat(cfg.Lat), trimFloat(cfg.Lon)), "apikey": k, "units": units, "timesteps": "1d,1h"})
	j, err := fetchJSONGo(ctx, u)
	if err != nil {
		return nil, err
	}
	d := emptyDailyGo()
	daily := jsonutil.List(anyMap(j["timelines"])["daily"])
	for _, raw := range daily[:min(len(daily), cfg.Days)] {
		x := anyMap(raw)
		v := anyMap(x["values"])
		d["time"] = append(d["time"], firstN(fmt.Sprint(x["time"]), 10))
		d["weather_code"] = append(d["weather_code"], textCodeGo(xOr(v["weatherCodeFullDay"], v["weatherCodeMax"])))
		d["temperature_2m_max"] = append(d["temperature_2m_max"], v["temperatureMax"])
		d["temperature_2m_min"] = append(d["temperature_2m_min"], v["temperatureMin"])
		d["apparent_temperature_max"] = append(d["apparent_temperature_max"], v["temperatureApparentMax"])
		d["precipitation_sum"] = append(d["precipitation_sum"], precipitationMMGo(v["precipitationAccumulationSum"], choice(units == "metric", "mm", "in").(string)))
		d["precipitation_probability_max"] = append(d["precipitation_probability_max"], v["precipitationProbabilityMax"])
		d["wind_speed_10m_max"] = append(d["wind_speed_10m_max"], toWindGo(v["windSpeedMax"], choice(units == "metric", "ms", "mph").(string), cfg.WindUnit))
		d["uv_index_max"] = append(d["uv_index_max"], v["uvIndexMax"])
		d["sunrise"] = append(d["sunrise"], v["sunriseTime"])
		d["sunset"] = append(d["sunset"], v["sunsetTime"])
	}
	current := map[string]any{}
	hourly := jsonutil.List(anyMap(j["timelines"])["hourly"])
	if len(hourly) > 0 {
		v := anyMap(anyMap(hourly[0])["values"])
		current = map[string]any{"temperature_2m": v["temperature"], "apparent_temperature": v["temperatureApparent"], "weather_code": textCodeGo(xOr(v["weatherCode"], v["weatherCodeFull"])), "wind_speed_10m": toWindGo(v["windSpeed"], choice(units == "metric", "ms", "mph").(string), cfg.WindUnit), "relative_humidity_2m": v["humidity"]}
	}
	return weatherOKGo("tomorrow", map[string]any{"current": current, "daily": d, "hourly": nil}), nil
}

func fetchVisualCrossingGo(ctx context.Context, cfg Config) (map[string]any, error) {
	k := weatherProviderKeyGo("visualcrossing", cfg)
	unit := "us"
	if cfg.TempUnit == "celsius" {
		unit = "metric"
	}
	u := fmt.Sprintf("https://weather.visualcrossing.com/VisualCrossingWebServices/rest/services/timeline/%s,%s?%s", trimFloat(cfg.Lat), trimFloat(cfg.Lon), weatherURLValues(map[string]string{"unitGroup": unit, "key": k, "include": "current,days"}))
	j, err := fetchJSONGo(ctx, u)
	if err != nil {
		return nil, err
	}
	d := emptyDailyGo()
	for _, raw := range jsonutil.List(j["days"])[:min(len(jsonutil.List(j["days"])), cfg.Days)] {
		x := anyMap(raw)
		d["time"] = append(d["time"], x["datetime"])
		d["weather_code"] = append(d["weather_code"], textCodeGo(xOr(x["conditions"], x["icon"])))
		d["temperature_2m_max"] = append(d["temperature_2m_max"], x["tempmax"])
		d["temperature_2m_min"] = append(d["temperature_2m_min"], x["tempmin"])
		d["apparent_temperature_max"] = append(d["apparent_temperature_max"], x["feelslikemax"])
		d["precipitation_sum"] = append(d["precipitation_sum"], precipitationMMGo(x["precip"], choice(unit == "metric", "mm", "in").(string)))
		d["precipitation_probability_max"] = append(d["precipitation_probability_max"], x["precipprob"])
		d["wind_speed_10m_max"] = append(d["wind_speed_10m_max"], toWindGo(x["windspeed"], "mph", cfg.WindUnit))
		d["uv_index_max"] = append(d["uv_index_max"], x["uvindex"])
		d["sunrise"] = append(d["sunrise"], x["sunrise"])
		d["sunset"] = append(d["sunset"], x["sunset"])
	}
	c := anyMap(j["currentConditions"])
	return weatherOKGo("visualcrossing", map[string]any{"current": map[string]any{"temperature_2m": c["temp"], "apparent_temperature": c["feelslike"], "weather_code": textCodeGo(xOr(c["conditions"], c["icon"])), "wind_speed_10m": toWindGo(c["windspeed"], "mph", cfg.WindUnit), "relative_humidity_2m": c["humidity"]}, "daily": d, "hourly": nil}), nil
}

func fetchWeatherbitGo(ctx context.Context, cfg Config) (map[string]any, error) {
	k := weatherProviderKeyGo("weatherbit", cfg)
	// Request Weatherbit in metric units regardless of display preference so
	// daily precipitation has one unambiguous adapter boundary: millimetres.
	units := "M"
	base := "https://api.weatherbit.io/v2.0"
	cur, err := fetchJSONGo(ctx, base+"/current?"+weatherURLValues(map[string]string{"lat": trimFloat(cfg.Lat), "lon": trimFloat(cfg.Lon), "key": k, "units": units}))
	if err != nil {
		return nil, err
	}
	daily, err := fetchJSONGo(ctx, base+"/forecast/daily?"+weatherURLValues(map[string]string{"lat": trimFloat(cfg.Lat), "lon": trimFloat(cfg.Lon), "key": k, "units": units, "days": strconv.Itoa(clamp(cfg.Days, 1, 7))}))
	if err != nil {
		return nil, err
	}
	d := emptyDailyGo()
	for _, raw := range jsonutil.List(daily["data"]) {
		x := anyMap(raw)
		w := anyMap(x["weather"])
		d["time"] = append(d["time"], x["valid_date"])
		d["weather_code"] = append(d["weather_code"], textCodeGo(xOr(w["description"], w["code"])))
		d["temperature_2m_max"] = append(d["temperature_2m_max"], toTempGo(x["max_temp"], "c", cfg.TempUnit))
		d["temperature_2m_min"] = append(d["temperature_2m_min"], toTempGo(x["min_temp"], "c", cfg.TempUnit))
		d["apparent_temperature_max"] = append(d["apparent_temperature_max"], toTempGo(x["app_max_temp"], "c", cfg.TempUnit))
		d["precipitation_sum"] = append(d["precipitation_sum"], precipitationMMGo(x["precip"], "mm"))
		d["precipitation_probability_max"] = append(d["precipitation_probability_max"], x["pop"])
		d["wind_speed_10m_max"] = append(d["wind_speed_10m_max"], toWindGo(x["wind_spd"], "ms", cfg.WindUnit))
		d["uv_index_max"] = append(d["uv_index_max"], x["uv"])
		d["sunrise"] = append(d["sunrise"], nil)
		d["sunset"] = append(d["sunset"], nil)
	}
	c := firstMap(jsonutil.List(cur["data"]))
	cw := anyMap(c["weather"])
	return weatherOKGo("weatherbit", map[string]any{"current": map[string]any{"temperature_2m": toTempGo(c["temp"], "c", cfg.TempUnit), "apparent_temperature": toTempGo(c["app_temp"], "c", cfg.TempUnit), "weather_code": textCodeGo(xOr(cw["description"], cw["code"])), "wind_speed_10m": toWindGo(c["wind_spd"], "ms", cfg.WindUnit), "relative_humidity_2m": c["rh"]}, "daily": d, "hourly": nil}), nil
}

func fetchPirateWeatherGo(ctx context.Context, cfg Config) (map[string]any, error) {
	k := weatherProviderKeyGo("pirateweather", cfg)
	units := "us"
	if cfg.TempUnit == "celsius" {
		units = "si"
	}
	u := fmt.Sprintf("https://api.pirateweather.net/forecast/%s/%s,%s?%s", k, trimFloat(cfg.Lat), trimFloat(cfg.Lon), weatherURLValues(map[string]string{"units": units, "exclude": "minutely,alerts"}))
	j, err := fetchJSONGo(ctx, u)
	if err != nil {
		return nil, err
	}
	d := emptyDailyGo()
	for _, raw := range jsonutil.List(anyMap(j["daily"])["data"])[:min(len(jsonutil.List(anyMap(j["daily"])["data"])), cfg.Days)] {
		x := anyMap(raw)
		d["time"] = append(d["time"], tsDateGo(x["time"]))
		d["weather_code"] = append(d["weather_code"], textCodeGo(xOr(x["summary"], x["icon"])))
		d["temperature_2m_max"] = append(d["temperature_2m_max"], xOr(x["temperatureHigh"], x["temperatureMax"]))
		d["temperature_2m_min"] = append(d["temperature_2m_min"], xOr(x["temperatureLow"], x["temperatureMin"]))
		d["apparent_temperature_max"] = append(d["apparent_temperature_max"], xOr(x["apparentTemperatureHigh"], x["apparentTemperatureMax"]))
		d["precipitation_sum"] = append(d["precipitation_sum"], precipitationMMGo(x["precipAccumulation"], choice(units == "si", "cm", "in").(string)))
		d["precipitation_probability_max"] = append(d["precipitation_probability_max"], mult100(x["precipProbability"]))
		d["wind_speed_10m_max"] = append(d["wind_speed_10m_max"], toWindGo(x["windSpeed"], choice(units == "si", "ms", "mph").(string), cfg.WindUnit))
		d["uv_index_max"] = append(d["uv_index_max"], x["uvIndex"])
		d["sunrise"] = append(d["sunrise"], tsISOGo(x["sunriseTime"]))
		d["sunset"] = append(d["sunset"], tsISOGo(x["sunsetTime"]))
	}
	c := anyMap(j["currently"])
	return weatherOKGo("pirateweather", map[string]any{"current": map[string]any{"temperature_2m": c["temperature"], "apparent_temperature": c["apparentTemperature"], "weather_code": textCodeGo(xOr(c["summary"], c["icon"])), "wind_speed_10m": toWindGo(c["windSpeed"], choice(units == "si", "ms", "mph").(string), cfg.WindUnit), "relative_humidity_2m": mult100(c["humidity"])}, "daily": d, "hourly": nil}), nil
}

func fetchAccuWeatherGo(ctx context.Context, cfg Config) (map[string]any, error) {
	k := weatherProviderKeyGo("accuweather", cfg)
	locURL := "https://dataservice.accuweather.com/locations/v1/cities/geoposition/search?" + weatherURLValues(map[string]string{"apikey": k, "q": fmt.Sprintf("%s,%s", trimFloat(cfg.Lat), trimFloat(cfg.Lon))})
	loc, err := fetchJSONGo(ctx, locURL)
	if err != nil {
		return nil, err
	}
	locKey := jsonutil.StringValue(loc["Key"])
	if locKey == "" {
		return nil, fmt.Errorf("AccuWeather location lookup failed")
	}
	metric := "false"
	if cfg.TempUnit == "celsius" {
		metric = "true"
	}
	curList, err := fetchJSONAnyGo(ctx, fmt.Sprintf("https://dataservice.accuweather.com/currentconditions/v1/%s?%s", url.PathEscape(locKey), weatherURLValues(map[string]string{"apikey": k, "details": "true"})))
	if err != nil {
		return nil, err
	}
	daily, err := fetchJSONGo(ctx, fmt.Sprintf("https://dataservice.accuweather.com/forecasts/v1/daily/5day/%s?%s", url.PathEscape(locKey), weatherURLValues(map[string]string{"apikey": k, "details": "true", "metric": metric})))
	if err != nil {
		return nil, err
	}
	d := emptyDailyGo()
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
		d["wind_speed_10m_max"] = append(d["wind_speed_10m_max"], toWindGo(anyMap(anyMap(day["Wind"])["Speed"])["Value"], choice(metric == "true", "kmh", "mph").(string), cfg.WindUnit))
		d["uv_index_max"] = append(d["uv_index_max"], nil)
		d["sunrise"] = append(d["sunrise"], nil)
		d["sunset"] = append(d["sunset"], nil)
	}
	c := map[string]any{}
	if arr := jsonutil.List(curList); len(arr) > 0 {
		c = anyMap(arr[0])
	}
	unitKey := "Imperial"
	if cfg.TempUnit == "celsius" {
		unitKey = "Metric"
	}
	temp := anyMap(c["Temperature"])
	real := anyMap(c["RealFeelTemperature"])
	wind := anyMap(c["Wind"])
	return weatherOKGo("accuweather", map[string]any{"current": map[string]any{"temperature_2m": anyMap(temp[unitKey])["Value"], "apparent_temperature": anyMap(real[unitKey])["Value"], "weather_code": textCodeGo(c["WeatherText"]), "wind_speed_10m": toWindGo(anyMap(anyMap(wind["Speed"])[choice(cfg.WindUnit == "kmh", "Metric", "Imperial").(string)])["Value"], cfg.WindUnit, cfg.WindUnit), "relative_humidity_2m": c["RelativeHumidity"]}, "daily": d, "hourly": nil}), nil
}
