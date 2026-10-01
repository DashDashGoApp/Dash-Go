package weather

import (
	"math"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// Providers disagree about publishing sunrise and sunset at all: WeatherAPI,
// Weatherbit, and NWS return none, so a household that disabled Open-Meteo saw
// an empty sun row. Dash-Go derives both from the configured coordinates with
// the NOAA solar-position approximation instead of depending on whichever
// provider happens to be enabled. Provider-supplied values are always kept.
const (
	weatherSunZenithDegrees = 90.833 // official sunrise/sunset: 90°50' including refraction
	weatherMinutesPerDay    = 720.0  // minutes from UTC midnight to solar noon
)

// weatherSunTimesForDay returns sunrise and sunset for one local date, or zero
// times during polar day or polar night.
func weatherSunTimesForDay(lat, lon float64, date time.Time, zone *time.Location) (time.Time, time.Time) {
	if zone == nil {
		zone = time.Local
	}
	start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, zone)
	gamma := 2 * math.Pi / 365.0 * (float64(start.YearDay()) - 1 + 0.5)
	eqTime := 229.18 * (0.000075 + 0.001868*math.Cos(gamma) - 0.032077*math.Sin(gamma) - 0.014615*math.Cos(2*gamma) - 0.040849*math.Sin(2*gamma))
	declination := 0.006918 - 0.399912*math.Cos(gamma) + 0.070257*math.Sin(gamma) - 0.006758*math.Cos(2*gamma) + 0.000907*math.Sin(2*gamma) - 0.002697*math.Cos(3*gamma) + 0.00148*math.Sin(3*gamma)
	latRad := lat * math.Pi / 180
	cosHourAngle := math.Cos(weatherSunZenithDegrees*math.Pi/180)/(math.Cos(latRad)*math.Cos(declination)) - math.Tan(latRad)*math.Tan(declination)
	if math.IsNaN(cosHourAngle) || cosHourAngle > 1 || cosHourAngle < -1 {
		return time.Time{}, time.Time{}
	}
	hourAngle := math.Acos(cosHourAngle) * 180 / math.Pi
	riseMinutes := weatherMinutesPerDay - 4*(lon+hourAngle) - eqTime
	setMinutes := weatherMinutesPerDay - 4*(lon-hourAngle) - eqTime
	return weatherSunInstant(start, riseMinutes, zone), weatherSunInstant(start, setMinutes, zone)
}

// weatherSunInstant turns minutes-from-UTC-midnight into a real instant in the
// household zone.
func weatherSunInstant(start time.Time, minutes float64, zone *time.Location) time.Time {
	midnight := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	return midnight.Add(time.Duration(minutes * float64(time.Minute))).In(zone)
}

// weatherDailyArrayGo reads one daily array from a source, regardless of how the
// source was built: adapters that assemble their own daily section use
// map[string][]any, while a decoded provider payload (Open-Meteo) uses
// map[string]any with array values. Reading only one shape silently skips every
// provider that uses the other.
func weatherDailyArrayGo(source map[string]any, key string) ([]any, bool) {
	switch daily := source["daily"].(type) {
	case map[string][]any:
		values, ok := daily[key]
		return values, ok
	case map[string]any:
		values, ok := daily[key].([]any)
		return values, ok
	}
	return nil, false
}

// weatherDailyArraySetGo stores one daily array back into a source without
// replacing the daily section itself.
func weatherDailyArraySetGo(source map[string]any, key string, values []any) {
	switch daily := source["daily"].(type) {
	case map[string][]any:
		daily[key] = values
	case map[string]any:
		daily[key] = values
	}
}

// fillDerivedSunTimesGo fills missing sunrise/sunset values for every source, so
// no provider combination can produce an empty sun row.
func fillDerivedSunTimesGo(sources []any, cfg Config) {
	for _, raw := range sources {
		source := anyMap(raw)
		if len(source) == 0 {
			continue
		}
		dates, ok := weatherDailyArrayGo(source, "time")
		if !ok || len(dates) == 0 {
			continue
		}
		rises, haveRises := weatherDailyArrayGo(source, "sunrise")
		sets, haveSets := weatherDailyArrayGo(source, "sunset")
		if !haveRises || !haveSets || len(rises) < len(dates) || len(sets) < len(dates) {
			continue
		}
		for index, item := range dates {
			date, err := time.ParseInLocation("2006-01-02", jsonutil.TextValue(item), time.Local)
			if err != nil {
				continue
			}
			rise, set := weatherSunTimesForDay(cfg.Lat, cfg.Lon, date, time.Local)
			if jsonutil.TextValue(rises[index]) == "" && !rise.IsZero() {
				rises[index] = rise.Format(time.RFC3339)
			}
			if jsonutil.TextValue(sets[index]) == "" && !set.IsZero() {
				sets[index] = set.Format(time.RFC3339)
			}
		}
		weatherDailyArraySetGo(source, "sunrise", rises)
		weatherDailyArraySetGo(source, "sunset", sets)
	}
}
