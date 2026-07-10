package events

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

func epochMs(t time.Time) int64 { return t.UnixMilli() }

func clamp(n, lo, hi int) int { return max(lo, min(hi, n)) }

func daysInMonth(y int, m time.Month) int { return time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC).Day() }

// calendarDayDiff compares civil dates without letting a DST-short/long day
// distort a recurrence fast-forward calculation.
func calendarDayDiff(start, end time.Time) int {
	a := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	b := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)
	return int(b.Sub(a).Hours() / 24)
}

func monthDiff(start, end time.Time) int {
	return (end.Year()-start.Year())*12 + int(end.Month()-start.Month())
}

func anyInt64(v any, def int64) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case float64:
		return int64(x)
	case json.Number:
		if n, err := x.Int64(); err == nil {
			return n
		}
	case string:
		if n, err := strconv.ParseInt(strings.TrimSpace(x), 10, 64); err == nil {
			return n
		}
	}
	return def
}

func defaultString(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
