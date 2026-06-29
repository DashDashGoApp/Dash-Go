package main

import (
	"strings"
	"sync"
	"testing"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func TestMessageCacheMutationsSerializeConcurrentDeletes(t *testing.T) {
	a := testApp(t)
	a.saveMessageCache(map[string]any{"items": []any{
		map[string]any{"id": "one", "text": "One", "source": "quotes", "weight": 1},
		map[string]any{"id": "two", "text": "Two", "source": "quotes", "weight": 1},
	}})
	a.saveMessageOverrides(map[string]any{"removed": []any{}, "edits": map[string]any{}})
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, id := range []string{"one", "two"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- a.deleteMessageItem(id)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	cache := jsonutil.Map(a.readJSONDefault(a.messageCachePath(), map[string]any{}))
	if got := len(jsonutil.List(cache["items"])); got != 0 {
		t.Fatalf("concurrent deletes lost an update: %#v", cache)
	}
	overrides := jsonutil.Map(a.readJSONDefault(a.messageOverridesPath(), map[string]any{}))
	if got := len(jsonutil.List(overrides["removed"])); got != 2 {
		t.Fatalf("removed overrides lost an update: %#v", overrides)
	}
}

func TestDecodeMessageJSONHonorsBodyLimit(t *testing.T) {
	var out []string
	payload := `[` + `"` + strings.Repeat("x", 2048) + `"]`
	if err := decodeMessageJSON(strings.NewReader(payload), 1024, &out); err == nil {
		t.Fatal("expected a truncated body to fail JSON decoding")
	}
}
