package todo

import "strings"

// todoMappedMicrosoftListIDs returns the two permanent App destinations when
// they are genuinely Microsoft-backed. These are the normal automatic inbound
// candidates; discovery alone must not make built-in/smart lists poll forever.
func (a *Service) todoMappedMicrosoftListIDs() []string {
	seen := map[string]bool{}
	out := make([]string, 0, 2)
	mapping := a.todoMap()
	for _, slot := range []string{"todo", "grocery"} {
		listID := strings.TrimSpace(mapping[slot])
		if listID == "" || seen[listID] || !a.todoListCloudSyncEnabled(listID) {
			continue
		}
		seen[listID] = true
		out = append(out, listID)
	}
	return out
}

// todoPendingMicrosoftListIDs includes a list with active local outbound work
// even if it is no longer one of the two launcher destinations. This is a
// bounded reconciliation exception, not a general "poll every discovered list"
// rule. Blocked writes are intentionally excluded until the user explicitly
// chooses Retry.
func (a *Service) todoPendingMicrosoftListIDs() []string {
	out := make([]string, 0)
	for _, raw := range a.readTodoListsIndex().Lists {
		item := normalizeTodoListInfo(raw)
		if item.ID == "" || todoListOriginOf(item) != todoListOriginMicrosoft || !a.todoListCloudSyncEnabled(item.ID) {
			continue
		}
		if todoActivePendingOpIndex(a.readTodoListCache(item.ID).PendingOps) >= 0 {
			out = append(out, item.ID)
		}
	}
	return out
}

// todoEstablishedCursorListIDs retains a bounded relationship with an
// additional Microsoft list once Dash-Go has actually opened or synced it. A
// discovered-but-untouched smart or built-in list has no cursor and remains
// excluded, so this does not become broad account polling.
func (a *Service) todoEstablishedCursorListIDs() []string {
	out := make([]string, 0)
	for _, raw := range a.readTodoListsIndex().Lists {
		item := normalizeTodoListInfo(raw)
		if item.ID == "" || todoListOriginOf(item) != todoListOriginMicrosoft || !a.todoListCloudSyncEnabled(item.ID) {
			continue
		}
		if strings.TrimSpace(a.readTodoListCache(item.ID).DeltaLink) != "" {
			out = append(out, item.ID)
		}
	}
	return out
}

// todoInboundMicrosoftListIDs deliberately keeps scheduled work small: mapped
// household lists, active locally queued work, and only Microsoft lists that
// Dash-Go has already established as tracked through a saved delta cursor.
func (a *Service) todoInboundMicrosoftListIDs() []string {
	requested := append(a.todoMappedMicrosoftListIDs(), a.todoPendingMicrosoftListIDs()...)
	requested = append(requested, a.todoEstablishedCursorListIDs()...)
	return todoUniqueMicrosoftListIDs(a, requested)
}

func (a *Service) todoInboundSyncReady() bool {
	return a.todoCloudSyncEnabled() && len(a.todoInboundMicrosoftListIDs()) > 0
}

func (a *Service) todoInboundSyncUnavailableReason() string {
	if !a.todoCloudSyncEnabled() {
		return "Microsoft To Do is not linked"
	}
	if len(a.todoInboundMicrosoftListIDs()) == 0 {
		return "Microsoft sync is linked, but To Do and Grocery are not mapped to a Microsoft list. Refresh available lists, then choose a Microsoft destination."
	}
	return ""
}

func (a *Service) todoInboundStatusLastError(lastError string) string {
	if lastError = strings.TrimSpace(lastError); lastError != "" {
		return lastError
	}
	return a.todoInboundSyncUnavailableReason()
}
