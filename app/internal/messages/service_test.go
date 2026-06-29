package messages

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func testService(t *testing.T) *Service {
	t.Helper()
	root := t.TempDir()
	config := filepath.Join(root, "config")
	if err := os.MkdirAll(config, 0755); err != nil {
		t.Fatal(err)
	}
	return New(ServiceConfig{
		Home:             root,
		ConfigDir:        config,
		ConfigLocal:      filepath.Join(config, "config.local.js"),
		CelebrationsFile: filepath.Join(root, ".dashboard-celebrations"),
		GenerateDefaultCalendars: func(bool) (map[string]any, error) {
			return map[string]any{"ok": true}, nil
		},
		NetworkLikelyAvailable: func() bool { return true },
	})
}

func TestRefreshUsesCurrentCatalogAndLocalFallback(t *testing.T) {
	s := testService(t)
	s.SaveSourcesStatus(map[string]any{"enabled": []any{"quotes", "jokes"}, "updatedAt": int64(1)})
	got := s.Refresh(context.Background(), false, false)
	if got["generator"] != "go" {
		t.Fatalf("expected go generator, got %#v", got["generator"])
	}
	if len(jsonutil.List(got["items"])) == 0 {
		t.Fatalf("expected local fallback messages")
	}
	if len(jsonutil.List(got["sourceStatus"])) != 2 {
		t.Fatalf("expected two source statuses, got %#v", got["sourceStatus"])
	}
}

func TestNormalizeEnabledRejectsRetiredAndNonStringValues(t *testing.T) {
	s := testService(t)
	got := s.NormalizeEnabled([]any{"quotes-calm", " jokes ", 42, nil, []any{"facts"}})
	if len(got) != 1 || got[0] != "jokes" {
		t.Fatalf("unexpected normalized values: %#v", got)
	}
}

func TestOverridesRemainAtomicAndApplyEdits(t *testing.T) {
	s := testService(t)
	s.SaveCache(map[string]any{"items": []any{
		map[string]any{"id": "keep", "text": "Old", "source": "quotes", "weight": 1},
		map[string]any{"id": "gone", "text": "Gone", "source": "quotes", "weight": 1},
	}})
	s.SaveOverrides(map[string]any{"removed": []any{"gone"}, "edits": map[string]any{"keep": map[string]any{"text": "New", "weight": 7}}})
	items := jsonutil.List(s.CachePayload()["items"])
	if len(items) != 1 {
		t.Fatalf("expected one item, got %#v", items)
	}
	item := jsonutil.Map(items[0])
	if item["text"] != "New" || jsonutil.Int(item["weight"], 0) != 7 || item["edited"] != true {
		t.Fatalf("override was not applied: %#v", item)
	}
	if err := s.DeleteItem("keep"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.CachePath()); err != nil {
		t.Fatalf("cache write missing: %v", err)
	}
	if _, err := os.Stat(s.OverridesPath()); err != nil {
		t.Fatalf("override write missing: %v", err)
	}
}

func TestCelebrationWriteUsesExistingCalendarCallback(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "config")
	if err := os.MkdirAll(config, 0755); err != nil {
		t.Fatal(err)
	}
	called := 0
	s := New(ServiceConfig{
		Home:             root,
		ConfigDir:        config,
		ConfigLocal:      filepath.Join(config, "config.local.js"),
		CelebrationsFile: filepath.Join(root, ".dashboard-celebrations"),
		GenerateDefaultCalendars: func(force bool) (map[string]any, error) {
			if !force {
				t.Fatal("expected refresh request")
			}
			called++
			return map[string]any{}, nil
		},
	})
	got := s.saveCelebrations([]any{map[string]any{"date": "12-25", "label": "Holiday"}}, true)
	if called != 1 || len(got) != 1 {
		t.Fatalf("unexpected callback/items: called=%d got=%#v", called, got)
	}
}

func TestCanonicalComplimentsPreservesCurrentV4Shape(t *testing.T) {
	raw := map[string]any{"items": []any{map[string]any{"text": "Hello", "weight": 2}, map[string]any{"text": "Hello", "weight": 4}}, "defaultsCleared": true}
	got, changed := CanonicalCompliments(raw)
	if !changed || len(jsonutil.List(got["messages"])) != 1 || jsonutil.Int(got["version"], 0) != 4 {
		t.Fatalf("unexpected canonical result: changed=%t payload=%#v", changed, got)
	}
}

func TestMessageSourcePreferencesWriteTheExistingPath(t *testing.T) {
	s := testService(t)
	s.SaveSourcesStatus(map[string]any{"enabled": []any{"quotes"}, "updatedAt": time.Now().UnixMilli()})
	path := filepath.Join(filepath.Dir(s.CachePath()), "message-sources.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("message source preferences missing: %v", err)
	}
	if err := fileio.WriteJSON(path, map[string]any{"enabled": []any{"jokes"}}); err != nil {
		t.Fatal(err)
	}
	prefs := s.Preferences()
	if len(jsonutil.List(prefs["enabled"])) != 1 || jsonutil.List(prefs["enabled"])[0] != "jokes" {
		t.Fatalf("unexpected preferences: %#v", prefs)
	}
}
