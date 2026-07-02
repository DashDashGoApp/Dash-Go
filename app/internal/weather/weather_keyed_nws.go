package weather

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func fetchXWeatherGo(ctx context.Context, cfg Config) (map[string]any, error) {
	cid, secret := xweatherCredsGo(weatherProviderKeyGo("xweather", cfg))
	if cid == "" || secret == "" {
		return nil, fmt.Errorf("missing Xweather key; enter client_id:client_secret in installer option 6")
	}
	place := trimFloat(cfg.Lat) + "," + trimFloat(cfg.Lon)
	auth := map[string]string{"client_id": cid, "client_secret": secret, "format": "json"}
	vals := mapCopy(auth)
	vals["limit"] = strconv.Itoa(clamp(cfg.Days, 1, 15))
	vals["filter"] = "day"
	daily, err := fetchJSONGo(ctx, "https://data.api.xweather.com/forecasts/"+url.PathEscape(place)+"?"+weatherURLValues(vals))
	if err != nil {
		return nil, err
	}
	vals = mapCopy(auth)
	vals["limit"] = "1"
	obs, err := fetchJSONGo(ctx, "https://data.api.xweather.com/observations/"+url.PathEscape(place)+"?"+weatherURLValues(vals))
	if err != nil {
		return nil, err
	}
	d := emptyDailyGo()
	periods := []any{}
	for _, resp := range jsonutil.List(daily["response"]) {
		periods = append(periods, jsonutil.List(anyMap(resp)["periods"])...)
	}
	for _, raw := range periods[:min(len(periods), cfg.Days)] {
		x := anyMap(raw)
		d["time"] = append(d["time"], firstN(fmt.Sprint(xOr(x["dateTimeISO"], x["timestamp"])), 10))
		d["weather_code"] = append(d["weather_code"], textCodeGo(xOr(x["weather"], xOr(x["weatherPrimary"], x["icon"]))))
		d["temperature_2m_max"] = append(d["temperature_2m_max"], toTempGo(x["maxTempF"], "f", cfg.TempUnit))
		d["temperature_2m_min"] = append(d["temperature_2m_min"], toTempGo(x["minTempF"], "f", cfg.TempUnit))
		d["apparent_temperature_max"] = append(d["apparent_temperature_max"], toTempGo(xOr(x["feelslikeF"], x["maxFeelslikeF"]), "f", cfg.TempUnit))
		d["precipitation_sum"] = append(d["precipitation_sum"], precipitationMMGo(x["precipIN"], "in"))
		d["precipitation_probability_max"] = append(d["precipitation_probability_max"], x["pop"])
		d["wind_speed_10m_max"] = append(d["wind_speed_10m_max"], toWindGo(x["windSpeedMPH"], "mph", cfg.WindUnit))
		d["uv_index_max"] = append(d["uv_index_max"], x["uvi"])
		d["sunrise"] = append(d["sunrise"], x["sunriseISO"])
		d["sunset"] = append(d["sunset"], x["sunsetISO"])
	}
	resp := obs["response"]
	var ob map[string]any
	if arr := jsonutil.List(resp); len(arr) > 0 {
		ob = anyMap(anyMap(arr[0])["ob"])
	} else {
		ob = anyMap(anyMap(resp)["ob"])
	}
	return weatherOKGo("xweather", map[string]any{"current": map[string]any{"temperature_2m": toTempGo(ob["tempF"], "f", cfg.TempUnit), "apparent_temperature": toTempGo(ob["feelslikeF"], "f", cfg.TempUnit), "weather_code": textCodeGo(xOr(ob["weather"], ob["weatherShort"])), "wind_speed_10m": toWindGo(ob["windSpeedMPH"], "mph", cfg.WindUnit), "relative_humidity_2m": ob["humidity"]}, "daily": d, "hourly": nil}), nil
}

func xweatherCredsGo(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	for _, sep := range []string{":", ",", "|"} {
		if strings.Contains(raw, sep) {
			p := strings.SplitN(raw, sep, 2)
			return strings.TrimSpace(p[0]), strings.TrimSpace(p[1])
		}
	}
	return raw, ""
}

