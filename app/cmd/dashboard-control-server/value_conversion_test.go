package main

import (
	"testing"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func TestStringValueRequiresStrings(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want string
	}{
		{name: "trimmed string", in: "  family  ", want: "family"},
		{name: "missing", in: nil, want: ""},
		{name: "number", in: 42, want: ""},
		{name: "boolean", in: true, want: ""},
		{name: "object", in: map[string]any{"id": "x"}, want: ""},
		{name: "array", in: []any{"x"}, want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := jsonutil.StringValue(tc.in); got != tc.want {
				t.Fatalf("stringValue(%#v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestTextValueKeepsTolerantScalarCompatibility(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{in: nil, want: ""},
		{in: 42, want: "42"},
		{in: true, want: "true"},
		{in: "  retained  ", want: "retained"},
	}
	for _, tc := range cases {
		if got := jsonutil.TextValue(tc.in); got != tc.want {
			t.Fatalf("textValue(%#v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestBodyStringRejectsNonStringRequestFields(t *testing.T) {
	body := map[string]any{
		"title": "  Milk  ",
		"id":    42,
		"pin":   []any{"not", "a", "pin"},
	}
	if got := jsonutil.BodyString(body, "title"); got != "Milk" {
		t.Fatalf("bodyString title = %q, want Milk", got)
	}
	if got := jsonutil.BodyString(body, "id"); got != "" {
		t.Fatalf("numeric request id = %q, want empty", got)
	}
	if got := jsonutil.BodyString(body, "pin"); got != "" {
		t.Fatalf("array request pin = %q, want empty", got)
	}
}

func TestTodoRequestFieldsDoNotCoerceUnexpectedTypes(t *testing.T) {
	task := todoTaskFromBody("task-1", map[string]any{
		"title":      42,
		"status":     true,
		"importance": []any{"high"},
	})
	if task.Title != "Untitled task" || task.Status != "notStarted" || task.Importance != "normal" {
		t.Fatalf("non-string task fields changed defaults: %#v", task)
	}
	if _, err := todoTaskPatchRequests([]any{map[string]any{
		"id":    42,
		"patch": map[string]any{"title": "new"},
	}}); err == nil {
		t.Fatal("numeric patch id was accepted")
	}
}

func TestGraphStringAndListPatchStayStrict(t *testing.T) {
	value, present := todoGraphString(map[string]any{"title": 42}, "title")
	if !present || value != "" {
		t.Fatalf("todoGraphString numeric title = (%q, %t), want empty present value", value, present)
	}
	if _, present := todoGraphString(map[string]any{}, "title"); present {
		t.Fatal("missing Graph property must remain distinguishable from an empty value")
	}

	current := todoListInfo{ID: "list-1", DisplayName: "Family", WellknownName: "tasks", Origin: todoListOriginMicrosoft}
	patched, ok := todoListInfoPatchFromGraph(current, map[string]any{
		"id":                "list-1",
		"displayName":       42,
		"wellknownListName": []any{"tasks"},
	})
	if !ok || patched.DisplayName != "Family" || patched.WellknownName != "tasks" {
		t.Fatalf("malformed Graph list fields replaced known state: %#v", patched)
	}

	patched, ok = todoListInfoPatchFromGraph(current, map[string]any{
		"id":                "list-1",
		"displayName":       "  Family tasks  ",
		"wellknownListName": "  ",
	})
	if !ok || patched.DisplayName != "Family tasks" || patched.WellknownName != "" {
		t.Fatalf("valid Graph list fields did not normalize: %#v", patched)
	}
}

func TestStrictDecodedFieldsRejectNonStringValues(t *testing.T) {
	if record := familyBoardInboxPinRecord(map[string]any{
		"hash":       42,
		"salt":       "salt",
		"iterations": 200000,
	}); record != nil {
		t.Fatalf("numeric Board PIN hash was accepted: %#v", record)
	}

	ids := todoTaskIDsFromBody([]any{" task-1 ", 42, nil, []any{"task-2"}, "task-1"})
	if len(ids) != 1 || ids[0] != "task-1" {
		t.Fatalf("task IDs retained non-string or duplicate input: %#v", ids)
	}

	got := (&app{}).normalizeMessageEnabled([]any{" jokes ", 42, nil, []any{"facts"}})
	if len(got) != 1 || got[0] != "jokes" {
		t.Fatalf("message categories retained non-string input: %#v", got)
	}
}
