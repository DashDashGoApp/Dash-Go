package events

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type parityFixture struct {
	Name     string `json:"name"`
	ICS      string `json:"ics"`
	Expected struct {
		Count       int      `json:"count"`
		Titles      []string `json:"titles"`
		DurationsMs []int64  `json:"durationsMs"`
		SkipCount   int      `json:"skipCount"`
	} `json:"expected"`
}

func TestSharedCalendarParserParityCorpus(t *testing.T) {
	path := filepath.Join("..", "..", "..", "tests", "fixtures", "calendar-parity", "corpus.json")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []parityFixture
	if err := json.Unmarshal(body, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			events := parseICS(fixture.ICS, CalendarSource{URL: "calendars/parity.ics"})
			if len(events) != fixture.Expected.Count {
				t.Fatalf("count=%d want %d", len(events), fixture.Expected.Count)
			}
			for index, event := range events {
				if index < len(fixture.Expected.Titles) && event.Title != fixture.Expected.Titles[index] {
					t.Fatalf("title=%q", event.Title)
				}
				duration := int64(0)
				if event.End != nil {
					duration = event.End.Sub(event.Start).Milliseconds()
				}
				if index < len(fixture.Expected.DurationsMs) && duration != fixture.Expected.DurationsMs[index] {
					t.Fatalf("duration=%d", duration)
				}
				skips := len(event.Skip) + len(event.SkipDays)
				if skips != fixture.Expected.SkipCount {
					t.Fatalf("skip count=%d want %d", skips, fixture.Expected.SkipCount)
				}
			}
		})
	}
}