func fetchNWSGo(ctx context.Context, cfg Config) (map[string]any, error) {
	if !insideNWSCoverageGo(cfg.Lat, cfg.Lon) {
		return nil, fmt.Errorf("NWS / NOAA is US-only and is unavailable for this location; source will be skipped")
	}
	pointsURL := fmt.Sprintf("https://api.weather.gov/points/%s,%s", trimFloat(cfg.Lat), trimFloat(cfg.Lon))
	points, err := fetchJSONGo(ctx, pointsURL)
	if err != nil {
		return nil, err
	}
	forecastURL := jsonutil.StringValue(anyMap(points["properties"])["forecast"])
	if forecastURL == "" {
		return nil, fmt.Errorf("NWS did not return a forecast grid for this location")
	}
	fc, err := fetchJSONGo(ctx, forecastURL)
	if err != nil {
		return nil, err
	}
	periods := jsonutil.List(anyMap(fc["properties"])["periods"])
	if len(periods) == 0 {
		return nil, fmt.Errorf("NWS forecast returned no periods for this location")
	}
	d := emptyDailyGo()
	buckets := map[string][]map[string]any{}
	order := []string{}
	for _, raw := range periods {
		p := anyMap(raw)
		date := firstN(fmt.Sprint(p["startTime"]), 10)
		if date == "" {
			continue
		}
		if _, ok := buckets[date]; !ok {
			order = append(order, date)
		}
		buckets[date] = append(buckets[date], p)
	}
	for _, date := range order[:min(len(order), cfg.Days)] {
		arr := buckets[date]
		nums := []float64{}
		pops := []float64{}
		winds := []float64{}
		primary := map[string]any{}
		for i, p := range arr {
			if i == 0 || p["isDaytime"] == true {
				primary = p
			}
			if v, ok := toFloatGo(toTempGo(p["temperature"], strings.ToLower(firstN(fmt.Sprint(p["temperatureUnit"]), 1)), cfg.TempUnit)); ok {
				nums = append(nums, v)
			}
			if v, ok := toFloatGo(popGo(p)); ok {
				pops = append(pops, v)
			}
			if v, ok := toFloatGo(nwsWindGo(p, cfg)); ok {
				winds = append(winds, v)
			}
		}
		d["time"] = append(d["time"], date)
		d["weather_code"] = append(d["weather_code"], textCodeGo(xOr(primary["shortForecast"], primary["detailedForecast"])))
		d["temperature_2m_max"] = append(d["temperature_2m_max"], maxFloat(nums))
		d["temperature_2m_min"] = append(d["temperature_2m_min"], minFloat(nums))
		d["apparent_temperature_max"] = append(d["apparent_temperature_max"], nil)
		d["precipitation_sum"] = append(d["precipitation_sum"], nil)
		d["precipitation_probability_max"] = append(d["precipitation_probability_max"], maxFloat(pops))
		d["wind_speed_10m_max"] = append(d["wind_speed_10m_max"], maxFloat(winds))
		d["uv_index_max"] = append(d["uv_index_max"], nil)
		d["sunrise"] = append(d["sunrise"], nil)
		d["sunset"] = append(d["sunset"], nil)
	}
	first := anyMap(periods[0])
	return weatherOKGo("nws", map[string]any{"current": map[string]any{"temperature_2m": toTempGo(first["temperature"], strings.ToLower(firstN(fmt.Sprint(first["temperatureUnit"]), 1)), cfg.TempUnit), "apparent_temperature": nil, "weather_code": textCodeGo(xOr(first["shortForecast"], first["detailedForecast"])), "wind_speed_10m": nwsWindGo(first, cfg), "relative_humidity_2m": nil}, "daily": d, "hourly": nil}), nil
}

func insideNWSCoverageGo(lat, lon float64) bool {
	boxes := [][4]float64{{24, 50.5, -125, -66}, {51, 72.5, -171, -129}, {18, 23.5, -162.5, -154}, {17, 19.5, -68.5, -64}, {12, 22, 143, 147}, {-15.5, -10, -172, -168}}
	for _, b := range boxes {
		if lat >= b[0] && lat <= b[1] && lon >= b[2] && lon <= b[3] {
			return true
		}
	}
	return false
}
func nwsWindGo(p map[string]any, cfg Config) any {
	text := fmt.Sprint(p["windSpeed"])
	v := firstNumberGo(text)
	unit := "mph"
	if strings.Contains(strings.ToLower(text), "km") {
		unit = "kmh"
	}
	return toWindGo(v, unit, cfg.WindUnit)
}
