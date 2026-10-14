package messages

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

type messageCategory struct {
	ID          string
	Label       string
	Description string
	Providers   []string
	Local       string
	NSFW        bool
}

type messageFetchResult struct {
	Items  []map[string]any
	Status map[string]any
}

var messageCategories = []messageCategory{
	{"jokes", "Jokes", "Clean jokes, dad jokes, and puns.", []string{"icanhazdadjoke", "jokeapi_safe", "official_joke", "api_ninjas_dadjokes", "api_ninjas_jokes"}, "jokes", false},
	{"quotes", "Quotes", "Short uplifting and reflective lines.", []string{"dummyjson_quotes", "quotable_mirror", "stoic_quotes", "thequoteshub", "favqs", "zenquotes", "typefit_quotes", "quotable", "api_ninjas_quotes"}, "quotes", false},
	{"facts", "Fun facts", "General trivia, science, space, history, and animal facts.", []string{"uselessfacts", "wikimedia_featured", "api_ninjas_facts", "catfact", "meowfacts"}, "facts", false},
	{"history", "This day in history", "What happened on today's date, and who was born today.", []string{"wikimedia_onthisday", "wikimedia_births"}, "history", false},
	{"trivia", "Trivia", "Household-safe quiz questions with the answer.", []string{"the_trivia_api", "opentdb"}, "trivia", false},
	{"riddles", "Riddles", "Quick riddle prompts.", []string{"api_ninjas_riddles", "riddles_api"}, "riddles", false},
	{"wellbeing", "Wellbeing prompts", "Gratitude, kindness, and mindfulness nudges.", []string{"affirmations", "advice_slip", "api_ninjas_quotes_positive", "positive_api"}, "wellbeing", false},
	{"family", "Family & home prompts", "Conversation starters, household nudges, seasonal notes, and coffee thoughts.", []string{"advice_slip", "bored_api"}, "family", false},
	{"nsfw-jokes", "NSFW adult jokes", "Adult joke feed. Off by default.", []string{"jokeapi_nsfw"}, "nsfw", true},
}

var providerLabels = map[string]string{
	"icanhazdadjoke": "icanhazdadjoke", "jokeapi_safe": "JokeAPI / Sv443 safe mode", "api_ninjas_dadjokes": "API Ninjas Dad Jokes", "api_ninjas_jokes": "API Ninjas Jokes", "official_joke": "Official Joke API",
	"quotable": "Quotable", "quotable_mirror": "Quotable community mirror", "favqs": "FavQs QOTD", "zenquotes": "ZenQuotes", "api_ninjas_quotes": "API Ninjas Quotes", "typefit_quotes": "Type.fit Quotes", "dummyjson_quotes": "DummyJSON Quotes", "stoic_quotes": "Stoic Quotes", "thequoteshub": "The Quotes Hub",
	"uselessfacts": "Useless Facts", "wikimedia_featured": "Wikipedia: featured article", "api_ninjas_facts": "API Ninjas Facts", "catfact": "catfact.ninja", "meowfacts": "MeowFacts",
	"wikimedia_onthisday": "Wikipedia: On this day", "wikimedia_births": "Wikipedia: born this day", "opentdb": "Open Trivia DB", "the_trivia_api": "The Trivia API",
	"api_ninjas_riddles": "API Ninjas Riddles", "riddles_api": "Riddles API",
	"affirmations": "Affirmations.dev", "advice_slip": "Advice Slip", "api_ninjas_quotes_positive": "API Ninjas positive quotes", "positive_api": "Positive API", "bored_api": "Bored API (kid-friendly)", "jokeapi_nsfw": "JokeAPI adult", "local": "local fallback",
}

var providerKeyEnv = map[string]string{
	"api_ninjas_dadjokes": "DASH_API_NINJAS_KEY", "api_ninjas_jokes": "DASH_API_NINJAS_KEY", "api_ninjas_quotes": "DASH_API_NINJAS_KEY", "api_ninjas_facts": "DASH_API_NINJAS_KEY", "api_ninjas_riddles": "DASH_API_NINJAS_KEY", "api_ninjas_quotes_positive": "DASH_API_NINJAS_KEY",
}

func (s *Service) messagePrefs() map[string]any {
	raw := jsonutil.Map(s.readJSONDefault(filepath.Join(s.configDir, "message-sources.json"), map[string]any{"enabled": defaultMessageEnabled(), "updatedAt": 0}))
	raw["enabled"] = s.normalizeMessageEnabled(jsonutil.List(raw["enabled"]))
	return raw
}

// defaultMessageEnabled is the shipped default for a fresh install: the two
// household-safe knowledge categories the 1.5.21 content additions feed. It
// only applies when no message-sources.json exists yet; existing installs
// keep whatever the owner has already chosen.
func defaultMessageEnabled() []any {
	return []any{"history", "trivia"}
}

