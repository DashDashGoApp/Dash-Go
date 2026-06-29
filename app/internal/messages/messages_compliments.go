package messages

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func cleanTextLimit(v any, limit int) string {
	s := strings.TrimSpace(controlChars.ReplaceAllString(jsonutil.StringValue(v), " "))
	if len(s) > limit {
		s = s[:limit]
	}
	return s
}
func validMMDD(s string) bool {
	if !reMonthDay.MatchString(s) {
		return false
	}
	m, _ := strconv.Atoi(s[:2])
	d, _ := strconv.Atoi(s[3:])
	return m >= 1 && m <= 12 && d >= 1 && d <= 31
}
func validDateOrMMDD(s string) bool {
	if validMMDD(s) {
		return true
	}
	if !reISODate.MatchString(s) {
		return false
	}
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}
func validClock(s string) bool {
	if !reTimeOfDay.MatchString(s) {
		return false
	}
	p := strings.Split(s, ":")
	h, _ := strconv.Atoi(p[0])
	m, _ := strconv.Atoi(p[1])
	return h >= 0 && h <= 23 && m >= 0 && m <= 59
}
func normClock(s string) string {
	p := strings.Split(s, ":")
	h, _ := strconv.Atoi(p[0])
	m, _ := strconv.Atoi(p[1])
	return fmt.Sprintf("%02d:%02d", h, m)
}
func nowMillis() int64 { return time.Now().UnixNano() / int64(time.Millisecond) }

