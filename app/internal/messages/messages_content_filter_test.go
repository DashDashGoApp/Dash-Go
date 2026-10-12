package messages

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func TestContentKeywordFilterBlocksBuiltinsAndPhrases(t *testing.T) {
	s := testService(t)
	items := []any{
		map[string]any{"id": "a", "text": "Why did the grape get mugged by a rapist? No — just a grape. Actually it let out a little wine.", "source": "jokes", "weight": 1},
		map[string]any{"id": "b", "text": "Bill Cosby once said: I will always be your friend.", "source": "quotes", "weight": 1},
		map[string]any{"id": "c", "text": "Jared Fogel on the stage, live from the show.", "source": "jokes", "weight": 1},
		map[string]any{"id": "d", "text": "A single grape is fine, and so is a rap. Nothing to see here.", "source": "quotes", "weight": 1},
	}
	out := applyMessageOverrides(items, map[string]any{"removed": []any{}, "edits": map[string]any{}}, s.contentKeywordPatterns())
	if len(out) != 1 {
		t.Fatalf("expected one surviving item, got %d: %#v", len(out), out)
	}
	if got := jsonutil.StringValue(jsonutil.Map(out[0])["id"]); got != "d" {
		t.Fatalf("expected item d to survive, got %q", got)
	}
}

func TestContentKeywordFilterAppliesToEditedText(t *testing.T) {
	s := testService(t)
	items := []any{
		map[string]any{"id": "a", "text": "Harmless line.", "source": "quotes", "weight": 1},
	}
	ov := map[string]any{"removed": []any{}, "edits": map[string]any{"a": map[string]any{"text": "Harvey Weinstein was a good man.", "weight": 1}}}
	out := applyMessageOverrides(items, ov, s.contentKeywordPatterns())
	if len(out) != 0 {
		t.Fatalf("expected the edited-in blocked phrase to be dropped, got %#v", out)
	}
}

func TestContentKeywordFileExtendsTheBuiltinList(t *testing.T) {
	s := testService(t)
	path := filepath.Join(s.configDir, "message-keywords.json")
	if err := fileio.WriteJSON(path, []any{"unhinged", 42, "  "}); err != nil {
		t.Fatal(err)
	}
	patterns := s.contentKeywordPatterns()
	if !slices.Contains(patterns, "unhinged") || !slices.Contains(patterns, "rape") {
		t.Fatalf("expected owner pattern plus builtins, got %#v", patterns)
	}
	out := applyMessageOverrides(
		[]any{map[string]any{"id": "a", "text": "He is an unhinged example.", "source": "quotes", "weight": 1}},
		map[string]any{"removed": []any{}, "edits": map[string]any{}},
		patterns,
	)
	if len(out) != 0 {
		t.Fatalf("expected the owner keyword to block the item, got %#v", out)
	}
}

func TestContentKeywordFoldToleratesCaseAndSpacing(t *testing.T) {
	if _, blocked := blockedByKeywords("jokes", "Bill  COSBY — a quote", []string{"bill cosby"}); !blocked {
		t.Fatal("expected the folded phrase to match despite casing and double space")
	}
	if _, blocked := blockedByKeywords("jokes", "A big plus", []string{"bill cosby"}); blocked {
		t.Fatal("phrase must not match unrelated text")
	}
	if _, blocked := blockedByKeywords("jokes", "grape", []string{"rape"}); blocked {
		t.Fatal("whole-word match must not fire inside another word")
	}
	if _, blocked := blockedByKeywords("jokes", "rape", []string{"rape"}); !blocked {
		t.Fatal("whole-word match at line start must fire")
	}
}

func TestContentKeywordFilterIsScopedToJokeAndQuoteFeeds(t *testing.T) {
	patterns := builtinContentKeywords
	// The same blocked text, but from a feed the filter does not guard,
	// must pass through untouched.
	if _, blocked := blockedByKeywords("facts", "rape", patterns); blocked {
		t.Fatal("facts feed must not be filtered by the joke/quote content filter")
	}
	if _, blocked := blockedByKeywords("history", "harvey weinstein", patterns); blocked {
		t.Fatal("history feed must not be filtered by the joke/quote content filter")
	}
	// The joke and quote feeds it does guard must still fire on the same text.
	if _, blocked := blockedByKeywords("jokes", "rape", patterns); !blocked {
		t.Fatal("jokes feed must be filtered")
	}
	if _, blocked := blockedByKeywords("nsfw-jokes", "bill cosby", patterns); !blocked {
		t.Fatal("nsfw-jokes feed must be filtered")
	}
}
