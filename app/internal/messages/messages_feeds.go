package messages

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func (s *Service) messageCachePath() string { return filepath.Join(s.configDir, "message-cache.json") }
func (s *Service) messageOverridesPath() string {
	return filepath.Join(s.configDir, "message-cache-overrides.json")
}

func (s *Service) saveMessageSourcesStatus(prefs map[string]any) {
	_ = fileio.WriteJSON(filepath.Join(s.configDir, "message-sources.json"), prefs)
}
func (s *Service) saveMessageCache(cache map[string]any) {
	s.messageWriteMu.Lock()
	defer s.messageWriteMu.Unlock()
	_ = fileio.WriteJSON(s.messageCachePath(), cache)
}
func (s *Service) saveMessageOverrides(ov map[string]any) {
	s.messageWriteMu.Lock()
	defer s.messageWriteMu.Unlock()
	_ = fileio.WriteJSON(s.messageOverridesPath(), ov)
}

// updateMessageState serializes the cache+override read-modify-write pair.
// The two files are atomically replaced individually; this lock prevents a
// concurrent Control tab or refresh from reading one base state and clobbering
// another editor's change.
func (s *Service) updateMessageState(mut func(cache, overrides map[string]any) error) error {
	s.messageWriteMu.Lock()
	defer s.messageWriteMu.Unlock()
	cache := jsonutil.Map(s.readJSONDefault(s.messageCachePath(), map[string]any{"items": []any{}, "generatedAt": 0, "sources": []any{}, "sourceStatus": []any{}}))
	overrides := s.messageOverrides()
	if err := mut(cache, overrides); err != nil {
		return err
	}
	if err := fileio.WriteJSON(s.messageOverridesPath(), overrides); err != nil {
		return err
	}
	if err := fileio.WriteJSON(s.messageCachePath(), cache); err != nil {
		return err
	}
	return nil
}

func (s *Service) deleteMessageItem(id string) error {
	return s.updateMessageState(func(cache, overrides map[string]any) error {
		items := jsonutil.List(cache["items"])
		keep := make([]any, 0, len(items))
		found := false
		for _, raw := range items {
			if fmt.Sprint(jsonutil.Map(raw)["id"]) == id {
				found = true
				continue
			}
			keep = append(keep, raw)
		}
		if !found {
			return errors.New("unknown message item id")
		}
		removed := jsonutil.List(overrides["removed"])
		for _, raw := range removed {
			if fmt.Sprint(raw) == id {
				cache["items"] = keep
				return nil
			}
		}
		overrides["removed"] = append(removed, id)
		cache["items"] = keep
		return nil
	})
}

