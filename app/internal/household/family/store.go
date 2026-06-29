package family

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func Default() map[string]any {
	return map[string]any{
		"schema": Schema,
		"settings": map[string]any{
			// New installations may surface urgent notes, but an existing
			// showPinnedOnDashboard preference is migrated below and preserved.
			"showUrgentAlertsOnDashboard": true,
		},
		"notes": []any{},
	}
}

func String(v any) string {
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func Text(v any, limit int) string {
	s := String(v)
	if len([]rune(s)) > limit {
		s = string([]rune(s)[:limit])
	}
	return s
}

func ID(v any) string { return strings.ReplaceAll(Text(v, 96), " ", "-") }

func Date(v any) string {
	s := jsonutil.StringValue(v)
	if len(s) != len("2006-01-02") {
		return ""
	}
	if _, err := time.ParseInLocation("2006-01-02", s, time.Local); err != nil {
		return ""
	}
	return s
}

func Stamp(v any) string {
	s := jsonutil.StringValue(v)
	if _, err := time.Parse(time.RFC3339, s); err != nil {
		return ""
	}
	return s
}

func ExpiryStamp(v any) string {
	stamp := Stamp(v)
	if stamp == "" {
		return ""
	}
	parsed, err := time.Parse(time.RFC3339, stamp)
	if err != nil {
		return ""
	}
	return parsed.In(time.Local).Format(time.RFC3339)
}

func ExpiryTime(v any) (time.Time, bool) {
	stamp := ExpiryStamp(v)
	if stamp == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339, stamp)
	if err != nil {
		return time.Time{}, false
	}
	return parsed.In(time.Local), true
}

func ExpiryAtEndOfLocalDate(day string) string {
	parsed, err := time.ParseInLocation("2006-01-02", day, time.Local)
	if err != nil {
		return ""
	}
	return parsed.AddDate(0, 0, 1).Format(time.RFC3339)
}

func ArchiveStamp(now time.Time) string { return now.In(time.Local).Format(time.RFC3339) }

func Expired(note map[string]any, now time.Time) bool {
	expiresAt, ok := ExpiryTime(note["expiresAt"])
	return ok && !now.Before(expiresAt)
}

func ArchiveTooOld(note map[string]any, cutoff time.Time) bool {
	if note["state"] != "archived" {
		return false
	}
	stamp, err := time.Parse(time.RFC3339, Stamp(note["archivedAt"]))
	return err == nil && stamp.In(time.Local).Before(cutoff)
}

func Priority(v any) string {
	if strings.ToLower(jsonutil.StringValue(v)) == "urgent" {
		return "urgent"
	}
	return "normal"
}

func State(v any) string {
	if strings.ToLower(jsonutil.StringValue(v)) == "archived" {
		return "archived"
	}
	return "active"
}

func Scope(v any) string {
	if strings.ToLower(jsonutil.StringValue(v)) == "direct" {
		return "direct"
	}
	return "household"
}

func PersonID(v any) string       { return ID(v) }
func PersonSnapshot(v any) string { return Text(v, 64) }

func Note(raw any, now time.Time) map[string]any {
	row := jsonutil.Map(raw)
	id := ID(row["id"])
	text := Text(row["text"], 320)
	if id == "" || text == "" {
		return nil
	}
	scope := Scope(row["scope"])
	state := State(row["state"])
	archivedAt := Stamp(row["archivedAt"])
	createdAt := Stamp(row["createdAt"])
	updatedAt := Stamp(row["updatedAt"])
	if createdAt == "" {
		createdAt = ArchiveStamp(now)
	}
	if updatedAt == "" {
		updatedAt = createdAt
	}
	note := map[string]any{
		"id": id, "text": text, "scope": scope, "priority": Priority(row["priority"]),
		"state": state, "createdAt": createdAt, "updatedAt": updatedAt,
	}
	if scope == "direct" {
		senderID := PersonID(row["senderPersonId"])
		recipientID := PersonID(row["recipientPersonId"])
		if senderID == "" || recipientID == "" || senderID == recipientID {
			return nil
		}
		note["senderPersonId"] = senderID
		note["senderNameSnapshot"] = PersonSnapshot(row["senderNameSnapshot"])
		note["recipientPersonId"] = recipientID
		note["recipientNameSnapshot"] = PersonSnapshot(row["recipientNameSnapshot"])
		note["recipientReadAt"] = Stamp(row["recipientReadAt"])
		note["recipientArchivedAt"] = Stamp(row["recipientArchivedAt"])
		note["recipientDeletedAt"] = Stamp(row["recipientDeletedAt"])
		note["withdrawnAt"] = Stamp(row["withdrawnAt"])
		note["pinned"] = false
		note["expiresAt"] = ""
		note["archivedAt"] = ""
		note["householdAcknowledgedAt"] = ""
		note["state"] = "active"
		return note
	}
	note["pinned"] = jsonutil.Truthy(row["pinned"])
	note["expiresAt"] = ExpiryStamp(row["expiresAt"])
	note["archivedAt"] = archivedAt
	note["senderPersonId"] = PersonID(row["senderPersonId"])
	note["senderNameSnapshot"] = PersonSnapshot(row["senderNameSnapshot"])
	note["recipientPersonId"] = ""
	note["recipientNameSnapshot"] = ""
	note["recipientReadAt"] = ""
	note["recipientArchivedAt"] = ""
	note["recipientDeletedAt"] = ""
	note["withdrawnAt"] = ""
	note["householdAcknowledgedAt"] = Stamp(row["householdAcknowledgedAt"])
	if state == "active" && Expired(note, now) {
		note["state"] = "archived"
		note["archivedAt"] = ArchiveStamp(now)
	}
	if note["state"] == "archived" && note["archivedAt"] == "" {
		note["archivedAt"] = ArchiveStamp(now)
	}
	return note
}

