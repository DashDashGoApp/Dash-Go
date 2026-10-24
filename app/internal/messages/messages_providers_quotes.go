package messages

import (
	"fmt"
	"net/url"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// quoteProvider owns the quotes and wellbeing categories: short reflective
// lines, affirmations, and advice-style nudges.
func (f *messageFetcher) quoteProvider(provider string, want int) ([]string, error) {
	switch provider {
	case "quotable":
		var rows []map[string]any
		err := f.getJSON("https://api.quotable.io/quotes/random?limit="+strconvI(clamp(want, 1, 10))+"&maxLength=220", nil, &rows)
		return quoteRowsTexts(rows), err
	case "quotable_mirror":
		// The community mirror has no maxLength filter and accepts only
		// limit 10/25/50/100, so the ticker length is enforced here instead.
		var data map[string]any
		err := f.getJSON("https://api.quotable.kurokeita.dev/api/quotes/random?limit=10", nil, &data)
		if err != nil {
			return nil, err
		}
		return mirrorQuoteTexts(jsonutil.List(data["quotes"]), want), nil
	case "stoic_quotes":
		// The apex domain 307-redirects to www.; pointing at www directly
		// saves a round trip on every refresh.
		var data map[string]any
		err := f.getJSON("https://www.stoic-quotes.com/api/quote", nil, &data)
		return namedQuoteTexts(data, 0), err
	case "thequoteshub":
		// This feed mixes short aphorisms with multi-sentence novel excerpts, so
		// anything longer than a ticker line is skipped rather than clamped.
		var data map[string]any
		err := f.getJSON("https://thequoteshub.com/api/random-quote", nil, &data)
		return namedQuoteTexts(data, tickerQuoteMaxRunes), err
	case "favqs":
		var data map[string]any
		err := f.getJSON("https://favqs.com/api/qotd", nil, &data)
		return quoteRowsTexts([]map[string]any{jsonutil.Map(data["quote"])}), err
	case "zenquotes":
		// /api/quotes returns ~50 attributed quotes per call; /api/random
		// returns one. The free tier asks for attribution, so the author
		// stays in the line. Use the plain-text field: the feed's HTML
		// variant leaks markup into the ticker line.
		var rows []map[string]any
		err := f.getJSON("https://zenquotes.io/api/quotes", nil, &rows)
		if err != nil {
			return nil, err
		}
		out := []string{}
		for _, r := range rows {
			body := cleanMessageText(r["q"])
			if body == "" {
				continue
			}
			if author := cleanMessageText(r["a"]); author != "" {
				body += " — " + author
			}
			out = append(out, body)
			if len(out) >= max(1, want) {
				break
			}
		}
		return out, nil
	case "typefit_quotes":
		var rows []map[string]any
		err := f.getJSON("https://type.fit/api/quotes", nil, &rows)
		if len(rows) > want {
			rows = rows[:want]
		}
		return quoteRowsTexts(rows), err
	case "dummyjson_quotes":
		var rows []map[string]any
		err := f.getJSON("https://dummyjson.com/quotes/random/"+strconvI(clamp(want, 1, 10)), nil, &rows)
		return quoteRowsTexts(rows), err
	case "affirmations":
		var data map[string]any
		err := f.getJSON("https://www.affirmations.dev/", nil, &data)
		return []string{cleanMessageText(data["affirmation"])}, err
	case "advice_slip":
		var data map[string]any
		err := f.getJSON("https://api.adviceslip.com/advice", nil, &data)
		return []string{cleanMessageText(jsonutil.Map(data["slip"])["advice"])}, err
	case "api_ninjas_quotes", "api_ninjas_quotes_positive":
		params := url.Values{}
		if provider == "api_ninjas_quotes_positive" {
			params.Set("category", "happiness")
		}
		rows, err := f.apiNinjas("/v2/randomquotes", params)
		return quoteRowsTexts(rows), err
	case "positive_api":
		// Keyless single-phrase service; last in the wellbeing chain so it
		// only serves when the established providers fail.
		var data map[string]any
		err := f.getJSON("https://positive-api.online/phrase", nil, &data)
		return []string{cleanMessageText(data["text"])}, err
	case "bored_api":
		// Kid-friendly household activity ideas; the API flags each one.
		var data map[string]any
		err := f.getJSON("https://bored-api.appbrewery.com/random", nil, &data)
		if err != nil {
			return nil, err
		}
		if !jsonutil.Truthy(data["kidFriendly"]) {
			return nil, nil
		}
		activity := cleanMessageText(data["activity"])
		if activity == "" {
			return nil, nil
		}
		return []string{"Idea: " + activity}, nil
	}
	return nil, fmt.Errorf("unknown quote provider %s", provider)
}

// tickerQuoteMaxRunes bounds a quote feed that mixes aphorisms with long prose.
const tickerQuoteMaxRunes = 240

// namedQuoteTexts renders a provider-specific {text, author} quote object in the
// same "line — author" shape the shared quote mapper produces. A maxRunes above
// zero skips entries that are too long for the ticker instead of clamping them.
func namedQuoteTexts(data map[string]any, maxRunes int) []string {
	body := cleanMessageText(firstMsgNonEmpty(data, "text", "quote", "content"))
	if body == "" || (maxRunes > 0 && len([]rune(body)) > maxRunes) {
		return nil
	}
	if author := cleanMessageText(firstMsgNonEmpty(data, "author", "a")); author != "" {
		body += " — " + author
	}
	return []string{body}
}

// mirrorQuoteTexts maps the community mirror's nested author object and applies
// the ticker-friendly length filter that mirror does not support itself.
func mirrorQuoteTexts(rows []any, want int) []string {
	const maxQuoteRunes = 220
	out := []string{}
	for _, raw := range rows {
		row := jsonutil.Map(raw)
		body := cleanMessageText(row["content"])
		if body == "" || len([]rune(body)) > maxQuoteRunes {
			continue
		}
		author := cleanMessageText(jsonutil.Map(row["author"])["name"])
		if author == "" {
			author = cleanMessageText(row["author"])
		}
		if author != "" {
			body += " — " + author
		}
		out = append(out, body)
		if len(out) >= max(1, want) {
			break
		}
	}
	return out
}
