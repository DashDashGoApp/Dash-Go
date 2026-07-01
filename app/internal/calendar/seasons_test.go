package calendar

import (
	"testing"
	"time"
)

func TestSeasonEventsUseCalculatedDatesAndLocalizeTheCivilDay(t *testing.T) {
	utc := seasonEventsForLocation([]int{2026}, time.UTC)
	wantUTC := []string{"2026-03-20", "2026-06-21", "2026-09-23", "2026-12-21"}
	if len(utc) != len(wantUTC) {
		t.Fatalf("UTC season event count = %d, want %d", len(utc), len(wantUTC))
	}
	for index, event := range utc {
		if got := scheduleDateKey(event.Date); got != wantUTC[index] {
			t.Fatalf("UTC season[%d] = %s, want %s", index, got, wantUTC[index])
		}
	}
	chicago, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Fatal(err)
	}
	local := seasonEventsForLocation([]int{2026}, chicago)
	if got := scheduleDateKey(local[2].Date); got != "2026-09-22" {
		t.Fatalf("Chicago autumn civil day = %s, want 2026-09-22", got)
	}
}
