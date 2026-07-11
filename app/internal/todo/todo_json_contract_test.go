package todo

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTodoTaskClosedJSONBoundariesAndZeroOmission(t *testing.T) {
	task := todoTask{
		ID:         "task-1",
		Title:      "Milk",
		Status:     "notStarted",
		Importance: "normal",
		DueDateTime: &todoDateTimeTimeZone{
			DateTime: "2026-07-12T00:00:00.0000000",
			TimeZone: "America/Chicago",
		},
		Body: &todoItemBody{
			Content:     "Two percent",
			ContentType: "text",
		},
	}

	encoded, err := json.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}
	got := string(encoded)
	want := `{"id":"task-1","title":"Milk","status":"notStarted","importance":"normal","dueDateTime":{"dateTime":"2026-07-12T00:00:00.0000000","timeZone":"America/Chicago"},"body":{"content":"Two percent","contentType":"text"}}`
	if got != want {
		t.Fatalf("todo task JSON changed:\n got: %s\nwant: %s", got, want)
	}
	for _, privateZero := range []string{`"_syncFailed"`, `"_cloudIgnored"`, `"_etag"`} {
		if strings.Contains(got, privateZero) {
			t.Fatalf("zero private field %s was not omitted: %s", privateZero, got)
		}
	}
}

func TestTodoTaskGraphPatchDropsUnknownClosedBoundaryFields(t *testing.T) {
	got := todoTaskPatchFromGraph(todoTask{ID: "task-1"}, map[string]any{
		"dueDateTime": map[string]any{
			"dateTime": "2026-07-12T00:00:00.0000000",
			"timeZone": "America/Chicago",
			"unknown":  "must not enter the durable cache",
		},
		"body": map[string]any{
			"content":     "Remember coupons",
			"contentType": "text",
			"unknown":     "must not enter the durable cache",
		},
	})
	if got.DueDateTime == nil || got.DueDateTime.DateTime == "" || got.DueDateTime.TimeZone != "America/Chicago" {
		t.Fatalf("typed due date was not preserved: %#v", got.DueDateTime)
	}
	if got.Body == nil || got.Body.Content != "Remember coupons" || got.Body.ContentType != "text" {
		t.Fatalf("typed item body was not preserved: %#v", got.Body)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "unknown") {
		t.Fatalf("unknown provider fields escaped the typed boundary: %s", encoded)
	}
}
