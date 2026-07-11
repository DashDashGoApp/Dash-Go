package events

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
)

func (s *Service) serializeEvent(ev ICSEvent, cal CalendarSource) map[string]any {
	uid := ev.UID
	startMs := epochMs(ev.Start)
	var endAny any = nil
	if ev.End != nil {
		endAny = epochMs(*ev.End)
	}
	identityParts := []string{cal.URL, uid, strconv.FormatInt(startMs, 10)}
	if uid == "" {
		endID := int64(0)
		if ev.End != nil {
			endID = epochMs(*ev.End)
		}
		identityParts = append(identityParts, strconv.FormatInt(endID, 10), ev.Title, ev.Location)
	}
	h := sha256.Sum256([]byte(strings.Join(identityParts, "|")))
	ident := hex.EncodeToString(h[:])[:16]
	owner := strings.TrimSpace(ev.AppOwner)
	if owner == "" {
		owner = strings.TrimSpace(cal.Owner)
	}
	calMeta := map[string]any{"url": cal.URL, "name": cal.Name, "color": cal.Color, "tag": cal.Tag}
	if owner != "" {
		calMeta["owner"] = owner
	}
	item := map[string]any{"id": ident, "title": defaultString(ev.Title, "(no title)"), "desc": ev.Desc, "location": ev.Location, "start": startMs, "end": endAny, "allDay": ev.AllDay, "uid": uid, "calUrl": cal.URL, "cal": calMeta}
	// Calendar edit eligibility is intrinsic event metadata. Keep it in the
	// cache whenever a source is a registered private vdir mirror, even when the
	// master Dashboard-edit switch is currently off. Popup controls then combine
	// this safe static eligibility with the current local writeback status, so a
	// control change cannot strand a previously cached event without actions.
	if s.knownWritebackSource(cal.URL) && owner == "" && uid != "" && !ev.HasScheduling {
		canEdit := !ev.Recur && ev.RecurID == nil
		canOccurrence := ev.Recur || ev.RecurID != nil
		occurrenceMS := startMs
		if ev.RecurID != nil {
			occurrenceMS = *ev.RecurID
		}
		seriesRule := false
		if ev.Recur && ev.RecurID == nil {
			seriesRule = simpleSeriesRule(ev)
		}
		item["writeback"] = map[string]any{
			"candidate":         true,
			"canEdit":           canEdit,
			"canOccurrenceEdit": canOccurrence,
			"canSeriesEdit":     seriesRule,
			"canSkip":           ev.Recur && ev.RecurID == nil,
			"hasExclusions":     len(ev.Exdates) > 0 || len(ev.ExdateDays) > 0,
			"occurrenceMs":      occurrenceMS,
		}
	}
	if owner != "" {
		item["appOwner"] = owner
	}
	if kind := strings.TrimSpace(ev.Meta["X-DASHGO-MANAGED-SCHEDULE"]); kind != "" {
		item["managedSchedule"] = map[string]any{
			"type":        kind,
			"ruleId":      strings.TrimSpace(ev.Meta["X-DASHGO-SCHEDULE-RULE-ID"]),
			"nominalDate": strings.TrimSpace(ev.Meta["X-DASHGO-NOMINAL-DATE"]),
			"actualDate":  strings.TrimSpace(ev.Meta["X-DASHGO-SCHEDULE-ACTUAL-DATE"]),
			"reason":      strings.TrimSpace(ev.Meta["X-DASHGO-SCHEDULE-REASON"]),
		}
	}
	return item
}

// simpleSeriesRule matches the deliberate server-side series-edit scope. The
// full RRULE remains untouched during a field edit; selector-heavy or RDATE
// series can still manage one occurrence but stay provider-managed as series.
func simpleSeriesRule(ev ICSEvent) bool {
	if strings.TrimSpace(ev.RRule) == "" || len(ev.Rdates) != 0 {
		return false
	}
	rule := parseRRule(ev.RRule)
	for key := range rule {
		switch key {
		case "FREQ", "INTERVAL", "COUNT", "UNTIL":
		default:
			return false
		}
	}
	switch rule["FREQ"] {
	case "DAILY", "WEEKLY", "MONTHLY", "YEARLY":
		return true
	default:
		return false
	}
}

func compareInt64(left, right int64) int {
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}
