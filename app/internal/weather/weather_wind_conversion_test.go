package weather

import (
	"math"
	"testing"
)

func TestWindToMetersPerSecondConvertsFromImperialAndMetric(t *testing.T) {
	cases := []struct {
		value  float64
		source string
		target string
		want   float64
	}{
		{20, "mph", "ms", 8.9408},   // was returned raw as 20
		{20, "kmh", "ms", 20 / 3.6}, // was returned raw as 20
		{20, "kph", "ms", 20 / 3.6},
		{20, "ms", "ms", 20},
		{20, "mph", "mph", 20},
		{20, "mph", "kmh", 32.18688},
		{20, "kmh", "mph", 20 / 1.609344},
	}
	for _, c := range cases {
		got, ok := toWindGo(c.value, c.source, c.target).(float64)
		if !ok {
			t.Fatalf("%s->%s: not a float", c.source, c.target)
		}
		if math.Abs(got-c.want) > 1e-9 {
			t.Fatalf("%s->%s: got %v want %v", c.source, c.target, got, c.want)
		}
	}
}
