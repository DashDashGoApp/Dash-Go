package messages

import (
	"strings"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func normalizeDefaultMessageKey(value any) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(jsonutil.StringValue(value))), " "))
}

func defaultCatalogAliases(raw []any) map[string]string {
	aliases := map[string]string{}
	for _, item := range raw {
		row := jsonutil.Map(item)
		target := normalizeDefaultMessageKey(row["text"])
		if target == "" {
			continue
		}
		aliases[target] = target
		for _, legacy := range jsonutil.List(row["legacyKeys"]) {
			key := normalizeDefaultMessageKey(legacy)
			if key != "" && key != target {
				aliases[key] = target
			}
		}
	}
	return aliases
}

func defaultKeyAliasesFromBody(body map[string]any) []string {
	out, seen := []string{}, map[string]bool{}
	add := func(value any) {
		key := normalizeDefaultMessageKey(value)
		if key != "" && !seen[key] {
			seen[key] = true
			out = append(out, key)
		}
	}
	add(body["key"])
	for _, value := range jsonutil.List(body["legacyKeys"]) {
		add(value)
	}
	return out
}

// reconcileDefaultCatalog moves only catalog-owned state from replaced default
// keys to their new text keys. It never rewrites the user's custom messages.
func reconcileDefaultCatalog(payload map[string]any, catalog []any) int {
	aliases := defaultCatalogAliases(catalog)
	if len(aliases) == 0 {
		return 0
	}
	changed := 0
	removed, seen := []any{}, map[string]bool{}
	for _, value := range jsonutil.List(payload["removedDefaults"]) {
		key := normalizeDefaultMessageKey(value)
		if replacement := aliases[key]; replacement != "" {
			if replacement != key {
				changed++
			}
			key = replacement
		}
		if key == "" || seen[key] {
			changed++
			continue
		}
		seen[key] = true
		removed = append(removed, key)
	}
	if changed > 0 || len(removed) != len(jsonutil.List(payload["removedDefaults"])) {
		payload["removedDefaults"] = removed
	}

	edits := jsonutil.Map(payload["defaultEdits"])
	if len(edits) == 0 {
		return changed
	}
	next := map[string]any{}
	// Current keys win over migrated legacy copies if both happen to exist.
	for key, value := range edits {
		normalized := normalizeDefaultMessageKey(key)
		if aliases[normalized] == normalized && normalized != "" {
			next[normalized] = value
		}
	}
	for key, value := range edits {
		normalized := normalizeDefaultMessageKey(key)
		target := aliases[normalized]
		if target == "" {
			target = normalized
		}
		if target == "" {
			changed++
			continue
		}
		if target != normalized {
			changed++
		}
		if _, exists := next[target]; !exists {
			next[target] = value
		}
	}
	if changed > 0 || len(next) != len(edits) {
		payload["defaultEdits"] = next
	}
	return changed
}
