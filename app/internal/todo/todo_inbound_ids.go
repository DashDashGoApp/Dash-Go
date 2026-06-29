package todo

import "strings"

// todoUniqueMicrosoftListIDs keeps every inbound entry point bounded to known,
// cloud-enabled Microsoft lists and preserves caller order for predictable UI
// diagnostics.
func todoUniqueMicrosoftListIDs(a *Service, listIDs []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(listIDs))
	for _, listID := range listIDs {
		listID = strings.TrimSpace(listID)
		if listID == "" || seen[listID] || !a.todoListCloudSyncEnabled(listID) {
			continue
		}
		seen[listID] = true
		out = append(out, listID)
	}
	return out
}
