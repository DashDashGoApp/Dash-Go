package messages

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func decodeMessageJSON(body io.Reader, limit int64, dst any) error {
	return json.NewDecoder(io.LimitReader(body, limit)).Decode(dst)
}

func (s *Service) fetchMessageProvider(ctx context.Context, provider string, want int) ([]string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	getJSON := func(u string, headers map[string]string, dst any) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", "Dash-Go/1.3.5-beta.47 (+local-kiosk)")
		req.Header.Set("Accept", "application/json")
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		res, err := client.Do(req)
		if err != nil {
			return err
		}
		defer res.Body.Close()
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			b, _ := io.ReadAll(io.LimitReader(res.Body, 256))
			return fmt.Errorf("%s: %s", res.Status, strings.TrimSpace(string(b)))
		}
		return decodeMessageJSON(res.Body, 2<<20, dst)
	}
	apiNinjas := func(path string, params url.Values) ([]map[string]any, error) {
		key := s.messageEnv("DASH_API_NINJAS_KEY")
		if key == "" {
			return nil, fmt.Errorf("missing API Ninjas key")
		}
		u := "https://api.api-ninjas.com" + path
		if len(params) > 0 {
			u += "?" + params.Encode()
		}
		var out []map[string]any
		err := getJSON(u, map[string]string{"X-Api-Key": key}, &out)
		return out, err
	}
	switch provider {
	case "icanhazdadjoke":
		var data map[string]any
		err := getJSON("https://icanhazdadjoke.com/search?limit="+strconvI(clamp(want, 1, 20)), map[string]string{"Accept": "application/json"}, &data)
		return textsFromList(jsonutil.List(data["results"]), "joke"), err
	case "jokeapi_safe", "jokeapi_nsfw":
		vals := url.Values{"amount": {strconvI(clamp(want, 1, 10))}}
		if provider == "jokeapi_safe" {
			vals.Set("safe-mode", "")
			vals.Set("blacklistFlags", "nsfw,religious,political,racist,sexist,explicit")
		} else {
			vals.Set("blacklistFlags", "religious,political,racist,sexist")
		}
		var data map[string]any
		err := getJSON("https://v2.jokeapi.dev/joke/Any?"+vals.Encode(), nil, &data)
		return jokeAPITexts(data), err
	case "official_joke":
		var rows []map[string]any
		err := getJSON("https://official-joke-api.appspot.com/jokes/random/"+strconvI(clamp(want, 1, 10)), nil, &rows)
		return jokeRowsTexts(rows), err
	case "quotable":
		var rows []map[string]any
		err := getJSON("https://api.quotable.io/quotes/random?limit="+strconvI(clamp(want, 1, 10))+"&maxLength=220", nil, &rows)
		return quoteRowsTexts(rows), err
	case "favqs":
		var data map[string]any
		err := getJSON("https://favqs.com/api/qotd", nil, &data)
		return quoteRowsTexts([]map[string]any{jsonutil.Map(data["quote"])}), err
	case "zenquotes":
		var rows []map[string]any
		err := getJSON("https://zenquotes.io/api/random", nil, &rows)
		return quoteRowsTexts(rows), err
	case "typefit_quotes":
		var rows []map[string]any
		err := getJSON("https://type.fit/api/quotes", nil, &rows)
		if len(rows) > want {
			rows = rows[:want]
		}
		return quoteRowsTexts(rows), err
	case "dummyjson_quotes":
		var rows []map[string]any
		err := getJSON("https://dummyjson.com/quotes/random/"+strconvI(clamp(want, 1, 10)), nil, &rows)
		return quoteRowsTexts(rows), err
	case "uselessfacts":
		out := []string{}
		for range clamp(want, 1, 4) {
			var data map[string]any
			if err := getJSON("https://uselessfacts.jsph.pl/api/v2/facts/random?language=en", nil, &data); err != nil {
				return out, err
			}
			if t := cleanMessageText(data["text"]); t != "" {
				out = append(out, t)
			}
		}
		return out, nil
	case "catfact":
		var data map[string]any
		err := getJSON("https://catfact.ninja/fact", nil, &data)
		return []string{cleanMessageText(data["fact"])}, err
	case "meowfacts":
		var data map[string]any
		err := getJSON("https://meowfacts.herokuapp.com/", nil, &data)
		return stringList(jsonutil.List(data["data"])), err
	case "numbersapi_https":
		out := []string{}
		for range clamp(want, 1, 4) {
			var data map[string]any
			n := 1 + rand.Intn(366)
			if err := getJSON(fmt.Sprintf("https://numbersapi.com/%d/trivia?json", n), nil, &data); err != nil {
				return out, err
			}
			if t := cleanMessageText(data["text"]); t != "" {
				out = append(out, t)
			}
		}
		return out, nil
	case "riddles_api":
		var data map[string]any
		err := getJSON("https://riddles-api.vercel.app/random", nil, &data)
		q := cleanMessageText(firstMsgNonEmpty(data, "riddle", "question", "title"))
		a := cleanMessageText(data["answer"])
		if q != "" && a != "" {
			q += " Answer: " + a
		}
		return []string{q}, err
	case "affirmations":
		var data map[string]any
		err := getJSON("https://www.affirmations.dev/", nil, &data)
		return []string{cleanMessageText(data["affirmation"])}, err
	case "advice_slip":
		var data map[string]any
		err := getJSON("https://api.adviceslip.com/advice", nil, &data)
		return []string{cleanMessageText(jsonutil.Map(data["slip"])["advice"])}, err
	case "api_ninjas_jokes", "api_ninjas_dadjokes":
		path := "/v1/jokes"
		if provider == "api_ninjas_dadjokes" {
			path = "/v1/dadjokes"
		}
		rows, err := apiNinjas(path, nil)
		return textsFromList(mapsToAny(rows), "joke"), err
	case "api_ninjas_quotes", "api_ninjas_quotes_positive":
		params := url.Values{}
		if provider == "api_ninjas_quotes_positive" {
			params.Set("category", "happiness")
		}
		rows, err := apiNinjas("/v2/randomquotes", params)
		return quoteRowsTexts(rows), err
	case "api_ninjas_facts":
		rows, err := apiNinjas("/v1/facts", nil)
		return textsFromList(mapsToAny(rows), "fact"), err
	case "api_ninjas_riddles":
		rows, err := apiNinjas("/v1/riddles", nil)
		out := []string{}
		for _, r := range rows {
			q := cleanMessageText(r["question"])
			ans := cleanMessageText(r["answer"])
			if q != "" && ans != "" {
				out = append(out, q+" Answer: "+ans)
			}
		}
		return out, err
	case "dictionary_random_word", "freedictionary_random_word":
		return s.fetchRandomWordDefinition(ctx, client, provider)
	}
	return nil, fmt.Errorf("unknown provider %s", provider)
}

