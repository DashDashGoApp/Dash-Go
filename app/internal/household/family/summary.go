package family

import (
	"slices"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func PublicNotes(payload map[string]any) []any {
	rows := []any{}
	for _, raw := range jsonutil.List(payload["notes"]) {
		note := jsonutil.Map(raw)
		if note["scope"] == "household" {
			rows = append(rows, note)
		}
	}
	return rows
}

func PublicPayload(payload map[string]any) map[string]any {
	return map[string]any{
		"schema":   Schema,
		"settings": jsonutil.Map(payload["settings"]),
		"notes":    PublicNotes(payload),
	}
}

func Active(payload map[string]any) []map[string]any {
	out := []map[string]any{}
	for _, raw := range PublicNotes(payload) {
		note := jsonutil.Map(raw)
		if note["state"] == "active" {
			out = append(out, note)
		}
	}
	return out
}

func Summary(payload map[string]any) map[string]any {
	settings := jsonutil.Map(payload["settings"])
	showUrgent := jsonutil.Truthy(settings["showUrgentAlertsOnDashboard"])
	urgent := []map[string]any{}
	pinnedUrgent := []map[string]any{}
	for _, note := range Active(payload) {
		if note["priority"] != "urgent" || Stamp(note["householdAcknowledgedAt"]) != "" {
			continue
		}
		urgent = append(urgent, note)
		if jsonutil.Truthy(note["pinned"]) {
			pinnedUrgent = append(pinnedUrgent, note)
		}
	}
	slices.SortStableFunc(pinnedUrgent, func(left, right map[string]any) int {
		if urgentLess(left, right) {
			return -1
		}
		if urgentLess(right, left) {
			return 1
		}
		return 0
	})
	out := map[string]any{
		"showUrgentAlertsOnDashboard": showUrgent,
		"urgentCount":                 len(urgent),
		"pinnedUrgentCount":           len(pinnedUrgent),
		"displayMode":                 "none",
		"note":                        nil,
		"nextUrgentExpiryAt":          "",
	}
	if !showUrgent || len(urgent) == 0 {
		return out
	}
	if len(pinnedUrgent) > 0 {
		out["displayMode"] = "message"
		out["note"] = pinnedUrgent[0]
	} else {
		out["displayMode"] = "alert"
	}
	var earliest time.Time
	for _, note := range urgent {
		candidate, ok := ExpiryTime(note["expiresAt"])
		if !ok || (!earliest.IsZero() && !candidate.Before(earliest)) {
			continue
		}
		earliest = candidate
	}
	if !earliest.IsZero() {
		out["nextUrgentExpiryAt"] = earliest.Format(time.RFC3339)
	}
	return out
}

func Find(payload map[string]any, id string) (int, map[string]any) {
	for i, raw := range jsonutil.List(payload["notes"]) {
		note := jsonutil.Map(raw)
		if note["id"] == id {
			return i, note
		}
	}
	return -1, nil
}

func DirectCount(payload map[string]any, personID string) int {
	count := 0
	for _, raw := range jsonutil.List(payload["notes"]) {
		note := jsonutil.Map(raw)
		if note["scope"] == "direct" && PersonID(note["recipientPersonId"]) == personID && Stamp(note["recipientDeletedAt"]) == "" && Stamp(note["withdrawnAt"]) == "" {
			count++
		}
	}
	return count
}

func (s *Service) PersonMessageCount(personID string) int {
	personID = PersonID(personID)
	if personID == "" {
		return 0
	}
	payload, err := s.Read()
	if err != nil {
		return 0
	}
	count := 0
	for _, raw := range jsonutil.List(payload["notes"]) {
		note := jsonutil.Map(raw)
		if note["scope"] != "direct" {
			continue
		}
		if PersonID(note["senderPersonId"]) == personID || PersonID(note["recipientPersonId"]) == personID {
			count++
		}
	}
	return count
}

func (s *Service) MessageStillCurrent(messageID, personID string, private bool) bool {
	if messageID == "" {
		return true
	}
	s.Lock()
	defer s.Unlock()
	_, note := Find(s.PayloadLocked(), messageID)
	if note == nil {
		return false
	}
	if private {
		return note["scope"] == "direct" && PersonID(note["recipientPersonId"]) == personID && Stamp(note["withdrawnAt"]) == "" && Stamp(note["recipientDeletedAt"]) == ""
	}
	return note["scope"] == "household" && note["state"] == "active" && note["priority"] == "urgent"
}
