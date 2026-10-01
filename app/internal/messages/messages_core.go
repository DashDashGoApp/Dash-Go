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
	{"facts", "Fun facts", "General trivia, science, space, history, and animal facts.", []string{"uselessfacts", "api_ninjas_facts", "catfact", "meowfacts"}, "facts", false},
	{"history", "This day in history", "What happened on today's date, and who was born today.", []string{"wikimedia_onthisday", "wikimedia_births"}, "history", false},
	{"trivia", "Trivia", "Household-safe quiz questions with the answer.", []string{"opentdb"}, "trivia", false},
	{"riddles", "Riddles", "Quick riddle prompts.", []string{"api_ninjas_riddles", "riddles_api"}, "riddles", false},
	{"wellbeing", "Wellbeing prompts", "Gratitude, kindness, and mindfulness nudges.", []string{"affirmations", "advice_slip", "api_ninjas_quotes_positive"}, "wellbeing", false},
	{"family", "Family & home prompts", "Conversation starters, household nudges, seasonal notes, and coffee thoughts.", []string{"advice_slip"}, "family", false},
	{"nsfw-jokes", "NSFW adult jokes", "Adult joke feed. Off by default.", []string{"jokeapi_nsfw"}, "nsfw", true},
}

var providerLabels = map[string]string{
	"icanhazdadjoke": "icanhazdadjoke", "jokeapi_safe": "JokeAPI / Sv443 safe mode", "api_ninjas_dadjokes": "API Ninjas Dad Jokes", "api_ninjas_jokes": "API Ninjas Jokes", "official_joke": "Official Joke API",
	"quotable": "Quotable", "quotable_mirror": "Quotable community mirror", "favqs": "FavQs QOTD", "zenquotes": "ZenQuotes", "api_ninjas_quotes": "API Ninjas Quotes", "typefit_quotes": "Type.fit Quotes", "dummyjson_quotes": "DummyJSON Quotes", "stoic_quotes": "Stoic Quotes", "thequoteshub": "The Quotes Hub",
	"uselessfacts": "Useless Facts", "api_ninjas_facts": "API Ninjas Facts", "catfact": "catfact.ninja", "meowfacts": "MeowFacts",
	"wikimedia_onthisday": "Wikipedia: On this day", "wikimedia_births": "Wikipedia: born this day", "opentdb": "Open Trivia DB",
	"api_ninjas_riddles": "API Ninjas Riddles", "riddles_api": "Riddles API",
	"affirmations": "Affirmations.dev", "advice_slip": "Advice Slip", "api_ninjas_quotes_positive": "API Ninjas positive quotes", "jokeapi_nsfw": "JokeAPI adult", "local": "local fallback",
}

var providerKeyEnv = map[string]string{
	"api_ninjas_dadjokes": "DASH_API_NINJAS_KEY", "api_ninjas_jokes": "DASH_API_NINJAS_KEY", "api_ninjas_quotes": "DASH_API_NINJAS_KEY", "api_ninjas_facts": "DASH_API_NINJAS_KEY", "api_ninjas_riddles": "DASH_API_NINJAS_KEY", "api_ninjas_quotes_positive": "DASH_API_NINJAS_KEY",
}