func strconvI(n int) string { return fmt.Sprintf("%d", n) }
func mapsToAny(rows []map[string]any) []any {
	out := make([]any, len(rows))
	for i := range rows {
		out[i] = rows[i]
	}
	return out
}
func stringList(vals []any) []string {
	out := []string{}
	for _, v := range vals {
		if t := cleanMessageText(v); t != "" {
			out = append(out, t)
		}
	}
	return out
}
func textsFromList(vals []any, key string) []string {
	out := []string{}
	for _, raw := range vals {
		if t := cleanMessageText(jsonutil.Map(raw)[key]); t != "" {
			out = append(out, t)
		}
	}
	return out
}
func firstMsgNonEmpty(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if t := cleanMessageText(m[k]); t != "" {
			return t
		}
	}
	return ""
}
func jokeRowsTexts(rows []map[string]any) []string {
	out := []string{}
	for _, r := range rows {
		if t := cleanMessageText(r["joke"]); t != "" {
			out = append(out, t)
			continue
		}
		setup := cleanMessageText(r["setup"])
		delivery := cleanMessageText(r["delivery"])
		punch := cleanMessageText(r["punchline"])
		if setup != "" && (delivery != "" || punch != "") {
			if delivery == "" {
				delivery = punch
			}
			out = append(out, setup+" "+delivery)
		}
	}
	return out
}
func jokeAPITexts(data map[string]any) []string {
	if rows := jsonutil.List(data["jokes"]); len(rows) > 0 {
		maps := []map[string]any{}
		for _, r := range rows {
			maps = append(maps, jsonutil.Map(r))
		}
		return jokeRowsTexts(maps)
	}
	return jokeRowsTexts([]map[string]any{data})
}
func quoteRowsTexts(rows []map[string]any) []string {
	out := []string{}
	for _, r := range rows {
		body := cleanMessageText(firstMsgNonEmpty(r, "content", "quote", "body", "text", "q"))
		auth := cleanMessageText(firstMsgNonEmpty(r, "author", "authorSlug", "a"))
		if body != "" {
			if auth != "" {
				body += " — " + auth
			}
			out = append(out, body)
		}
	}
	return out
}

func (s *Service) fetchRandomWordDefinition(ctx context.Context, client *http.Client, provider string) ([]string, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://random-word-api.herokuapp.com/word?number=1", nil)
	req.Header.Set("User-Agent", "Dash-Go/1.3.5-beta.47 (+local-kiosk)")
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	var words []string
	if err := decodeMessageJSON(res.Body, 1<<20, &words); err != nil {
		return nil, err
	}
	if len(words) == 0 {
		return nil, fmt.Errorf("random word unavailable")
	}
	word := strings.TrimSpace(words[0])
	if word == "" {
		return nil, fmt.Errorf("random word unavailable")
	}
	return []string{strings.Title(word) + " — a useful word to look up together."}, nil
}
