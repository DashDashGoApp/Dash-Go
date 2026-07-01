package weather

import (
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func popGo(p map[string]any) any { return anyMap(p["probabilityOfPrecipitation"])["value"] }
func emptyDailyGo() map[string][]any {
	return map[string][]any{"time": {}, "weather_code": {}, "temperature_2m_max": {}, "temperature_2m_min": {}, "apparent_temperature_max": {}, "precipitation_sum": {}, "precipitation_probability_max": {}, "wind_speed_10m_max": {}, "uv_index_max": {}, "sunrise": {}, "sunset": {}}
}
func weatherOKGo(id string, data map[string]any) map[string]any {
	data["_source"] = id
	data["_sourceLabel"] = weatherProviderLabel(id)
	data["_fetchedAt"] = time.Now().Unix()
	return data
}
func anyMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}
func firstMap(xs []any) map[string]any {
	if len(xs) > 0 {
		return anyMap(xs[0])
	}
	return map[string]any{}
}
func choice(ok bool, a, b any) any {
	if ok {
		return a
	}
	return b
}
func xOr(a, b any) any {
	if jsonutil.TextValue(a) == "" {
		return b
	}
	return a
}
func mult100(v any) any {
	if f, ok := toFloatGo(v); ok {
		return f * 100
	}
	return nil
}
func toFloatGo(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		if !math.IsNaN(x) && !math.IsInf(x, 0) {
			return x, true
		}
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f, err == nil
	}
	return 0, false
}
func toTempGo(v any, unit, target string) any {
	f, ok := toFloatGo(v)
	if !ok {
		return nil
	}
	unit = strings.ToLower(unit)
	target = normalizeTempUnit(target)
	if target == "celsius" && strings.HasPrefix(unit, "f") {
		return (f - 32) * 5 / 9
	}
	if target != "celsius" && strings.HasPrefix(unit, "c") {
		return f*9/5 + 32
	}
	return f
}

// precipitationMMGo converts a provider precipitation quantity to the
// dashboard's canonical daily-total unit: millimetres of liquid water.
// Adapters must convert at their boundary so weather blending never mixes
// inches, centimetres, and millimetres.
func precipitationMMGo(v any, unit string) any {
	f, ok := toFloatGo(v)
	if !ok || f < 0 {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "mm", "millimeter", "millimeters", "millimetre", "millimetres":
		return f
	case "cm", "centimeter", "centimeters", "centimetre", "centimetres":
		return f * 10
	case "in", "inch", "inches":
		return f * 25.4
	default:
		return nil
	}
}

// precipitationSumMMGo sums fields that a provider already documents as
// millimetres (for example separate rain and snow liquid-water fields).
func precipitationSumMMGo(values ...any) any {
	sum := 0.0
	found := false
	for _, value := range values {
		f, ok := toFloatGo(value)
		if !ok || f < 0 {
			continue
		}
		sum += f
		found = true
	}
	if !found {
		return 0.0
	}
	return sum
}

// googlePrecipitationMMGo respects the unit supplied beside Google's qpf
// quantity and uses the requested unit system only when a response omits it.
func googlePrecipitationMMGo(v any, fallbackUnit string) any {
	qpf := anyMap(v)
	unit := strings.TrimSpace(fmt.Sprint(qpf["unit"]))
	if unit == "" || unit == "<nil>" {
		unit = fallbackUnit
	}
	return precipitationMMGo(qpf["quantity"], unit)
}

