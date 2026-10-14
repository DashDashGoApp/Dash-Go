package messages

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// TestCleanTextLimitCutsRunesNeverBytes pins the 1.5.21 fix: the limit counts
// runes, so a multi-byte rune (em dash) straddling the boundary is kept whole
// instead of leaving half a rune behind as invalid UTF-8.
func TestCleanTextLimitCutsRunesNeverBytes(t *testing.T) {
	text := strings.Repeat("a", 258) + " \u2014" // 260 runes, 262 bytes; the em dash straddles the 260-byte cut
	got := cleanTextLimit(text, 260)
	if !utf8.ValidString(got) {
		t.Fatalf("cleanTextLimit produced invalid UTF-8: %q", got)
	}
	if n := len([]rune(got)); n != 260 {
		t.Fatalf("expected 260 runes, got %d", n)
	}
	if !strings.HasSuffix(got, "\u2014") {
		t.Fatalf("expected the em dash to survive the cut, got tail %q", got[len(got)-8:])
	}
	if emoji := cleanTextLimit(strings.Repeat("b", 259)+"\U0001F600"+strings.Repeat("c", 4), 260); !utf8.ValidString(emoji) || len([]rune(emoji)) != 260 {
		t.Fatalf("emoji cut not rune-safe: %q", emoji)
	}
	if short := cleanTextLimit("short \u2014 text", 260); short != "short \u2014 text" {
		t.Fatalf("short text must pass through unchanged: %q", short)
	}
}

// TestRotatingFeedItemsDropsOversizedLines pins the server-side length gate:
// a provider line over rotatingFeedMaxRunes is excluded, never truncated, and
// the 200-rune boundary is inclusive.
func TestRotatingFeedItemsDropsOversizedLines(t *testing.T) {
	c := messageCategory{ID: "quotes", Label: "Quotes", Local: "quotes"}
	got := rotatingFeedItems([]string{strings.Repeat("x", 250), "A short, safe line."}, c)
	if len(got) != 1 || got[0]["text"] != "A short, safe line." {
		t.Fatalf("expected the 250-rune line dropped and the short one kept, got %#v", got)
	}
	boundary := rotatingFeedItems([]string{strings.Repeat("y", 200), strings.Repeat("z", 201)}, c)
	if len(boundary) != 1 || boundary[0]["text"] != strings.Repeat("y", 200) {
		t.Fatalf("expected exactly the 200-rune line to pass the gate, got %d items", len(boundary))
	}
}

var reFourDigitYear = regexp.MustCompile(`\d{4}`)

// TestLocalMessagePools is the 1.5.21 dictionary gate: every pool present and
// sized for rotation, no duplicates, ticker-length bounded, trivia/riddles
// answer-shaped, history year-free (a local fallback must never invent a
// dated anniversary), attributed quotes well-formed, and joke/quote entries
// clean under the content filter.
func TestLocalMessagePools(t *testing.T) {
	pools := loadLocalMessagePools()
	for _, c := range messageCategories {
		if _, ok := pools[c.Local]; !ok {
			t.Errorf("missing local pool %q for category %s", c.Local, c.ID)
		}
	}
	seen := map[string]string{}
	for key, entries := range pools {
		if key != "nsfw" && len(entries) < 20 {
			t.Errorf("pool %s has %d entries, want at least 20", key, len(entries))
		}
		for _, entry := range entries {
			if entry == "" {
				t.Errorf("pool %s has an empty entry", key)
				continue
			}
			if n := len([]rune(entry)); n > 160 {
				t.Errorf("pool %s entry is %d runes (>160): %.60q", key, n, entry)
			}
			nk := normMessage(entry)
			if prev, dup := seen[nk]; dup {
				t.Errorf("duplicate entry %q also appears in pool %s", entry, prev)
			}
			seen[nk] = key
			switch key {
			case "trivia", "riddles":
				if !strings.Contains(entry, "Answer:") && !strings.Contains(entry, "?") {
					t.Errorf("pool %s entry has neither an answer nor a question: %.60q", key, entry)
				}
			case "history":
				if reFourDigitYear.MatchString(entry) {
					t.Errorf("history entry carries a 4-digit year: %.60q", entry)
				}
			case "quotes":
				if idx := strings.LastIndex(entry, " \u2014 "); idx >= 0 {
					author := entry[idx+len(" \u2014 "):]
					if r, _ := utf8.DecodeRuneInString(author); !unicode.IsUpper(r) {
						t.Errorf("attributed quote has a malformed author tail: %.60q", entry)
					}
				}
			}
			if filteredMessageSources[key] {
				if _, blocked := blockedByKeywords(key, entry, builtinContentKeywords); blocked {
					t.Errorf("pool %s entry trips the content filter: %.60q", key, entry)
				}
			}
		}
	}
}

