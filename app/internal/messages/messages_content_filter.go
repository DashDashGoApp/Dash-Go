package messages

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

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
