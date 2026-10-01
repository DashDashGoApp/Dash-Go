package messages

import (
	"fmt"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// factProvider owns the fun-facts and riddles categories. Multi-call providers
// run under one bounded deadline so a slow endpoint cannot stall the refresh.
func (f *messageFetcher) factProvider(provider string, want int) ([]string, error) {
	switch provider {
	case "uselessfacts":
		out := []string{}
		bounded, cancel := f.bounded(12 * time.Second)
		defer cancel()
		for range clamp(want, 1, 4) {
			var data map[string]any
			if err := f.getJSONCtx(bounded, "https://uselessfacts.jsph.pl/api/v2/facts/random?language=en", nil, &data); err != nil {
				return out, err
			}
			if t := cleanMessageText(data["text"]); t != "" {
				out = append(out, t)
			}
		}
		return out, nil
	case "catfact":
		var data map[string]any
		err := f.getJSON("https://catfact.ninja/fact", nil, &data)
		return []string{cleanMessageText(data["fact"])}, err
	case "meowfacts":
		var data map[string]any
		err := f.getJSON("https://meowfacts.herokuapp.com/", nil, &data)
		return stringList(jsonutil.List(data["data"])), err
	case "api_ninjas_facts":
		rows, err := f.apiNinjas("/v1/facts", nil)
		return textsFromList(mapsToAny(rows), "fact"), err
	case "riddles_api":
		var data map[string]any
		err := f.getJSON("https://riddles-api.vercel.app/random", nil, &data)
		q := cleanMessageText(firstMsgNonEmpty(data, "riddle", "question", "title"))
		a := cleanMessageText(data["answer"])
		if q != "" && a != "" {
			q += " Answer: " + a
		}
		return []string{q}, err
	case "api_ninjas_riddles":
		rows, err := f.apiNinjas("/v1/riddles", nil)
		out := []string{}
		for _, r := range rows {
			q := cleanMessageText(r["question"])
			ans := cleanMessageText(r["answer"])
			if q != "" && ans != "" {
				out = append(out, q+" Answer: "+ans)
			}
		}
		return out, err
	}
	return nil, fmt.Errorf("unknown fact provider %s", provider)
}