func (s *Service) updateMessageItem(id, text string, weight int) (map[string]any, error) {
	var item map[string]any
	err := s.updateMessageState(func(cache, overrides map[string]any) error {
		items := jsonutil.List(cache["items"])
		found := false
		for i, raw := range items {
			m := jsonutil.Map(raw)
			if fmt.Sprint(m["id"]) != id {
				continue
			}
			m["text"] = text
			m["weight"] = weight
			m["edited"] = true
			items[i] = m
			item = m
			found = true
			break
		}
		if !found {
			return errors.New("unknown message item id")
		}
		edits := jsonutil.Map(overrides["edits"])
		edits[id] = map[string]any{"text": text, "weight": weight}
		overrides["edits"] = edits
		cache["items"] = items
		return nil
	})
	return item, err
}
func (s *Service) handleMessages(w http.ResponseWriter, path string, body map[string]any) {
	status := s.messageSourcesStatus()
	cache := jsonutil.Map(status["cache"])
	switch path {
	case "/api/message-sources":
		enabled := s.normalizeMessageEnabled(jsonutil.List(body["enabled"]))
		prefs := map[string]any{"enabled": enabled, "updatedAt": nowMillis()}
		s.saveMessageSourcesStatus(prefs)
		if jsonutil.Truthy(body["refresh"]) {
			cache = s.refreshMessages(context.Background(), !jsonutil.Truthy(body["localOnly"]), true)
		}
		s.json(w, map[string]any{"ok": true, "status": s.messageSourcesStatus(), "cache": cache})
	case "/api/message-sources/refresh":
		cache = s.refreshMessages(context.Background(), !jsonutil.Truthy(body["localOnly"]), true)
		s.json(w, map[string]any{"ok": true, "cache": cache, "status": s.messageSourcesStatus()})
	case "/api/message-sources/item/delete":
		id := jsonutil.BodyString(body, "id")
		if id == "" {
			s.err(w, "missing message item id", 400)
			return
		}
		if err := s.deleteMessageItem(id); err != nil {
			s.err(w, err.Error(), 400)
			return
		}
		s.json(w, map[string]any{"deleted": id, "status": s.messageSourcesStatus()})
	case "/api/message-sources/item/update":
		id := jsonutil.BodyString(body, "id")
		if id == "" {
			s.err(w, "missing message item id", 400)
			return
		}
		text := cleanTextLimit(body["text"], 300)
		if text == "" {
			s.err(w, "message text required", 400)
			return
		}
		weight := clamp(jsonutil.Int(body["weight"], 1), 1, 10000)
		item, err := s.updateMessageItem(id, text, weight)
		if err != nil {
			s.err(w, err.Error(), 400)
			return
		}
		s.json(w, map[string]any{"item": item, "status": s.messageSourcesStatus()})
	case "/api/temporary-messages/add":
		item, err := s.addTempMessage(body)
		if err != nil {
			s.err(w, err.Error(), 400)
			return
		}
		s.json(w, item)
	case "/api/temporary-messages/delete":
		id := jsonutil.Int(body["id"], -1)
		items := jsonutil.List(s.readJSONDefault(filepath.Join(s.configDir, "temp-messages.json"), []any{}))
		keep := []any{}
		for _, raw := range items {
			if jsonutil.Int(jsonutil.Map(raw)["id"], -2) != id {
				keep = append(keep, raw)
			}
		}
		_ = fileio.WriteJSON(filepath.Join(s.configDir, "temp-messages.json"), keep)
		if len(keep) == len(items) {
			s.err(w, "unknown temporary message id", 400)
			return
		}
		s.json(w, map[string]any{"deleted": id})
	case "/api/scheduled-messages/add":
		item, err := s.cleanScheduled(body, nil)
		if err != nil {
			s.err(w, err.Error(), 400)
			return
		}
		items := jsonutil.List(s.readJSONDefault(filepath.Join(s.configDir, "scheduled-messages.json"), []any{}))
		item["id"] = nextNumericID(items)
		item["createdAt"] = nowMillis()
		items = append(items, item)
		_ = fileio.WriteJSON(filepath.Join(s.configDir, "scheduled-messages.json"), items)
		s.json(w, item)
	case "/api/scheduled-messages/update":
		id := jsonutil.Int(body["id"], 0)
		items := jsonutil.List(s.readJSONDefault(filepath.Join(s.configDir, "scheduled-messages.json"), []any{}))
		found := false
		var item map[string]any
		for i, raw := range items {
			old := jsonutil.Map(raw)
			if jsonutil.Int(old["id"], 0) == id {
				n, err := s.cleanScheduled(body, old)
				if err != nil {
					s.err(w, err.Error(), 400)
					return
				}
				n["id"] = id
				n["createdAt"] = old["createdAt"]
				items[i] = n
				item = n
				found = true
				break
			}
		}
		if !found {
			s.err(w, "unknown scheduled message id", 400)
			return
		}
		_ = fileio.WriteJSON(filepath.Join(s.configDir, "scheduled-messages.json"), items)
		s.json(w, item)
	case "/api/scheduled-messages/delete":
		id := jsonutil.Int(body["id"], -1)
		items := jsonutil.List(s.readJSONDefault(filepath.Join(s.configDir, "scheduled-messages.json"), []any{}))
		keep := []any{}
		for _, raw := range items {
			if jsonutil.Int(jsonutil.Map(raw)["id"], -2) != id {
				keep = append(keep, raw)
			}
		}
		_ = fileio.WriteJSON(filepath.Join(s.configDir, "scheduled-messages.json"), keep)
		if len(keep) == len(items) {
			s.err(w, "unknown scheduled message id", 400)
			return
		}
		s.json(w, map[string]any{"deleted": id})
	}
}
func (s *Service) addTempMessage(body map[string]any) (map[string]any, error) {
	text := cleanTextLimit(body["text"], 260)
	if text == "" {
		return nil, errors.New("message text required")
	}
	expires := jsonutil.BodyString(body, "expires")
	m := reFeedExpiry.FindStringSubmatch(expires)
	if len(m) < 2 {
		return nil, errors.New("expiry must be one of 30m, 60m, 120m, 360m, 720m, or 1440m")
	}
	mins, _ := strconv.Atoi(m[1])
	items := jsonutil.List(s.readJSONDefault(filepath.Join(s.configDir, "temp-messages.json"), []any{}))
	fresh := []any{}
	now := nowMillis()
	for _, raw := range items {
		if int64(jsonutil.Int(jsonutil.Map(raw)["expiresAt"], 0)) > now {
			fresh = append(fresh, raw)
		}
	}
	item := map[string]any{"id": nextNumericID(fresh), "text": text, "weight": clamp(jsonutil.Int(body["weight"], 500), 1, 10000), "createdAt": now, "expiresAt": now + int64(mins)*60000, "temporary": true}
	fresh = append(fresh, item)
	_ = fileio.WriteJSON(filepath.Join(s.configDir, "temp-messages.json"), fresh)
	return item, nil
}
func (s *Service) cleanScheduled(body map[string]any, old map[string]any) (map[string]any, error) {
	merged := make(map[string]any, len(old)+len(body))
	maps.Copy(merged, old)
	for k, v := range body {
		merged[k] = v
	}
	text := cleanTextLimit(merged["text"], 260)
	if text == "" {
		return nil, errors.New("message text required")
	}
	rec := strings.ToLower(strOr(merged["recurrence"], "once"))
	allowed := map[string]bool{"once": true, "daily": true, "weekly": true, "biweekly": true, "xweeks": true, "monthly": true, "xmonths": true, "yearly": true}
	if !allowed[rec] {
		return nil, errors.New("unknown recurrence")
	}
	startDate := strOr(merged["startDate"], "")
	if !validDateOrMMDD(startDate) || len(startDate) != 10 {
		return nil, errors.New("start date must be YYYY-MM-DD")
	}
	startTime := strOr(merged["startTime"], "")
	endTime := strOr(merged["endTime"], "")
	if !validClock(startTime) {
		return nil, errors.New("start time must be HH:MM")
	}
	if !validClock(endTime) {
		return nil, errors.New("end time must be HH:MM")
	}
	out := map[string]any{"text": text, "weight": clamp(jsonutil.Int(merged["weight"], 25), 1, 10000), "recurrence": rec, "startDate": startDate, "startTime": normClock(startTime), "endTime": normClock(endTime)}
	if ed := jsonutil.StringValue(merged["endDate"]); ed != "" {
		if !validDateOrMMDD(ed) || len(ed) != 10 {
			return nil, errors.New("end date must be YYYY-MM-DD")
		}
		out["endDate"] = ed
	} else if rec == "once" {
		out["endDate"] = startDate
	}
	if arr, ok := merged["days"].([]any); ok {
		days := []any{}
		seen := map[int]bool{}
		for _, d := range arr {
			n := jsonutil.Int(d, -1)
			if n >= 0 && n <= 6 && !seen[n] {
				seen[n] = true
				days = append(days, n)
			}
		}
		if (rec == "weekly" || rec == "biweekly" || rec == "xweeks") && len(days) == 0 {
			return nil, errors.New("choose at least one weekday")
		}
		if len(days) > 0 {
			out["days"] = days
		}
	}
	if rec == "xweeks" {
		out["intervalWeeks"] = clamp(jsonutil.Int(merged["intervalWeeks"], 3), 3, 4)
	}
	if rec == "xmonths" {
		out["intervalMonths"] = clamp(jsonutil.Int(merged["intervalMonths"], 2), 2, 11)
	}
	return out, nil
}
