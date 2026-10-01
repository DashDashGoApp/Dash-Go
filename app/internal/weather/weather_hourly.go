package weather

import (
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// Canonical hourly contract shared by every provider adapter and the browser
// blend: time[] is a provider-local naive wall clock in the same
// "2006-01-02T15:04" shape Open-Meteo returns with timezone=auto,
// temperature_2m[] is canonical Celsius, weather_code[] is a WMO code, and
// precipitation_probability[] is a percentage. weatherDisplaySourceGo converts
// temperatures to the household's display unit on the way out.
//
// The browser blend unions providers by exact time-string identity, so every
// adapter must normalize to this one shape. Before that, two providers
// describing the same hour produced different strings (an offset-bearing ISO
// timestamp beside a naive local one), the union found no overlap, and the
// hourly view silently collapsed to a single source.
const weatherHourlyLayout = "2006-01-02T15:04"

// weatherHourlyMaxRows bounds one provider's hourly block to the horizon the
// dashboard actually shows, so the aggregate payload stays small on a Pi Zero
// instead of carrying sixteen days of hours nothing renders.
const weatherHourlyMaxRows = 72

// googleWeatherHourlyPageSize is Google Weather's documented maximum page size
// for one hourly lookup; larger requests need nextPageToken pagination.
const googleWeatherHourlyPageSize = 24

type weatherHourlyBlockGo struct {
	time []any
	temp []any
	code []any
	pop  []any
}

func (b *weatherHourlyBlockGo) add(local string, temp, code, pop any) {
	if local == "" || len(b.time) >= weatherHourlyMaxRows {
		return
	}
	b.time = append(b.time, local)
	b.temp = append(b.temp, temp)
	b.code = append(b.code, code)
	b.pop = append(b.pop, pop)
}

func (b *weatherHourlyBlockGo) payload() any {
	if len(b.time) == 0 {
		return nil
	}
	return map[string]any{"time": b.time, "temperature_2m": b.temp, "weather_code": b.code, "precipitation_probability": b.pop}
}

// weatherHourlyLocalTime renders a UTC epoch into the provider's local wall
// clock. Providers describe their zone inconsistently: an IANA name, a seconds
// offset (OpenWeather), an hours offset (Pirate Weather), or nothing at all
// (Tomorrow.io, Weatherbit, AccuWeather). When a provider publishes bare UTC,
// the device zone is the right answer because the device is the household kiosk.
func weatherHourlyLocalTime(payload map[string]any, seconds float64) string {
	return weatherHourlyLocalTimeIn(jsonutil.StringValue(payload["timezone"]), payload, seconds)
}

func weatherHourlyLocalTimeIn(zone string, payload map[string]any, seconds float64) string {
	if seconds <= 0 {
		return ""
	}
	instant := time.Unix(int64(seconds), 0)
	if name := strings.TrimSpace(zone); name != "" {
		if location, err := time.LoadLocation(name); err == nil {
			return instant.In(location).Format(weatherHourlyLayout)
		}
	}
	if offset, ok := toFloatGo(payload["timezone_offset"]); ok {
		return instant.Add(time.Duration(offset * float64(time.Second))).UTC().Format(weatherHourlyLayout)
	}
	if offset, ok := toFloatGo(payload["offset"]); ok {
		return instant.Add(time.Duration(offset * float64(time.Hour))).UTC().Format(weatherHourlyLayout)
	}
	return instant.In(time.Local).Format(weatherHourlyLayout)
}

// weatherHourlyEpochFromValue accepts either a UTC epoch or an ISO-8601 instant.
func weatherHourlyEpochFromValue(v any) float64 {
	if seconds, ok := toFloatGo(v); ok && seconds > 0 {
		return seconds
	}
	if text := jsonutil.StringValue(v); text != "" {
		if instant, err := time.Parse(time.RFC3339, text); err == nil {
			return float64(instant.Unix())
		}
	}
	return 0
}

// weatherHourlyFromRFC3339 keeps an offset-bearing provider timestamp (NWS,
// Xweather) in that provider's own local wall clock.
func weatherHourlyFromRFC3339(v any) string {
	text := jsonutil.StringValue(v)
	if text == "" {
		return ""
	}
	if instant, err := time.Parse(time.RFC3339, text); err == nil {
		return instant.Format(weatherHourlyLayout)
	}
	if len(text) >= len(weatherHourlyLayout) {
		return text[:len(weatherHourlyLayout)]
	}
	return ""
}

// weatherHourlyFromUTCInstant converts a bare UTC instant (Google Weather) into
// the device's local wall clock.
func weatherHourlyFromUTCInstant(v any) string {
	text := jsonutil.StringValue(v)
	if text == "" {
		return ""
	}
	instant, err := time.Parse(time.RFC3339, text)
	if err != nil {
		return ""
	}
	return instant.In(time.Local).Format(weatherHourlyLayout)
}

// weatherHourlyFromLocalISO trims a provider-local ISO timestamp that already
// carries the location's wall clock but includes seconds (Visual Crossing).
func weatherHourlyFromLocalISO(v any) string {
	text := jsonutil.StringValue(v)
	if len(text) < len(weatherHourlyLayout) {
		return ""
	}
	return text[:len(weatherHourlyLayout)]
}

// trimOpenMeteoHourlyGo keeps Open-Meteo's hourly arrays inside the same horizon
// every other adapter uses.
func trimOpenMeteoHourlyGo(payload map[string]any) {
	hourly, ok := payload["hourly"].(map[string]any)
	if !ok {
		return
	}
	for key, raw := range hourly {
		if values, ok := raw.([]any); ok && len(values) > weatherHourlyMaxRows {
			hourly[key] = values[:weatherHourlyMaxRows]
		}
	}
}

// hourlyBlockFromCallGo maps a provider's separate hourly response into the
// canonical block, or nil when that call failed or returned nothing usable. A
// failed hourly call must leave the rest of the source intact.
func hourlyBlockFromCallGo(id string, result weatherCallResult, cfg Config) any {
	switch value := result.Value.(type) {
	case map[string]any:
		return weatherHourlyForProviderGo(id, value, cfg)
	case []any:
		if id == "accuweather" {
			return accuWeatherHourlyGo(value)
		}
	}
	return nil
}

// weatherHourlyForProviderGo maps hourly data that the provider already returned
// in the response Dash-Go fetched. Providers whose hourly data needs a separate
// request call this with that response instead, so one function owns every
// provider's hourly field names.
func weatherHourlyForProviderGo(id string, raw map[string]any, cfg Config) any {
	switch id {
	case "openweather":
		return openWeatherHourlyGo(raw)
	case "tomorrow":
		return tomorrowHourlyGo(raw)
	case "pirateweather":
		return pirateHourlyGo(raw)
	case "weatherapi":
		return weatherAPIHourlyGo(raw, cfg)
	case "visualcrossing":
		return visualCrossingHourlyGo(raw)
	case "weatherbit":
		return weatherbitHourlyGo(raw)
	case "accuweather":
		return accuWeatherHourlyGo(raw)
	case "xweather":
		return xweatherHourlyGo(raw)
	case "googleweather":
		return googleWeatherHourlyGo(raw)
	}
	return nil
}

// openWeatherHourlyGo: One Call 3.0 already returns 48 hours in the response
// Dash-Go fetches; only minutely and alerts are excluded.
func openWeatherHourlyGo(raw map[string]any) any {
	var block weatherHourlyBlockGo
	for _, item := range jsonutil.List(raw["hourly"]) {
		row := anyMap(item)
		condition := firstMap(jsonutil.List(row["weather"]))
		block.add(weatherHourlyLocalTime(raw, weatherHourlyEpochFromValue(row["dt"])), row["temp"], owCodeGo(condition["id"]), mult100(row["pop"]))
	}
	return block.payload()
}

// tomorrowHourlyGo: the forecast request already asks for 1h timesteps; only the
// current snapshot used the first hour before.
func tomorrowHourlyGo(raw map[string]any) any {
	var block weatherHourlyBlockGo
	for _, item := range jsonutil.List(anyMap(raw["timelines"])["hourly"]) {
		row := anyMap(item)
		values := anyMap(row["values"])
		block.add(weatherHourlyLocalTime(raw, weatherHourlyEpochFromValue(row["time"])), values["temperature"], textCodeGo(xOr(values["weatherCode"], values["weatherCodeFull"])), values["precipitationProbability"])
	}
	return block.payload()
}

// pirateHourlyGo: hourly.data already arrives and is already parsed for liquid
// precipitation totals, but never reached the dashboard as an hourly view.
func pirateHourlyGo(raw map[string]any) any {
	var block weatherHourlyBlockGo
	for _, item := range jsonutil.List(anyMap(raw["hourly"])["data"]) {
		row := anyMap(item)
		block.add(weatherHourlyLocalTime(raw, weatherHourlyEpochFromValue(row["time"])), row["temperature"], textCodeGo(xOr(row["summary"], row["icon"])), mult100(row["precipProbability"]))
	}
	return block.payload()
}

// weatherAPIHourlyGo: every hour of the requested days is in the same
// forecast.json response, and hourly is part of that provider's free plan.
func weatherAPIHourlyGo(raw map[string]any, cfg Config) any {
	unit := "f"
	if cfg.TempUnit == "celsius" {
		unit = "c"
	}
	zone := jsonutil.StringValue(anyMap(raw["location"])["tz_id"])
	var block weatherHourlyBlockGo
	for _, dayItem := range jsonutil.List(anyMap(raw["forecast"])["forecastday"]) {
		for _, hourItem := range jsonutil.List(anyMap(dayItem)["hour"]) {
			row := anyMap(hourItem)
			block.add(weatherHourlyLocalTimeIn(zone, raw, weatherHourlyEpochFromValue(row["time_epoch"])), choice(unit == "c", row["temp_c"], row["temp_f"]), textCodeGo(anyMap(row["condition"])["text"]), maxProbabilityGo(row["chance_of_rain"], row["chance_of_snow"]))
		}
	}
	return block.payload()
}

// visualCrossingHourlyGo reads days[].hours[] from the same forecast query once
// include=hours is requested. Visual Crossing counts a whole forecast response
// as one record, so the hourly array costs nothing extra against its quota.
func visualCrossingHourlyGo(raw map[string]any) any {
	var block weatherHourlyBlockGo
	for _, dayItem := range jsonutil.List(raw["days"]) {
		day := anyMap(dayItem)
		for _, hourItem := range jsonutil.List(day["hours"]) {
			row := anyMap(hourItem)
			block.add(visualCrossingHourlyStampGo(day["datetime"], row["datetime"]), row["temp"], textCodeGo(xOr(row["conditions"], row["icon"])), row["precipprob"])
		}
	}
	return block.payload()
}

// visualCrossingHourlyStampGo joins the parent day's date to the hour's clock.
// Visual Crossing publishes each hour's datetime as a time-only string
// ("00:00:00") that the caller must combine with the day it lives under, so
// passing it straight to the shared normalizer — which requires a full stamp —
// returned "" for every row. The block then came back nil, and the source
// reported success while contributing no hourly data at all. A full stamp is
// still accepted and passed straight through.
func visualCrossingHourlyStampGo(date, clock any) string {
	stamp := jsonutil.StringValue(clock)
	if strings.Contains(stamp, "T") {
		return weatherHourlyFromLocalISO(stamp)
	}
	day := jsonutil.StringValue(date)
	if len(day) < 10 || len(stamp) < 5 {
		return ""
	}
	return day[:10] + "T" + stamp[:5]
}

// weatherbitHourlyGo maps the dedicated hourly response. Weatherbit publishes
// UTC epochs with no zone field, so the device zone applies.
func weatherbitHourlyGo(raw map[string]any) any {
	var block weatherHourlyBlockGo
	for _, item := range jsonutil.List(raw["data"]) {
		row := anyMap(item)
		condition := anyMap(row["weather"])
		block.add(weatherHourlyLocalTime(raw, weatherHourlyEpochFromValue(row["ts"])), row["temp"], textCodeGo(xOr(condition["description"], condition["code"])), row["pop"])
	}
	return block.payload()
}

// accuWeatherHourlyGo reads the documented 12-hour hourly response. AccuWeather
// returns both Metric and Imperial objects, so canonical Celsius is always
// available regardless of the requested flag.
func accuWeatherHourlyGo(raw any) any {
	var block weatherHourlyBlockGo
	for _, item := range jsonutil.List(raw) {
		row := anyMap(item)
		local := weatherHourlyFromRFC3339(row["DateTime"])
		if local == "" {
			local = weatherHourlyLocalTime(nil, weatherHourlyEpochFromValue(row["EpochDateTime"]))
		}
		block.add(local, anyMap(anyMap(row["Temperature"])["Metric"])["Value"], textCodeGo(row["IconPhrase"]), row["PrecipitationProbability"])
	}
	return block.payload()
}

// xweatherHourlyGo reads the credentialed hourly response; tempC is always
// present beside tempF.
func xweatherHourlyGo(raw map[string]any) any {
	periods := []any{}
	for _, response := range jsonutil.List(raw["response"]) {
		periods = append(periods, jsonutil.List(anyMap(response)["periods"])...)
	}
	var block weatherHourlyBlockGo
	for _, item := range periods {
		row := anyMap(item)
		block.add(weatherHourlyFromRFC3339(row["dateTimeISO"]), row["tempC"], textCodeGo(xOr(row["weather"], xOr(row["weatherPrimary"], row["icon"]))), row["pop"])
	}
	return block.payload()
}

// googleWeatherHourlyGo reads forecast/hours:lookup, whose intervals are bare
// UTC instants.
func googleWeatherHourlyGo(raw map[string]any) any {
	var block weatherHourlyBlockGo
	for _, item := range jsonutil.List(raw["forecastHours"]) {
		row := anyMap(item)
		block.add(weatherHourlyFromUTCInstant(anyMap(row["interval"])["startTime"]), degreesGo(row["temperature"]), textCodeGo(conditionTextGo(row)), anyMap(anyMap(row["precipitation"])["probability"])["percent"])
	}
	return block.payload()
}

// nwsHourlyGo maps National Weather Service hourly periods, whose startTime
// already carries the location's UTC offset.
func nwsHourlyGo(periods []any) any {
	var block weatherHourlyBlockGo
	for _, item := range periods {
		row := anyMap(item)
		unit := strings.ToLower(firstN(jsonutil.TextValue(row["temperatureUnit"]), 1))
		block.add(weatherHourlyFromRFC3339(row["startTime"]), toTempGo(row["temperature"], unit, "c"), textCodeGo(xOr(row["shortForecast"], row["detailedForecast"])), popGo(row))
	}
	return block.payload()
}
