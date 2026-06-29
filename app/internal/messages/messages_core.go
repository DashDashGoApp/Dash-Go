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
	{"jokes", "Jokes", "Clean jokes, dad jokes, and puns.", []string{"icanhazdadjoke", "jokeapi_safe", "api_ninjas_dadjokes", "api_ninjas_jokes", "official_joke"}, "jokes", false},
	{"quotes", "Quotes", "Short uplifting and reflective lines.", []string{"quotable", "favqs", "zenquotes", "api_ninjas_quotes", "typefit_quotes", "dummyjson_quotes"}, "quotes", false},
	{"facts", "Fun facts", "General trivia, science, space, history, and animal facts.", []string{"uselessfacts", "api_ninjas_facts", "catfact", "meowfacts", "numbersapi_https"}, "facts", false},
	{"riddles", "Riddles", "Quick riddle prompts.", []string{"api_ninjas_riddles", "riddles_api"}, "riddles", false},
	{"words", "Word of the day", "Useful vocabulary and definitions.", []string{"dictionary_random_word", "freedictionary_random_word"}, "words", false},
	{"wellbeing", "Wellbeing prompts", "Gratitude, kindness, and mindfulness nudges.", []string{"affirmations", "advice_slip", "api_ninjas_quotes_positive"}, "wellbeing", false},
	{"family", "Family & home prompts", "Conversation starters, household nudges, seasonal notes, and coffee thoughts.", []string{"advice_slip"}, "family", false},
	{"nsfw-jokes", "NSFW adult jokes", "Adult joke feed. Off by default.", []string{"jokeapi_nsfw"}, "nsfw", true},
}

var providerLabels = map[string]string{
	"icanhazdadjoke": "icanhazdadjoke", "jokeapi_safe": "JokeAPI / Sv443 safe mode", "api_ninjas_dadjokes": "API Ninjas Dad Jokes", "api_ninjas_jokes": "API Ninjas Jokes", "official_joke": "Official Joke API",
	"quotable": "Quotable", "favqs": "FavQs QOTD", "zenquotes": "ZenQuotes", "api_ninjas_quotes": "API Ninjas Quotes", "typefit_quotes": "Type.fit Quotes", "dummyjson_quotes": "DummyJSON Quotes",
	"uselessfacts": "Useless Facts", "api_ninjas_facts": "API Ninjas Facts", "catfact": "catfact.ninja", "meowfacts": "MeowFacts", "numbersapi_https": "Numbers API HTTPS",
	"api_ninjas_riddles": "API Ninjas Riddles", "riddles_api": "Riddles API", "dictionary_random_word": "Random Word + Dictionary API", "freedictionary_random_word": "Random Word + FreeDictionaryAPI",
	"affirmations": "Affirmations.dev", "advice_slip": "Advice Slip", "api_ninjas_quotes_positive": "API Ninjas positive quotes", "jokeapi_nsfw": "JokeAPI adult", "local": "local fallback",
}

var providerKeyEnv = map[string]string{
	"api_ninjas_dadjokes": "DASH_API_NINJAS_KEY", "api_ninjas_jokes": "DASH_API_NINJAS_KEY", "api_ninjas_quotes": "DASH_API_NINJAS_KEY", "api_ninjas_facts": "DASH_API_NINJAS_KEY", "api_ninjas_riddles": "DASH_API_NINJAS_KEY", "api_ninjas_quotes_positive": "DASH_API_NINJAS_KEY",
}

var localMessages = map[string][]string{
	"quotes":    {"A steady pace still gets there.", "Small progress is still progress.", "Start where you are; use what you have.", "Today gets easier when you take the next step.", "Breathe in. Reset. Continue.", "Quiet moments count too.", "Peace can be a plan.", "Soft starts are still starts.", "Home is built from tiny acts of care.", "The best days leave room for each other.", "Little traditions become big memories.", "This house runs on love and snacks."},
	"jokes":     {"Why did the calendar feel popular? It had a lot of dates.", "I only know 25 letters of the alphabet. I don't know y.", "Why did the scarecrow win? He was outstanding in his field.", "Parallel lines have so much in common. It is a shame they'll never meet.", "Why did the bicycle fall over? It was two-tired.", "What do clouds wear under their shorts? Thunderwear.", "Why can't your nose be twelve inches long? Then it would be a foot.", "What did one wall say to the other? I'll meet you at the corner."},
	"facts":     {"Honey never spoils when stored properly.", "Bananas are berries, botanically speaking.", "Octopuses have three hearts.", "A day on Venus is longer than a Venus year.", "Sound travels faster in water than in air.", "Water expands when it freezes.", "Lightning can heat nearby air hotter than the Sun's surface.", "The Moon is slowly drifting away from Earth."},
	"riddles":   {"What has hands but cannot clap? A clock.", "What gets wetter as it dries? A towel.", "What has many keys but opens no locks? A piano.", "What has a head, a tail, and no body? A coin.", "What can travel around the world while staying in a corner? A stamp.", "What has words but never speaks? A book."},
	"words":     {"Lucid — clear and easy to understand.", "Kindred — similar in character or related.", "Resilient — able to recover after difficulty.", "Nimble — quick and light in movement or thought.", "Curious — eager to learn or know more.", "Sturdy — strong and solid."},
	"wellbeing": {"Name one small thing that helped today.", "Thank someone for an ordinary kindness.", "Notice something that made the room better.", "What went right today?", "Send one encouraging message.", "Take three slow breaths."},
	"family":    {"What was the best part of your day?", "What are you looking forward to?", "What should we cook soon?", "What made you laugh lately?", "Water bottles to the sink.", "Check tomorrow's calendar before bedtime."},
	"nsfw":      {},
}

func (s *Service) messagePrefs() map[string]any {
	raw := jsonutil.Map(s.readJSONDefault(filepath.Join(s.configDir, "message-sources.json"), map[string]any{"enabled": []any{}, "updatedAt": 0}))
	raw["enabled"] = s.normalizeMessageEnabled(jsonutil.List(raw["enabled"]))
	return raw
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

func applyMessageOverrides(items []any, ov map[string]any) []any {
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
		it["id"] = id
		it["text"] = text
		it["source"] = source
		it["weight"] = clamp(jsonutil.Int(it["weight"], 1), 1, 10000)
		out = append(out, it)
	}
	return out
}