func (s *Service) normalizeMessageEnabled(values []any) []any {
	valid := map[string]bool{}
	for _, c := range messageCategories {
		valid[c.ID] = true
	}
	seen := map[string]bool{}
	out := []any{}
	for _, raw := range values {
		id := jsonutil.StringValue(raw)
		if valid[id] && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	slices.SortFunc(out, func(left, right any) int { return compareText(left, right) })
	return out
}

func (s *Service) messageDefs() []any {
	defs := []any{}
	for _, c := range messageCategories {
		keyed := []any{}
		keyEnv := map[string]bool{}
		labels := []any{}
		providers := []any{}
		for _, p := range c.Providers {
			providers = append(providers, p)
			labels = append(labels, providerLabels[p])
			if env, ok := providerKeyEnv[p]; ok {
				keyed = append(keyed, p)
				keyEnv[env] = true
			}
		}
		envs := []any{}
		for env := range keyEnv {
			envs = append(envs, env)
		}
		slices.SortFunc(envs, func(left, right any) int { return compareText(left, right) })
		defs = append(defs, map[string]any{"id": c.ID, "label": c.Label, "description": c.Description, "kind": "api", "category": true, "providers": providers, "providerLabels": labels, "keyedProviders": keyed, "keyEnv": envs, "nsfw": c.NSFW})
	}
	return defs
}

func (s *Service) messageProviderReady(provider string) bool {
	if env, ok := providerKeyEnv[provider]; ok {
		return s.messageEnv(env) != ""
	}
	return true
}

func (s *Service) messageEnv(key string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return readEnv(filepath.Join(s.home, ".dashboard-message.env"))[key]
}

func cleanMessageText(v any) string {
	s := strings.TrimSpace(controlChars.ReplaceAllString(jsonutil.StringValue(v), " "))
	s = sanitizeFeedText(s)
	if len([]rune(s)) <= 300 {
		return s
	}
	r := []rune(s)
	cut := string(r[:300])
	if idx := strings.LastIndex(cut, ". "); idx > 80 {
		return strings.TrimSpace(cut[:idx+1])
	}
	return strings.TrimSpace(string(r[:297])) + "..."
}
func normMessage(s string) string {
	return strings.ToLower(strings.TrimSpace(whitespaceRun.ReplaceAllString(cleanMessageText(s), " ")))
}
func stableMessageID(source, text string) string {
	sum := sha256.Sum256([]byte(source + "|" + normMessage(text)))
	return fmt.Sprintf("%x", sum)[:12]
}

func messageItem(text, source string, nsfw bool, weight int) map[string]any {
	t := cleanMessageText(text)
	if nsfw {
		t = strings.TrimSpace(reNSFWPrefix.ReplaceAllString(t, ""))
	}
	if t == "" {
		return nil
	}
	return map[string]any{"id": stableMessageID(source, t), "text": t, "source": source, "nsfw": nsfw, "weight": clamp(weight, 1, 10000), "edited": false}
}

func (s *Service) messageOverrides() map[string]any {
	ov := jsonutil.Map(s.readJSONDefault(filepath.Join(s.configDir, "message-cache-overrides.json"), map[string]any{"removed": []any{}, "edits": map[string]any{}}))
	if _, ok := ov["removed"].([]any); !ok {
		ov["removed"] = []any{}
	}
	if _, ok := ov["edits"].(map[string]any); !ok {
		ov["edits"] = map[string]any{}
	}
	return ov
}

func applyMessageOverrides(items []any, ov map[string]any, patterns []string) []any {
	removed := map[string]bool{}
	for _, id := range jsonutil.List(ov["removed"]) {
		removed[jsonutil.StringValue(id)] = true
	}
	edits := jsonutil.Map(ov["edits"])
	out := []any{}
	for _, raw := range items {
		it := jsonutil.Map(raw)
		text := cleanMessageText(it["text"])
		source := jsonutil.StringValue(it["source"])
		if source == "" {
			source = "feed"
		}
		id := jsonutil.StringValue(it["id"])
		if id == "" {
			id = stableMessageID(source, text)
		}
		if removed[id] || text == "" {
			continue
		}
		if editRaw, ok := edits[id]; ok {
			edit := jsonutil.Map(editRaw)
			if t := cleanMessageText(edit["text"]); t != "" {
				text = t
				it["text"] = t
				it["edited"] = true
			}
			if edit["weight"] != nil {
				it["weight"] = clamp(jsonutil.Int(edit["weight"], 1), 1, 10000)
				it["edited"] = true
			}
		}
		// The filter runs on the final displayed text, including owner edits:
		// re-adding a blocked phrase through the editor must not surface it.
		if _, blocked := blockedByKeywords(source, text, patterns); blocked {
			continue
		}
		it["id"] = id
		it["text"] = text
		it["source"] = source
		it["weight"] = clamp(jsonutil.Int(it["weight"], 1), 1, 10000)
		out = append(out, it)
	}
	return out
}
