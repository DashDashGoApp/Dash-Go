package messages

import (
	"html"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// sanitizeFeedText applies the feed text hygiene rules in one place, for
// every provider line, local entry, and owner edit:
//   - decode the HTML entities feeds leak into plain text (amp, quot, nbsp...);
//   - normalize stray "”~Author" / "~Author" tails to the standard " — Author"
//     shape (The Quotes Hub ships "…”~Sherlock Holmes." style lines);
//   - drop " — Unknown" / " — Anonymous" author suffixes (keep the text);
//   - collapse doubled punctuation (".." -> ".").
//
// It is deliberately conservative: a line that does not match one of the
// shapes passes through byte-for-byte, and no content-filter scope is widened
// (1.5.20 limits the filter to the joke and quote feeds on purpose).
var (
	reTildeAuthor   = regexp.MustCompile(`(?s)^(.{8,}?)\s*~\s*([A-Z][\p{L}\p{M}'\- ]{1,38})\s*\.?\s*$`)
	reUnknownAuthor = regexp.MustCompile(`(?is)\s*—\s*(?:unknown|anonymous|unnamed)(?:\s+author)?\s*\.?\s*$`)
	// RE2 has no backreferences, so the run is matched as "first punctuation
	// char + 1-3 more" and collapsed with a $1 substitution. This covers
	// "..", "...", "!!", "??", and mixed runs like ".?" -> "." while leaving
	// ordinary punctuation (digits after a dot, single sentence ends) alone.
	reDoubledPunct = regexp.MustCompile(`([,.!?])[,.!?]{1,3}`)
)

func sanitizeFeedText(s string) string {
	if s == "" {
		return s
	}
	s = html.UnescapeString(s)
	if m := reTildeAuthor.FindStringSubmatch(s); m != nil {
		s = m[1] + " — " + strings.TrimSpace(m[2])
	} else if m := reUnknownAuthor.FindStringSubmatchIndex(s); m != nil {
		// Byte indices; the match starts at the whitespace before the dash,
		// so cutting at the match start drops the whole author tail.
		s = s[:m[0]]
	}
	s = reDoubledPunct.ReplaceAllString(s, "$1")
	return strings.TrimSpace(s)
}

// builtinContentKeywords are the household-safety defaults: jokes or quotes
// about sexual violence, or quoting a person publicly identified with it,
// never reach the screen. The set is deliberately tight — each term was chosen
// to carry little or no chance of colliding with an innocent joke or quote —
// and the owner can extend it through message-keywords.json. Broader netting
// (e.g. "predator", "groomer") is left to the owner, who sees the content and
// can add it when they want the extra catch.
var builtinContentKeywords = []string{
	"rape",
	"rapist",
	"pedophile",
	"pedo",
	"molester",
	"child abuse",
	"child molestation",
	"child molester",
	"child grooming",
	"underage",
	"bill cosby",
	"jared fogel",
	"harvey weinstein",
}

// filteredMessageSources are the categories the content filter guards. It is
// scoped to the joke and quote feeds the owner flagged — not facts, history,
// trivia, riddles, or wellbeing — so a legitimate trivia answer or a
// philosophical quote is never eaten by the filter.
var filteredMessageSources = map[string]bool{
	"jokes":      true,
	"quotes":     true,
	"nsfw-jokes": true,
}

// contentKeywordFile lets the owner add or remove patterns without a code
// change: a JSON array of strings at <config>/message-keywords.json.
func contentKeywordFile(s *Service) string {
	return filepath.Join(s.configDir, "message-keywords.json")
}

// contentKeywordPatterns is the active blocklist: the built-in defaults plus
// any owner-added patterns, deduplicated, each already folded to lowercase
// with collapsed whitespace.
func (s *Service) contentKeywordPatterns() []string {
	patterns := append([]string{}, builtinContentKeywords...)
	for _, raw := range jsonutil.List(s.readJSONDefault(contentKeywordFile(s), []any{})) {
		p := normContentFold(cleanMessageText(raw))
		if p != "" && !slices.Contains(patterns, p) {
			patterns = append(patterns, p)
		}
	}
	return patterns
}

// normContentFold lowercases and collapses whitespace so "Bill  COSBY" and
// "bill cosby" both match. Matching stays case-insensitive and whitespace-
// tolerant; it is deliberately not diacritic-aware, which keeps the check
// predictable and cheap on a single-core Pi.
func normContentFold(t string) string {
	return strings.TrimSpace(whitespaceRun.ReplaceAllString(strings.ToLower(t), " "))
}

// blockedByKeywords reports whether a finished message line trips the
// content filter, returning the first matching pattern for diagnostics. It
// only guards the joke and quote feeds (see filteredMessageSources); any other
// source passes through untouched. A single-word pattern must match as a whole
// word ("rape" must not fire inside "rap" or "grape"); a multi-word phrase
// matches as written, because it is already specific enough.
func blockedByKeywords(source, text string, patterns []string) (string, bool) {
	if !filteredMessageSources[source] {
		return "", false
	}
	folded := normContentFold(text)
	for _, p := range patterns {
		if !strings.ContainsAny(p, " ") {
			re, err := regexp.Compile("(^|[^a-z0-9])" + regexp.QuoteMeta(p) + "([^a-z0-9]|$)")
			if err != nil || !re.MatchString(folded) {
				continue
			}
			return p, true
		}
		if strings.Contains(folded, p) {
			return p, true
		}
	}
	return "", false
}
