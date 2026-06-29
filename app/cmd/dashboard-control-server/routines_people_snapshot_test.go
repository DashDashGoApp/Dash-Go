package main

import (
	"testing"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func TestRoutineOccurrenceAndHistoryKeepPersonSnapshot(t *testing.T) {
	day := routinesToday()
	payload := normalizeRoutinesPayload(map[string]any{
		"people": []any{
			map[string]any{"id": "sam", "name": "Sam Renamed", "state": "active"},
		},
		"routines": []any{
			map[string]any{
				"id": "morning", "title": "Morning",
				"steps": []any{map[string]any{"id": "one", "text": "One"}},
				"assignments": []any{map[string]any{
					"id": "sam-am", "personId": "sam",
					"schedule": map[string]any{"kind": "days", "startOn": day},
				}},
			},
		},
		"occurrences": []any{
			map[string]any{"id": "saved", "routineId": "morning", "assignmentId": "sam-am", "personId": "sam", "personNameSnapshot": "Sam Before Rename", "date": day, "state": "active"},
		},
	})
	items := routinesOccurrencesForDay(payload, day)
	if len(items) != 1 || items[0]["personName"] != "Sam Before Rename" {
		t.Fatalf("stored routine occurrence lost immutable person snapshot: %#v", items)
	}
	routinesAppendHistory(payload, "completed", jsonutil.Map(items[0]))
	history := jsonutil.Map(jsonutil.List(payload["history"])[0])
	if history["personName"] != "Sam Before Rename" {
		t.Fatalf("routine history lost occurrence snapshot: %#v", history)
	}
}
