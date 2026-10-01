package messages

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// messageOutboundUserAgent identifies every message-feed request. Dash-Go keeps
// one deliberately generic agent for all outbound message traffic: no version
// and no contact string, so shared public assets cannot leak household or
// release identity. Sources that ask for a descriptive agent get one, and an
// anonymous household refresh stays far inside their published rate limits.
const messageOutboundUserAgent = "Dash-Go (+local-kiosk)"

func decodeMessageJSON(body io.Reader, limit int64, dst any) error {
	return json.NewDecoder(io.LimitReader(body, limit)).Decode(dst)
}

// messageFetcher carries the per-refresh HTTP plumbing shared by every provider
// adapter. Keeping it separate lets each adapter file stay a plain
// "request -> text lines" function inside the source-navigability limit.
type messageFetcher struct {
	ctx    context.Context
	client *http.Client
	env    func(string) string
}

func (s *Service) newMessageFetcher(ctx context.Context) *messageFetcher {
	return &messageFetcher{ctx: ctx, client: &http.Client{Timeout: 5 * time.Second}, env: s.messageEnv}
}

// bounded returns a child context so a provider that makes several sequential
// calls can never exceed the message refresh budget.
func (f *messageFetcher) bounded(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(f.ctx, d)
}

func (f *messageFetcher) getJSONCtx(ctx context.Context, u string, headers map[string]string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", messageOutboundUserAgent)
	req.Header.Set("Accept", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := f.client.Do(req)
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

func (f *messageFetcher) getJSON(u string, headers map[string]string, dst any) error {
	return f.getJSONCtx(f.ctx, u, headers, dst)
}

func (f *messageFetcher) apiNinjas(path string, params url.Values) ([]map[string]any, error) {
	key := f.env("DASH_API_NINJAS_KEY")
	if key == "" {
		return nil, fmt.Errorf("missing API Ninjas key")
	}
	u := "https://api.api-ninjas.com" + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	var out []map[string]any
	err := f.getJSON(u, map[string]string{"X-Api-Key": key}, &out)
	return out, err
}

// fetchMessageProvider dispatches one provider id to its category-owned adapter.
// Every adapter returns plain text lines; trimming, clamping, and the ticker
// length limit are applied once in cleanMessageText, so a new provider cannot
// bypass the rotating-message limits.
func (s *Service) fetchMessageProvider(ctx context.Context, provider string, want int) ([]string, error) {
	f := s.newMessageFetcher(ctx)
	switch provider {
	case "icanhazdadjoke", "jokeapi_safe", "jokeapi_nsfw", "official_joke",
		"api_ninjas_jokes", "api_ninjas_dadjokes":
		return f.jokeProvider(provider, want)
	case "quotable", "quotable_mirror", "stoic_quotes", "thequoteshub", "favqs",
		"zenquotes", "typefit_quotes", "dummyjson_quotes",
		"api_ninjas_quotes", "api_ninjas_quotes_positive", "affirmations", "advice_slip":
		return f.quoteProvider(provider, want)
	case "uselessfacts", "api_ninjas_facts", "catfact", "meowfacts",
		"riddles_api", "api_ninjas_riddles":
		return f.factProvider(provider, want)
	case "wikimedia_onthisday", "wikimedia_births", "opentdb":
		return f.historyProvider(provider, want)
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
