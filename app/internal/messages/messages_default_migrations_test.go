package messages

import (
	"reflect"
	"testing"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func TestReconcileDefaultCatalogMigratesOnlyDefaultCatalogState(t *testing.T) {
	payload := map[string]any{
		"messages": []any{map[string]any{"id": 7, "origin": "custom", "text": "Looking lovely today."}},
		"removedDefaults": []any{
			"Looking lovely today.",
			"looking lovely today.",
			"keep this unknown key",
		},
		"defaultEdits": map[string]any{
			"Looking lovely today.": map[string]any{"weight": 4},
			"keep this unknown key": map[string]any{"weight": 2},
		},
	}
	catalog := []any{map[string]any{
		"text":       "Lovely to have you around.",
		"legacyKeys": []any{"Looking lovely today."},
	}}

	if changed := reconcileDefaultCatalog(payload, catalog); changed < 2 {
		t.Fatalf("expected legacy default state migration, changed=%d payload=%#v", changed, payload)
	}
	removed := jsonutil.List(payload["removedDefaults"])
	wantRemoved := []any{"lovely to have you around.", "keep this unknown key"}
	if !reflect.DeepEqual(removed, wantRemoved) {
		t.Fatalf("removed defaults=%#v want %#v", removed, wantRemoved)
	}
	edits := jsonutil.Map(payload["defaultEdits"])
	if _, ok := edits["lovely to have you around."]; !ok {
		t.Fatalf("legacy default edit did not migrate: %#v", edits)
	}
	if _, ok := edits["keep this unknown key"]; !ok {
		t.Fatalf("unrelated default state was lost: %#v", edits)
	}
	custom := jsonutil.Map(jsonutil.List(payload["messages"])[0])
	if custom["text"] != "Looking lovely today." || custom["origin"] != "custom" {
		t.Fatalf("custom message was changed: %#v", custom)
	}
}

func TestDefaultKeyAliasesFromBodyUsesCurrentAndLegacyKeys(t *testing.T) {
	got := defaultKeyAliasesFromBody(map[string]any{
		"key":        "Lovely to have you around.",
		"legacyKeys": []any{"Looking lovely today.", " lovely to have you around. "},
	})
	want := []string{"lovely to have you around.", "looking lovely today."}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("aliases=%#v want %#v", got, want)
	}
}
