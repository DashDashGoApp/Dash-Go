package events

import (
	"testing"
	"time"
)

func TestMonthlyRecurrenceRFC5545ByMonthDayAndByDay(t *testing.T) {
	loc := time.UTC
	windowStart := time.Date(2026, 1, 1, 0, 0, 0, 0, loc)
	windowEnd := time.Date(2026, 8, 31, 23, 59, 59, 0, loc)
	tests := []struct {
		name  string
		start time.Time
		rule  string
		want  []string
	}{
		{
			name:  "skip overflow BYMONTHDAY",
			start: time.Date(2026, 1, 31, 9, 0, 0, 0, loc),
			rule:  "FREQ=MONTHLY;BYMONTHDAY=31",
			want:  []string{"20260131", "20260331", "20260531", "20260731", "20260831"},
		},
		{
			name:  "negative BYMONTHDAY last day",
			start: time.Date(2026, 1, 31, 9, 0, 0, 0, loc),
			rule:  "FREQ=MONTHLY;BYMONTHDAY=-1;COUNT=4",
			want:  []string{"20260131", "20260228", "20260331", "20260430"},
		},
		{
			name:  "positional BYDAY",
			start: time.Date(2026, 1, 13, 9, 0, 0, 0, loc),
			rule:  "FREQ=MONTHLY;BYDAY=2TU;COUNT=4",
			want:  []string{"20260113", "20260210", "20260310", "20260414"},
		},
		{
			name:  "multiple positional BYDAY values",
			start: time.Date(2026, 1, 13, 9, 0, 0, 0, loc),
			rule:  "FREQ=MONTHLY;BYDAY=2TU,-1FR;COUNT=4",
			want:  []string{"20260113", "20260130", "20260210", "20260227"},
		},
		{
			name:  "multiple BYMONTHDAY values",
			start: time.Date(2026, 1, 1, 9, 0, 0, 0, loc),
			rule:  "FREQ=MONTHLY;BYMONTHDAY=1,15;COUNT=6",
			want:  []string{"20260101", "20260115", "20260201", "20260215", "20260301", "20260315"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			instances := expand(ICSEvent{Start: tc.start, RRule: tc.rule}, windowStart, windowEnd)
			got := make([]string, 0, len(instances))
			for _, instance := range instances {
				got = append(got, instance.Start.Format("20060102"))
			}
			if len(got) != len(tc.want) {
				t.Fatalf("dates=%v want=%v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("dates=%v want=%v", got, tc.want)
				}
			}
		})
	}
}