func toWindGo(v any, unit, target string) any {
	f, ok := toFloatGo(v)
	if !ok {
		return nil
	}
	unit = strings.ToLower(unit)
	target = normalizeWindUnit(target)
	if unit == "ms" || unit == "m/s" {
		if target == "kmh" {
			return f * 3.6
		}
		if target == "mph" {
			return f / 0.44704
		}
		return f
	}
	if target == "kmh" && unit == "mph" {
		return f * 1.609344
	}
	if target == "mph" && (unit == "kmh" || unit == "kph") {
		return f / 1.609344
	}
	return f
}
func textCodeGo(v any) int {
	text := strings.ToLower(fmt.Sprint(v))
	switch {
	case strings.Contains(text, "thunder") || strings.Contains(text, "storm"):
		return 95
	case reWeatherSnow.MatchString(text):
		return 71
	case reWeatherRain.MatchString(text):
		return 61
	case reWeatherFog.MatchString(text):
		return 45
	case strings.Contains(text, "overcast") || strings.Contains(text, "cloudy"):
		return 3
	case strings.Contains(text, "partly") || strings.Contains(text, "mostly sunny") || strings.Contains(text, "few"):
		return 2
	case strings.Contains(text, "clear") || strings.Contains(text, "sunny"):
		return 0
	default:
		return 3
	}
}
func owCodeGo(v any) int {
	f, _ := toFloatGo(v)
	c := int(f)
	switch {
	case c >= 200 && c < 300:
		return 95
	case c >= 300 && c < 600:
		if c >= 500 {
			return 61
		}
		return 51
	case c >= 600 && c < 700:
		return 71
	case c >= 700 && c < 800:
		return 45
	case c == 800:
		return 0
	case c == 801:
		return 2
	default:
		return 3
	}
}
func tsDateGo(v any) string {
	if f, ok := toFloatGo(v); ok && f > 0 {
		return time.Unix(int64(f), 0).UTC().Format("2006-01-02")
	}
	return firstN(fmt.Sprint(v), 10)
}
func tsISOGo(v any) any {
	if f, ok := toFloatGo(v); ok && f > 0 {
		return time.Unix(int64(f), 0).UTC().Format(time.RFC3339)
	}
	return nil
}
func maxFloat(xs []float64) any {
	if len(xs) == 0 {
		return nil
	}
	return slices.Max(xs)
}
func minFloat(xs []float64) any {
	if len(xs) == 0 {
		return nil
	}
	return slices.Min(xs)
}
func firstNumberGo(s string) any {
	m := reFirstNumber.FindString(s)
	if m == "" {
		return nil
	}
	f, _ := strconv.ParseFloat(m, 64)
	return f
}
func degreesGo(v any) any { return anyMap(v)["degrees"] }
func conditionTextGo(v any) any {
	wc := anyMap(anyMap(v)["weatherCondition"])
	desc := anyMap(wc["description"])
	return xOr(desc["text"], wc["type"])
}
func googleDateGo(v map[string]any) string {
	d := anyMap(v["displayDate"])
	if y, ok := toFloatGo(d["year"]); ok {
		m, _ := toFloatGo(d["month"])
		day, _ := toFloatGo(d["day"])
		return fmt.Sprintf("%04d-%02d-%02d", int(y), int(m), int(day))
	}
	return firstN(fmt.Sprint(anyMap(v["interval"])["startTime"]), 10)
}
func googleWindGo(v any, cfg Config) any {
	sp := anyMap(anyMap(v)["speed"])
	unit := strings.ToUpper(fmt.Sprint(sp["unit"]))
	if unit == "MILES_PER_HOUR" {
		return toWindGo(sp["value"], "mph", cfg.WindUnit)
	}
	if unit == "KILOMETERS_PER_HOUR" {
		return toWindGo(sp["value"], "kmh", cfg.WindUnit)
	}
	return sp["value"]
}
func mapCopy(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	maps.Copy(out, m)
	return out
}
func mapMerge(a, b map[string]any) map[string]any {
	out := make(map[string]any, len(a)+len(b))
	maps.Copy(out, a)
	maps.Copy(out, b)
	return out
}
func stringListContains(xs []string, needle string) bool {
	for _, x := range xs {
		if strings.EqualFold(strings.TrimSpace(x), needle) {
			return true
		}
	}
	return false
}
func firstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