// TestLocalMessagePoolsNewEntriesCarrySources verifies the review rule: every
// fact and trivia entry, and every attributed quote, carries a verification
// URL. Existing aphorisms without authors have no citable source and are
// exempt by design.
func TestLocalMessagePoolsNewEntriesCarrySources(t *testing.T) {
	var pools map[string][]localEntry
	if err := json.Unmarshal([]byte(localMessagesRaw()), &pools); err != nil {
		t.Fatalf("embedded dictionary does not parse: %v", err)
	}
	for _, key := range []string{"facts", "trivia"} {
		for _, entry := range pools[key] {
			if strings.TrimSpace(entry.Src) == "" {
				t.Errorf("pool %s entry lacks a verification URL: %.60q", key, entry.Text)
			}
		}
	}
	for _, entry := range pools["quotes"] {
		if strings.Contains(entry.Text, " \u2014 ") && strings.TrimSpace(entry.Src) == "" {
			t.Errorf("attributed quote lacks a verification URL: %.60q", entry.Text)
		}
	}
}

// TestSanitizeFeedText pins the feed hygiene rules against the live shapes
// the 1.5.21 audit found in the wild: The Quotes Hub's tilde-attribution
// tail, leaked HTML entities, Unknown/Anonymous authors, doubled
// punctuation, and plain lines that must pass through untouched.
func TestSanitizeFeedText(t *testing.T) {
	if got := sanitizeFeedText("\u201cThe world is full of obvious things which nobody has thought to see.\u201d~Sherlock Holmes."); got != "\u201cThe world is full of obvious things which nobody has thought to see.\u201d \u2014 Sherlock Holmes" {
		t.Fatalf("tilde author tail not normalized: %q", got)
	}
	if got := sanitizeFeedText("Fish & chips &amp; a rainy walk"); got != "Fish & chips & a rainy walk" {
		t.Fatalf("entity not decoded: %q", got)
	}
	if got := sanitizeFeedText("A quiet thought \u2014 Unknown"); got != "A quiet thought" {
		t.Fatalf("Unknown author not dropped: %q", got)
	}
	if got := sanitizeFeedText("Words, words, words..."); got != "Words, words, words." {
		t.Fatalf("doubled punctuation not collapsed: %q", got)
	}
	if got := sanitizeFeedText("No change needed here."); got != "No change needed here." {
		t.Fatalf("plain line modified: %q", got)
	}
}

// TestFreshInstallDefaultsEnableTriviaAndHistory pins the 1.5.21 default
// prefs: a fresh install ships trivia and history enabled so the new
// knowledge content is visible from the start, while a saved prefs file
// always wins (existing installs are untouched).
func TestFreshInstallDefaultsEnableTriviaAndHistory(t *testing.T) {
	s := testService(t)
	enabled := map[string]bool{}
	for _, v := range jsonutil.List(s.messagePrefs()["enabled"]) {
		enabled[fmt.Sprint(v)] = true
	}
	if !enabled["trivia"] || !enabled["history"] {
		t.Fatalf("fresh install should default trivia and history to enabled, got %#v", enabled)
	}
	s.SaveSourcesStatus(map[string]any{"enabled": []any{"quotes"}, "updatedAt": int64(1)})
	saved := map[string]bool{}
	for _, v := range jsonutil.List(s.messagePrefs()["enabled"]) {
		saved[fmt.Sprint(v)] = true
	}
	if len(saved) != 1 || !saved["quotes"] {
		t.Fatalf("saved prefs must win over the shipped default, got %#v", saved)
	}
}