func (s *Service) complimentsPath() string { return filepath.Join(s.configDir, "compliments.json") }
func (s *Service) complimentsPayload() map[string]any {
	m := jsonutil.Map(s.readJSONDefault(s.complimentsPath(), map[string]any{}))
	if _, ok := m["messages"].([]any); !ok {
		m["messages"] = []any{}
	}
	if _, ok := m["removedDefaults"].([]any); !ok {
		m["removedDefaults"] = []any{}
	}
	if _, ok := m["defaultEdits"].(map[string]any); !ok {
		m["defaultEdits"] = map[string]any{}
	}
	if _, ok := m["version"]; !ok {
		m["version"] = 4
	}
	if _, ok := m["defaultsCleared"]; !ok {
		m["defaultsCleared"] = false
	}
	if _, ok := m["defaultsSeeded"]; !ok {
		m["defaultsSeeded"] = false
	}
	return m
}
func (s *Service) saveCompliments(m map[string]any) { _ = fileio.WriteJSON(s.complimentsPath(), m) }
func nextNumericID(items []any) int {
	mx := 0
	for _, raw := range items {
		if n := jsonutil.Int(jsonutil.Map(raw)["id"], 0); n > mx {
			mx = n
		}
	}
	return mx + 1
}
func cleanCompliment(body map[string]any, existing map[string]any) (map[string]any, error) {
	text := cleanTextLimit(body["text"], 300)
	if text == "" {
		return nil, errors.New("message text required")
	}
	out := map[string]any{"text": text, "weight": clamp(jsonutil.Int(body["weight"], jsonutil.Int(existing["weight"], 1)), 1, 10000)}
	if d := jsonutil.BodyString(body, "date"); d != "" {
		if !validMMDD(d) {
			return nil, errors.New("date must be MM-DD")
		}
		out["date"] = d
	}
	for _, k := range []string{"origin", "holiday", "share"} {
		if v, ok := body[k]; ok {
			out[k] = v
		} else if v, ok := existing[k]; ok {
			out[k] = v
		}
	}
	return out, nil
}
func defaultKeyFromBody(body map[string]any) string {
	if k := jsonutil.BodyString(body, "key"); k != "" {
		return k
	}
	if text := cleanTextLimit(body["text"], 300); text != "" {
		return fmt.Sprintf("%x", sha1.Sum([]byte(strings.ToLower(text))))[:12]
	}
	return ""
}
func (s *Service) handleCompliments(w http.ResponseWriter, path string, body map[string]any) {
	payload := s.complimentsPayload()
	items := jsonutil.List(payload["messages"])
	switch path {
	case "/api/compliments/add":
		item, err := cleanCompliment(body, map[string]any{})
		if err != nil {
			s.err(w, err.Error(), 400)
			return
		}
		item["id"] = nextNumericID(items)
		item["origin"] = strOr(item["origin"], "custom")
		items = append(items, item)
		payload["messages"] = items
		s.saveCompliments(payload)
		s.json(w, item)
	case "/api/compliments/update":
		id := jsonutil.Int(body["id"], 0)
		changed := false
		var item map[string]any
		for i, raw := range items {
			m := jsonutil.Map(raw)
			if jsonutil.Int(m["id"], 0) == id {
				n, err := cleanCompliment(body, m)
				if err != nil {
					s.err(w, err.Error(), 400)
					return
				}
				n["id"] = id
				items[i] = n
				item = n
				changed = true
				break
			}
		}
		if !changed {
			s.err(w, "unknown message id", 400)
			return
		}
		payload["messages"] = items
		s.saveCompliments(payload)
		s.json(w, item)
	case "/api/compliments/delete":
		id := jsonutil.Int(body["id"], 0)
		keep := []any{}
		for _, raw := range items {
			if jsonutil.Int(jsonutil.Map(raw)["id"], 0) != id {
				keep = append(keep, raw)
			}
		}
		if len(keep) == len(items) {
			s.err(w, "unknown message id", 400)
			return
		}
		payload["messages"] = keep
		s.saveCompliments(payload)
		s.json(w, map[string]any{"deleted": id})
	case "/api/compliments/import":
		raw := jsonutil.List(body["messages"])
		if len(items) > 0 {
			s.err(w, "messages already present — import is first-time only", 400)
			return
		}
		if len(raw) > 500 {
			s.err(w, "bad import payload", 400)
			return
		}
		newItems := []any{}
		for _, r := range raw {
			m := jsonutil.Map(r)
			item, err := cleanCompliment(m, map[string]any{})
			if err == nil {
				item["id"] = len(newItems) + 1
				newItems = append(newItems, item)
			}
		}
		payload["messages"] = newItems
		s.saveCompliments(payload)
		s.json(w, map[string]any{"imported": len(newItems), "messages": newItems})
	case "/api/compliments/defaults/toggle":
		key := defaultKeyFromBody(body)
		if key == "" {
			s.err(w, "missing default key", 400)
			return
		}
		removed := jsonutil.List(payload["removedDefaults"])
		out := []any{}
		found := false
		for _, v := range removed {
			if fmt.Sprint(v) == key {
				found = true
			} else {
				out = append(out, v)
			}
		}
		if !found {
			out = append(out, key)
		}
		payload["removedDefaults"] = out
		payload["defaultsCleared"] = false
		s.saveCompliments(payload)
		s.json(w, s.complimentsPayload())
	case "/api/compliments/defaults/remove-all":
		keys := jsonutil.List(body["keys"])
		payload["removedDefaults"] = keys
		payload["defaultsCleared"] = true
		s.saveCompliments(payload)
		s.json(w, s.complimentsPayload())
	case "/api/compliments/defaults/add-all":
		payload["removedDefaults"] = []any{}
		payload["defaultsCleared"] = false
		payload["defaultsSeeded"] = true
		s.saveCompliments(payload)
		s.json(w, s.complimentsPayload())
	case "/api/compliments/clear-defaults":
		payload["messages"] = func() []any {
			keep := []any{}
			for _, raw := range items {
				if jsonutil.Map(raw)["origin"] != "default" {
					keep = append(keep, raw)
				}
			}
			return keep
		}()
		payload["defaultsCleared"] = true
		payload["removedDefaults"] = []any{}
		s.saveCompliments(payload)
		s.json(w, map[string]any{"cleared": true, "kept": len(jsonutil.List(payload["messages"]))})
	case "/api/compliments/restore-defaults":
		raw := jsonutil.List(body["messages"])
		kept := []any{}
		for _, item := range items {
			if jsonutil.Map(item)["origin"] != "default" {
				kept = append(kept, item)
			}
		}
		next := nextNumericID(kept)
		for _, r := range raw {
			m := jsonutil.Map(r)
			m["origin"] = "default"
			item, err := cleanCompliment(m, map[string]any{})
			if err == nil {
				item["id"] = next
				next++
				kept = append(kept, item)
			}
		}
		payload["messages"] = kept
		payload["defaultsCleared"] = false
		payload["defaultsSeeded"] = true
		payload["removedDefaults"] = []any{}
		s.saveCompliments(payload)
		s.json(w, s.complimentsPayload())
	case "/api/compliments/reconcile-defaults":
		payload["defaultsSeeded"] = true
		s.saveCompliments(payload)
		s.json(w, map[string]any{"changed": 0, "messages": payload["messages"], "removedDefaults": payload["removedDefaults"], "defaultsCleared": payload["defaultsCleared"], "defaultsSeeded": payload["defaultsSeeded"], "defaultEdits": payload["defaultEdits"], "version": payload["version"]})
	}
}