func urgentLess(a, b map[string]any) bool {
	aExpiry, aHasExpiry := ExpiryTime(a["expiresAt"])
	bExpiry, bHasExpiry := ExpiryTime(b["expiresAt"])
	if aHasExpiry != bHasExpiry {
		return aHasExpiry
	}
	if aHasExpiry && !aExpiry.Equal(bExpiry) {
		return aExpiry.Before(bExpiry)
	}
	aUpdated, aHasUpdated := ExpiryTime(a["updatedAt"])
	bUpdated, bHasUpdated := ExpiryTime(b["updatedAt"])
	if aHasUpdated && bHasUpdated && !aUpdated.Equal(bUpdated) {
		return aUpdated.After(bUpdated)
	}
	if aHasUpdated != bHasUpdated {
		return aHasUpdated
	}
	return fmt.Sprint(a["id"]) < fmt.Sprint(b["id"])
}

func Normalize(raw map[string]any, now time.Time) map[string]any {
	now = now.In(time.Local)
	out := Default()
	settings := jsonutil.Map(raw["settings"])
	showUrgent := true
	if _, ok := settings["showUrgentAlertsOnDashboard"]; ok {
		showUrgent = jsonutil.Truthy(settings["showUrgentAlertsOnDashboard"])
	} else if _, ok := settings["showPinnedOnDashboard"]; ok {
		showUrgent = jsonutil.Truthy(settings["showPinnedOnDashboard"])
	}
	out["settings"] = map[string]any{"showUrgentAlertsOnDashboard": showUrgent}
	seen := map[string]bool{}
	publicNotes := []any{}
	directNotes := []any{}
	cutoff := now.AddDate(0, 0, -ArchiveDays)
	for _, rawNote := range jsonutil.List(raw["notes"]) {
		note := Note(rawNote, now)
		if note == nil || seen[note["id"].(string)] {
			continue
		}
		seen[note["id"].(string)] = true
		if note["scope"] == "direct" {
			directNotes = append(directNotes, note)
			continue
		}
		if ArchiveTooOld(note, cutoff) {
			continue
		}
		publicNotes = append(publicNotes, note)
	}
	slices.SortStableFunc(publicNotes, func(left, right any) int {
		a, b := jsonutil.Map(left), jsonutil.Map(right)
		if a["state"] != b["state"] {
			return boolTrueFirst(a["state"] == "active", b["state"] == "active")
		}
		if a["state"] == "active" {
			if au, bu := a["priority"] == "urgent", b["priority"] == "urgent"; au != bu {
				return boolTrueFirst(au, bu)
			}
			if ap, bp := jsonutil.Truthy(a["pinned"]), jsonutil.Truthy(b["pinned"]); ap != bp {
				return boolTrueFirst(ap, bp)
			}
		}
		return -compareText(a["updatedAt"], b["updatedAt"])
	})
	slices.SortStableFunc(directNotes, func(left, right any) int {
		return -compareText(jsonutil.Map(left)["updatedAt"], jsonutil.Map(right)["updatedAt"])
	})
	active, archived := 0, 0
	bounded := []any{}
	for _, note := range publicNotes {
		if jsonutil.Map(note)["state"] == "active" {
			if active >= 100 {
				continue
			}
			active++
		} else {
			if archived >= 300 {
				continue
			}
			archived++
		}
		bounded = append(bounded, note)
	}
	// Direct messages have no automatic retention purge. Unread messages must
	// never disappear merely because time passed. Creation is bounded per inbox.
	bounded = append(bounded, directNotes...)
	out["notes"] = bounded
	return out
}

func Equal(left, right map[string]any, _ time.Time) bool {
	// JSON parsing represents numbers as float64 while normalized defaults use
	// Go ints. Compare canonical JSON bytes so a stable Board is not rewritten
	// on every GET merely because decoded numeric types differ.
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftJSON, rightJSON)
}

func boolTrueFirst(left, right bool) int {
	if left == right {
		return 0
	}
	if left {
		return -1
	}
	return 1
}

func compareText(left, right any) int {
	a, b := strings.TrimSpace(fmt.Sprint(left)), strings.TrimSpace(fmt.Sprint(right))
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
