package messages

import (
	"fmt"
	"net/url"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// jokeProvider owns the jokes and NSFW-joke categories. Every branch returns
// plain text lines so the ticker limits stay in one place.
func (f *messageFetcher) jokeProvider(provider string, want int) ([]string, error) {
	switch provider {
	case "icanhazdadjoke":
		var data map[string]any
		err := f.getJSON("https://icanhazdadjoke.com/search?limit="+strconvI(clamp(want, 1, 20)), map[string]string{"Accept": "application/json"}, &data)
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
		err := f.getJSON("https://v2.jokeapi.dev/joke/Any?"+vals.Encode(), nil, &data)
		return jokeAPITexts(data), err
	case "official_joke":
		var rows []map[string]any
		err := f.getJSON("https://official-joke-api.appspot.com/jokes/random/"+strconvI(clamp(want, 1, 10)), nil, &rows)
		return jokeRowsTexts(rows), err
	case "api_ninjas_jokes", "api_ninjas_dadjokes":
		path := "/v1/jokes"
		if provider == "api_ninjas_dadjokes" {
			path = "/v1/dadjokes"
		}
		rows, err := f.apiNinjas(path, nil)
		return textsFromList(mapsToAny(rows), "joke"), err
	}
	return nil, fmt.Errorf("unknown joke provider %s", provider)
}
