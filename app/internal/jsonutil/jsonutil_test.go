package jsonutil

import "testing"

func TestDecodedValueContracts(t *testing.T) {
	if got := StringValue("  family  "); got != "family" {
		t.Fatalf("StringValue trimmed string = %q", got)
	}
	for _, value := range []any{nil, 42, true, []any{"x"}, map[string]any{"id": "x"}} {
		if got := StringValue(value); got != "" {
			t.Fatalf("StringValue(%#v) = %q, want empty", value, got)
		}
	}
	if got := TextValue(nil); got != "" {
		t.Fatalf("TextValue(nil) = %q, want empty", got)
	}
	if got := TextValue(42); got != "42" {
		t.Fatalf("TextValue number = %q", got)
	}
	if got := BodyString(map[string]any{"title": "  Milk  ", "id": 42}, "title"); got != "Milk" {
		t.Fatalf("BodyString title = %q", got)
	}
	if got := BodyString(map[string]any{"id": 42}, "id"); got != "" {
		t.Fatalf("BodyString numeric field = %q, want empty", got)
	}
}

func TestJSONCollectionAndScalarHelpers(t *testing.T) {
	if !Truthy(true) || !Truthy("TRUE") || !Truthy("1") || Truthy("yes") || Truthy(1) {
		t.Fatal("Truthy did not preserve supported forms")
	}
	if got := Int(" 42 ", 7); got != 42 {
		t.Fatalf("Int string = %d", got)
	}
	if got := Int(3.8, 7); got != 3 {
		t.Fatalf("Int float = %d", got)
	}
	if got := Int([]any{3}, 7); got != 7 {
		t.Fatalf("Int fallback = %d", got)
	}
	if got := List("not a list"); got == nil || len(got) != 0 {
		t.Fatalf("List fallback = %#v", got)
	}
	if got := Map("not an object"); got == nil || len(got) != 0 {
		t.Fatalf("Map fallback = %#v", got)
	}
	original := map[string]any{"name": "Family"}
	clone := CloneMap(original)
	clone["name"] = "Changed"
	if original["name"] != "Family" {
		t.Fatalf("CloneMap mutated source: %#v", original)
	}
	if got := CloneMap(nil); got == nil || len(got) != 0 {
		t.Fatalf("CloneMap(nil) = %#v", got)
	}
}