var localMessages = map[string][]string{
	"quotes":    {"A steady pace still gets there.", "Small progress is still progress.", "Start where you are; use what you have.", "Today gets easier when you take the next step.", "Breathe in. Reset. Continue.", "Quiet moments count too.", "Peace can be a plan.", "Soft starts are still starts.", "Home is built from tiny acts of care.", "The best days leave room for each other.", "Little traditions become big memories.", "This house runs on love and snacks.", "You don't have to see the whole staircase, just the next step.", "Rest is part of the work, not a break from it.", "Do the next right thing; the rest follows.", "A calm morning is a gift you give the day.", "Kindness costs nothing and returns more than it asks.", "The day is yours to shape, one small choice at a time.", "Slow is smooth, and smooth is fast.", "What you do today echoes into tomorrow.", "You are allowed to go slowly.", "A calm morning is a quiet thing worth protecting.", "Do a small kindness you do not have to.", "A tidy corner is a small, quiet victory.", "Gratitude is the shortest road to a good mood.", "A warm hello can carry a whole day.", "Finish one thing and the rest finds its shape.", "Water, rest, and patience are not laziness.", "Small steps taken daily go a very long way.", "The house breathes easier when the mind does."},
	"jokes":     {"Why did the calendar feel popular? It had a lot of dates.", "I only know 25 letters of the alphabet. I don't know y.", "Why did the scarecrow win? He was outstanding in his field.", "Parallel lines have so much in common. It is a shame they'll never meet.", "Why did the bicycle fall over? It was two-tired.", "What do clouds wear under their shorts? Thunderwear.", "Why can't your nose be twelve inches long? Then it would be a foot.", "What did one wall say to the other? I'll meet you at the corner.", "I told my wife she was drawing her eyebrows too high. She looked surprised.", "Why don't scientists trust atoms? Because they make up everything.", "I'm reading a book about anti-gravity. It's impossible to put down.", "What do you call a fake noodle? An impasta.", "Why did the coffee file a police report? It got mugged.", "I used to be a banker, but I lost interest.", "What's the best thing about Switzerland? I don't know, but the flag is a big plus.", "Why did the math book look sad? It had too many problems.", "What did the grape do when it got stepped on? It let out a little wine.", "I told my dad a joke about the kitchen sink. You'll never believe the punchline.", "Why did the picture go to jail? Because it was framed.", "What do you call a bear with no teeth? A gummy bear.", "Why did the tomato blush? Because it saw the salad dressing.", "What do you call cheese that is not yours? Nacho cheese.", "I would tell you a joke about construction, but I am still working on it.", "Why don't eggs tell jokes? They would crack each other up.", "What did the ocean say to the beach? Nothing, it just waved.", "Why did the golfer bring two pairs of pants? In case he got a hole in one.", "I used to think I was indecisive. Now I am not so sure.", "Why did the robot go on a diet? It had too much disk space."},
	"facts":     {"Honey never spoils when stored properly.", "Bananas are berries, botanically speaking.", "Octopuses have three hearts.", "A day on Venus is longer than a Venus year.", "Sound travels faster in water than in air.", "Water expands when it freezes.", "Lightning can heat nearby air hotter than the Sun's surface.", "The Moon is slowly drifting away from Earth.", "Wombat droppings are roughly cube-shaped.", "A group of flamingos is called a flamboyance.", "The shortest war in history lasted about 38 minutes.", "There are more possible chess games than atoms in the observable universe.", "Sea otters hold hands while they sleep so they don't drift apart.", "The Eiffel Tower can be more than 15 cm taller in summer as the metal expands.", "A single lightning bolt is about five times hotter than the surface of the Sun.", "The human brain uses roughly 20 percent of the body's energy while taking up only 2 percent of its weight.", "Blue whales have hearts the size of a small car.", "Tigers have striped skin, not just striped fur.", "A group of owls is called a parliament.", "The smallest bone in the human body is the stapes in the ear.", "A day on Neptune lasts about sixteen Earth hours.", "The longest recorded sneeze lasted over one minute.", "Elephants can recognize themselves in a mirror.", "A group of lions is called a pride.", "The heart of a blue whale can be heard from about a mile away."},
	"riddles":   {"What has hands but cannot clap? A clock.", "What gets wetter as it dries? A towel.", "What has many keys but opens no locks? A piano.", "What has a head, a tail, and no body? A coin.", "What can travel around the world while staying in a corner? A stamp.", "What has words but never speaks? A book.", "What has a bed but never sleeps? A river.", "What has a neck but no head? A bottle.", "What goes up but never comes down? Your age.", "What can fill a room but takes up no space? Light.", "What has a face and two hands but no arms or legs? A clock.", "What has cities but no houses, forests but no trees, and water but no fish? A map."},
	"wellbeing": {"Name one small thing that helped today.", "Thank someone for an ordinary kindness.", "Notice something that made the room better.", "What went right today?", "Send one encouraging message.", "Take three slow breaths.", "Name one thing you are looking forward to this week.", "Step outside for two minutes and just look up.", "Write down one thing you are grateful for right now.", "Drink a full glass of water before your next coffee.", "Text someone you have not spoken to in a while.", "Stretch for sixty seconds; your shoulders will thank you.", "What is one thing you can let go of today?", "Name one small win from the last few days."},
	"family":    {"What was the best part of your day?", "What are you looking forward to?", "What should we cook soon?", "What made you laugh lately?", "Water bottles to the sink.", "Check tomorrow's calendar before bedtime.", "What is one thing you want to try this weekend?", "Who should we call or visit this week?", "What snack should we pick up on the way home?", "What is one chore we can knock out together in ten minutes?", "What song should be on the playlist tonight?", "What is one thing you are proud of from this week?", "What should we plan for next month?", "What is one small thing that made today better?"},
	// The history pool intentionally holds prompts rather than facts: a local
	// fallback must never invent an anniversary that did not happen.
	"history": {"Look up what happened on this day in history together.", "Ask what your family was doing on this date ten years ago.", "Pick a year and find out what happened that day.", "Every date has a story; which one do you remember?", "What is one thing from this date you want to remember?", "Small anniversaries are worth noticing.", "Ask someone older what they remember about this month.", "What was in the news the year you were born?", "Name a day you would happily live again.", "Who in the family has a story from this exact date?", "Write down one thing that happened today.", "What happened this week in the town you grew up in?", "Look up one famous person born on this day.", "What is the oldest thing in this house?", "Pick a favorite year and ask everyone about it.", "What would you like this date to be remembered for?"},
	"trivia":  {"What is the largest planet in our solar system? Answer: Jupiter.", "What is the largest ocean on Earth? Answer: The Pacific Ocean.", "How many continents are there? Answer: Seven.", "How many inches are in a foot? Answer: Twelve.", "At what temperature does water freeze in Fahrenheit? Answer: 32 degrees.", "How many legs does a spider have? Answer: Eight.", "Which gas do plants take in to make food? Answer: Carbon dioxide.", "What is the tallest mountain above sea level? Answer: Mount Everest.", "How many minutes are in an hour? Answer: Sixty.", "How many days are in a leap year? Answer: 366.", "What is the chemical formula for water? Answer: H2O.", "Are bats mammals? Answer: Yes.", "Can penguins fly? Answer: No.", "How many players does a soccer team field? Answer: Eleven.", "What is the largest mammal on Earth? Answer: The blue whale.", "How many sides does a hexagon have? Answer: Six."},
	"nsfw":    {},
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
