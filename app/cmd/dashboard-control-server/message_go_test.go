package main

import (
	"context"
	"testing"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func TestGoMessageRefreshLocalFallback(t *testing.T) {
	a := testApp(t)
	a.ensureDirs()
	a.saveMessageSourcesStatus(map[string]any{"enabled": []any{"quotes", "jokes"}, "updatedAt": int64(1)})
	got := a.refreshMessages(context.Background(), false, false)
	if got["generator"] != "go" {
		t.Fatalf("expected go generator, got %#v", got["generator"])
	}
	if len(jsonutil.List(got["items"])) == 0 {
		t.Fatalf("expected local fallback messages")
	}
	if len(jsonutil.List(got["sourceStatus"])) != 2 {
		t.Fatalf("expected two source status rows, got %#v", got["sourceStatus"])
	}
}

func TestGoMessageEnabledKeepsOnlyCurrentCategories(t *testing.T) {
	a := testApp(t)
	enabled := a.normalizeMessageEnabled([]any{"quotes-calm", "jokes-dad", "quotes", "bad"})
	if len(enabled) != 1 || enabled[0] != "quotes" {
		t.Fatalf("retired category IDs were retained: %#v", enabled)
	}
}

func TestGoMessageOverridesApply(t *testing.T) {
	items := []any{
		map[string]any{"id": "keep", "text": "Old", "source": "quotes", "weight": 1},
		map[string]any{"id": "gone", "text": "Gone", "source": "quotes", "weight": 1},
	}
	ov := map[string]any{"removed": []any{"gone"}, "edits": map[string]any{"keep": map[string]any{"text": "New", "weight": 7}}}
	got := applyMessageOverrides(items, ov)
	if len(got) != 1 {
		t.Fatalf("expected one item after remove, got %#v", got)
	}
	m := jsonutil.Map(got[0])
	if m["text"] != "New" || jsonutil.Int(m["weight"], 0) != 7 || m["edited"] != true {
		t.Fatalf("edit not applied: %#v", m)
	}
}
