package messages

import (
	"encoding/json"
	"strings"
	"sync"

	"embed"
)

// local_messages.json is the reviewable "dictionary" of every local message
// pool: one entry per line, each with a src field carrying the verification
// URL for new facts/trivia/quotes (empty for long-standing entries). The
// server ignores src; it exists so a reviewer can check every addition.
//
//go:embed local_messages.json
var localMessagesFile embed.FS

type localEntry struct {
	Text string `json:"text"`
	Src  string `json:"src"`
}

// loadLocalMessagePools parses the embedded dictionary once. Entries are
// trimmed and blank lines are dropped; pool keys match messageCategory.Local.
var loadLocalMessagePools = sync.OnceValue(func() map[string][]string {
	var pools map[string][]localEntry
	if err := json.Unmarshal([]byte(localMessagesRaw()), &pools); err != nil {
		panic("messages: local_messages.json: " + err.Error())
	}
	out := make(map[string][]string, len(pools))
	for key, entries := range pools {
		list := make([]string, 0, len(entries))
		for _, entry := range entries {
			if text := strings.TrimSpace(entry.Text); text != "" {
				list = append(list, text)
			}
		}
		out[key] = list
	}
	return out
})

func localMessagesRaw() string {
	body, err := localMessagesFile.ReadFile("local_messages.json")
	if err != nil {
		panic("messages: embedded local_messages.json: " + err.Error())
	}
	return string(body)
}

// localMessages keeps the name and type (map[string][]string) every caller
// has always seen; it is now the parsed embedded dictionary.
var localMessages = loadLocalMessagePools()
