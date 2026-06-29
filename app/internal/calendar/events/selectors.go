package events

import (
	"slices"
	"strconv"
	"strings"
	"time"
)

type monthlyByDay struct{ weekday, ordinal int }

func parseMonthlyByDay(raw string) []monthlyByDay {
	ical := map[string]int{"SU": 0, "MO": 1, "TU": 2, "WE": 3, "TH": 4, "FR": 5, "SA": 6}
	out := []monthlyByDay{}
	for _, part := range strings.Split(raw, ",") {
		token := strings.ToUpper(strings.TrimSpace(part))
		if len(token) < 2 {
			continue
		}
		weekday, ok := ical[token[len(token)-2:]]
		if !ok {
			continue
		}
		ordinal := 0
		if prefix := token[:len(token)-2]; prefix != "" {
			n, err := strconv.Atoi(prefix)
			if err != nil || n == 0 {
				continue
			}
			ordinal = n
		}
		out = append(out, monthlyByDay{weekday, ordinal})
	}
	return out
}

func parseMonthNumbers(raw string, fallback int) []int {
	out := []int{}
	for _, part := range strings.Split(raw, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err == nil && n >= 1 && n <= 12 {
			out = append(out, n)
		}
	}
	if len(out) == 0 {
		out = []int{fallback}
	}
	slices.Sort(out)
	return uniqueInts(out)
}

func parseMonthDays(raw string, year int, month time.Month) []int {
	days := daysInMonth(year, month)
	out := []int{}
	for _, part := range strings.Split(raw, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || n == 0 {
			continue
		}
		day := n
		if n < 0 {
			day = days + n + 1
		}
		if day >= 1 && day <= days {
			out = append(out, day)
		}
	}
	slices.Sort(out)
	return uniqueInts(out)
}

func selectorDays(year int, month time.Month, fallbackDay int, r map[string]string, strictFallback bool) []int {
	byMonthDay := strings.TrimSpace(r["BYMONTHDAY"])
	byDay := parseMonthlyByDay(r["BYDAY"])
	values := []int{}
	switch {
	case byMonthDay != "":
		values = parseMonthDays(byMonthDay, year, month)
		if len(byDay) > 0 {
			values = filterMonthlyByDay(values, year, month, time.UTC, byDay)
		}
	case len(byDay) > 0:
		for _, spec := range byDay {
			if spec.ordinal != 0 {
				if day := nthWeekdayOfMonth(year, month, spec.weekday, spec.ordinal, time.UTC); day != 0 {
					values = append(values, day)
				}
				continue
			}
			for day := 1; day <= daysInMonth(year, month); day++ {
				if int(time.Date(year, month, day, 0, 0, 0, 0, time.UTC).Weekday()) == spec.weekday {
					values = append(values, day)
				}
			}
		}
	default:
		if fallbackDay >= 1 && fallbackDay <= daysInMonth(year, month) {
			values = append(values, fallbackDay)
		} else if !strictFallback {
			values = append(values, clamp(fallbackDay, 1, daysInMonth(year, month)))
		}
	}
	slices.Sort(values)
	return uniqueInts(values)
}

func filterMonthlyByDay(days []int, year int, month time.Month, loc *time.Location, specs []monthlyByDay) []int {
	filtered := days[:0]
	for _, day := range days {
		for _, spec := range specs {
			if int(time.Date(year, month, day, 0, 0, 0, 0, loc).Weekday()) != spec.weekday {
				continue
			}
			if spec.ordinal == 0 || nthWeekdayOfMonth(year, month, spec.weekday, spec.ordinal, loc) == day {
				filtered = append(filtered, day)
				break
			}
		}
	}
	return filtered
}

func nthWeekdayOfMonth(year int, month time.Month, weekday, ordinal int, loc *time.Location) int {
	if ordinal == 0 {
		return 0
	}
	days := daysInMonth(year, month)
	if ordinal > 0 {
		first := int(time.Date(year, month, 1, 0, 0, 0, 0, loc).Weekday())
		day := 1 + (weekday-first+7)%7 + (ordinal-1)*7
		if day <= days {
			return day
		}
		return 0
	}
	last := int(time.Date(year, month, days, 0, 0, 0, 0, loc).Weekday())
	day := days - (last-weekday+7)%7 - (-ordinal-1)*7
	if day >= 1 {
		return day
	}
	return 0
}

func uniqueInts(values []int) []int {
	if len(values) < 2 {
		return values
	}
	out := values[:0]
	for _, value := range values {
		if len(out) == 0 || out[len(out)-1] != value {
			out = append(out, value)
		}
	}
	return out
}
