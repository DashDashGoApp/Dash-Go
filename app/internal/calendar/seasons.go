package calendar

import (
	"math"
	"time"
)

// Season labels use the conventional Northern Hemisphere names. The event
// instant is calculated offline then localized to the dashboard timezone so
// its all-day calendar entry follows the household's local civil date.
type seasonSpec struct {
	label string
	uid   string
	a     float64
	b     float64
	c     float64
	d     float64
	e     float64
}

var northernSeasonSpecs = []seasonSpec{
	{"Spring begins", "spring", 2451623.80984, 365242.37404, 0.05169, -0.00411, -0.00057},
	{"Summer begins", "summer", 2451716.56767, 365241.62603, 0.00325, 0.00888, -0.00030},
	{"Autumn begins", "autumn", 2451810.21715, 365242.01767, -0.11575, 0.00337, 0.00078},
	{"Winter begins", "winter", 2451900.05952, 365242.74049, -0.06223, -0.00823, 0.00032},
}

type seasonTerm struct{ a, b, c float64 }

var seasonTerms = []seasonTerm{
	{485, 324.96, 1934.136}, {203, 337.23, 32964.467}, {199, 342.08, 20.186}, {182, 27.85, 445267.112},
	{156, 73.14, 45036.886}, {136, 171.52, 22518.443}, {77, 222.54, 65928.934}, {74, 296.72, 3034.906},
	{70, 243.58, 9037.513}, {58, 119.81, 33718.147}, {52, 297.17, 150.678}, {50, 21.02, 2281.226},
	{45, 247.54, 29929.562}, {44, 325.15, 31555.956}, {29, 60.93, 4443.417}, {18, 155.12, 67555.328},
	{17, 288.79, 4562.452}, {16, 198.04, 62894.029}, {14, 199.76, 31436.921}, {12, 95.39, 14577.848},
	{12, 287.11, 31931.756}, {12, 320.81, 34777.259}, {9, 227.73, 1222.114}, {8, 15.45, 16859.074},
}

func SeasonEvents(years []int) []Event {
	location, err := time.LoadLocation(LocalTimezoneName())
	if err != nil {
		location = time.UTC
	}
	return seasonEventsForLocation(years, location)
}

func seasonEventsForLocation(years []int, location *time.Location) []Event {
	if location == nil {
		location = time.UTC
	}
	out := make([]Event, 0, len(years)*len(northernSeasonSpecs))
	for _, year := range years {
		for index, spec := range northernSeasonSpecs {
			moment := seasonMomentUTC(year, index).In(location)
			out = append(out, AllDayEvent(moment.Year(), moment.Month(), moment.Day(), spec.label, spec.uid))
		}
	}
	return out
}

// seasonMomentUTC is the Meeus periodic-term approximation for 1000-3000.
// Its sub-day precision is more than enough for selecting the local civil date.
func seasonMomentUTC(year, index int) time.Time {
	spec := northernSeasonSpecs[index]
	y := float64(year-2000) / 1000
	jde0 := spec.a + spec.b*y + spec.c*y*y + spec.d*y*y*y + spec.e*y*y*y*y
	t := (jde0 - 2451545.0) / 36525
	w := radians(35999.373*t - 2.47)
	deltaLambda := 1 + 0.0334*math.Cos(w) + 0.0007*math.Cos(2*w)
	sum := 0.0
	for _, term := range seasonTerms {
		sum += term.a * math.Cos(radians(term.b+term.c*t))
	}
	jde := jde0 + 0.00001*sum/deltaLambda
	// ΔT around the supported modern dashboard horizon is roughly 69 seconds.
	// A conservative one-minute correction avoids treating TT as UTC at a date edge.
	unixSeconds := (jde-2440587.5)*86400 - 69
	seconds, nanos := math.Modf(unixSeconds)
	return time.Unix(int64(seconds), int64(math.Round(nanos*1e9))).UTC()
}

func radians(degrees float64) float64 { return degrees * math.Pi / 180 }
