package messages

import (
	"fmt"
	"math/rand"
	"net/url"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// opentdbSafeCategories restricts trivia to household-safe subjects. The
// unfiltered pool spans categories a family kiosk should not rotate, and Open
// Trivia DB accepts only one category per request.
var opentdbSafeCategories = []int{17, 18, 19, 20, 21, 22, 23, 27, 28}

// historyProvider owns today's-date content: curated Wikipedia "on this day"
// entries, births, and safe trivia. All of these are keyless public sources.
func (f *messageFetcher) historyProvider(provider string, want int) ([]string, error) {
	switch provider {
	case "wikimedia_onthisday":
		return f.wikimediaDay("selected", want)
	case "wikimedia_births":
		return f.wikimediaDay("births", want)
	case "opentdb":
		category := opentdbSafeCategories[rand.Intn(len(opentdbSafeCategories))]
		var data map[string]any
		err := f.getJSON(fmt.Sprintf("https://opentdb.com/api.php?amount=%d&category=%d&type=multiple&encode=url3986", clamp(want, 1, 10), category), nil, &data)
		if err != nil {
			return nil, err
		}
		// 0 is success. 1 no results, 2 invalid parameter, 3 bad token,
		// 4 exhausted token, 5 rate limit (one request per 5 seconds per IP).
		if code := jsonutil.Int(data["response_code"], 0); code != 0 {
			return nil, fmt.Errorf("Open Trivia DB response code %d", code)
		}
		out := []string{}
		for _, raw := range jsonutil.List(data["results"]) {
			row := jsonutil.Map(raw)
			question := opentdbURLText(row["question"])
			answer := opentdbURLText(row["correct_answer"])
			if question != "" && answer != "" {
				out = append(out, question+" Answer: "+answer)
			}
		}
		return out, nil
	}
	return nil, fmt.Errorf("unknown history provider %s", provider)
}

// wikimediaDay reads one Wikimedia "on this day" feed. The feed is keyless and
// receives Dash-Go's shared message user agent. Entries are sampled so a busy
// date rotates through the section instead of repeating the same first rows.
func (f *messageFetcher) wikimediaDay(feed string, want int) ([]string, error) {
	now := time.Now()
	var data map[string]any
	u := fmt.Sprintf("https://api.wikimedia.org/feed/v1/wikipedia/en/onthisday/%s/%02d/%02d", feed, int(now.Month()), now.Day())
	if err := f.getJSON(u, nil, &data); err != nil {
		return nil, err
	}
	out := []string{}
	for _, raw := range jsonutil.List(data[feed]) {
		row := jsonutil.Map(raw)
		text := cleanMessageText(row["text"])
		if text == "" {
			continue
		}
		if year := cleanMessageText(jsonutil.TextValue(row["year"])); year != "" {
			text = year + " — " + text
		}
		out = append(out, text)
	}
	return sampleMessageTexts(out, want), nil
}

// opentdbURLText decodes the API's url3986 encoding. PathUnescape is deliberate:
// QueryUnescape would turn a literal plus in a maths question into a space.
func opentdbURLText(v any) string {
	raw := jsonutil.StringValue(v)
	if raw == "" {
		return ""
	}
	if decoded, err := url.PathUnescape(raw); err == nil {
		return cleanMessageText(decoded)
	}
	return cleanMessageText(raw)
}

// sampleMessageTexts returns a bounded, rotating slice of a larger feed.
func sampleMessageTexts(values []string, want int) []string {
	want = max(1, want)
	if len(values) <= want {
		return values
	}
	shuffled := append([]string{}, values...)
	rand.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
	return shuffled[:want]
}
