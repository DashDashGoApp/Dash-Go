package messages

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func (s *Service) loadBirthdays() []any {
	b, err := os.ReadFile(s.configLocal)
	if err != nil {
		return []any{}
	}
	re := reBirthdayList
	m := re.FindStringSubmatch(string(b))
	if len(m) < 2 {
		return []any{}
	}
	var arr []any
	if json.Unmarshal([]byte(m[1]), &arr) != nil {
		return []any{}
	}
	out := []any{}
	for i, raw := range arr {
		item := jsonutil.Map(raw)
		name := cleanTextLimit(item["name"], 80)
		date := strOr(item["date"], "")
		if name != "" && validMMDD(date) {
			id := jsonutil.Int(item["id"], i+1)
			out = append(out, map[string]any{"id": id, "name": name, "date": date})
		}
	}
	return out
}
func (s *Service) saveBirthdays(items []any) []any {
	clean := []any{}
	seen := map[int]bool{}
	next := 1
	for _, raw := range items {
		m := jsonutil.Map(raw)
		name := cleanTextLimit(m["name"], 80)
		date := strOr(m["date"], "")
		if name == "" || !validMMDD(date) {
			continue
		}
		id := jsonutil.Int(m["id"], 0)
		if id <= 0 || seen[id] {
			id = next
		}
		seen[id] = true
		if id >= next {
			next = id + 1
		}
		clean = append(clean, map[string]any{"id": id, "name": name, "date": date})
	}
	b, _ := os.ReadFile(s.configLocal)
	txt := string(b)
	if strings.TrimSpace(txt) == "" {
		txt = "window.DASHBOARD_LOCAL = {\n};\n"
	}
	arr, _ := json.Marshal(clean)
	pat := reBirthdayListLine
	if pat.MatchString(txt) {
		txt = pat.ReplaceAllString(txt, "${1}birthdays: "+string(arr)+",")
	} else {
		idx := strings.LastIndex(txt, "};")
		ins := "  birthdays: " + string(arr) + ",\n"
		if idx >= 0 {
			txt = txt[:idx] + ins + txt[idx:]
		} else {
			txt += "\n" + ins
		}
	}
	_ = os.MkdirAll(filepath.Dir(s.configLocal), 0755)
	_ = os.WriteFile(s.configLocal, []byte(txt), 0644)
	return clean
}
func (s *Service) loadCelebrations() []any {
	b, _ := os.ReadFile(s.celebrationsFile)
	out := []any{}
	for ln := range strings.SplitSeq(string(b), "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "#") || !strings.Contains(ln, "|") {
			continue
		}
		p := strings.SplitN(ln, "|", 2)
		date := strings.TrimSpace(p[0])
		label := cleanTextLimit(p[1], 120)
		if validDateOrMMDD(date) && label != "" {
			out = append(out, map[string]any{"id": len(out) + 1, "date": date, "label": label})
		}
	}
	return out
}
func (s *Service) saveCelebrations(items []any, refresh bool) []any {
	clean := []any{}
	for _, raw := range items {
		m := jsonutil.Map(raw)
		date := strOr(m["date"], "")
		label := cleanTextLimit(m["label"], 120)
		if validDateOrMMDD(date) && label != "" {
			clean = append(clean, map[string]any{"id": len(clean) + 1, "date": date, "label": label})
		}
	}
	var sb strings.Builder
	for _, raw := range clean {
		m := jsonutil.Map(raw)
		sb.WriteString(fmt.Sprintf("%s | %s\n", m["date"], m["label"]))
	}
	_ = os.WriteFile(s.celebrationsFile, []byte(sb.String()), 0600)
	if refresh {
		_, _ = s.generateDefaultCalendars(true)
	}
	return s.loadCelebrations()
}
func (s *Service) handleSpecialDates(w http.ResponseWriter, path string, body map[string]any) {
	switch path {
	case "/api/birthdays/add":
		name := cleanTextLimit(body["name"], 80)
		date := strOr(body["date"], "")
		if name == "" || !validMMDD(date) {
			s.err(w, "birthday needs a name and MM-DD date", 400)
			return
		}
		items := s.loadBirthdays()
		items = append(items, map[string]any{"id": nextNumericID(items), "name": name, "date": date})
		items = s.saveBirthdays(items)
		s.json(w, map[string]any{"item": items[len(items)-1], "items": items})
	case "/api/birthdays/update":
		id := jsonutil.Int(body["id"], 0)
		items := s.loadBirthdays()
		found := false
		var item any
		for _, raw := range items {
			m := jsonutil.Map(raw)
			if jsonutil.Int(m["id"], 0) == id {
				name := cleanTextLimit(body["name"], 80)
				date := strOr(body["date"], "")
				if name == "" || !validMMDD(date) {
					s.err(w, "birthday needs a name and MM-DD date", 400)
					return
				}
				m["name"] = name
				m["date"] = date
				item = m
				found = true
			}
		}
		if !found {
			s.err(w, "unknown birthday id", 400)
			return
		}
		items = s.saveBirthdays(items)
		s.json(w, map[string]any{"item": item, "items": items})
	case "/api/birthdays/delete":
		id := jsonutil.Int(body["id"], 0)
		old := s.loadBirthdays()
		keep := []any{}
		for _, raw := range old {
			if jsonutil.Int(jsonutil.Map(raw)["id"], 0) != id {
				keep = append(keep, raw)
			}
		}
		if len(keep) == len(old) {
			s.err(w, "unknown birthday id", 400)
			return
		}
		keep = s.saveBirthdays(keep)
		s.json(w, map[string]any{"deleted": id, "items": keep})
	case "/api/celebrations/add":
		label := cleanTextLimit(body["label"], 120)
		date := strOr(body["date"], "")
		if label == "" || !validDateOrMMDD(date) {
			s.err(w, "celebration needs MM-DD or YYYY-MM-DD date and a label", 400)
			return
		}
		items := s.loadCelebrations()
		items = append(items, map[string]any{"date": date, "label": label})
		items = s.saveCelebrations(items, true)
		s.json(w, map[string]any{"item": items[len(items)-1], "items": items})
	case "/api/celebrations/update":
		id := jsonutil.Int(body["id"], 0)
		items := s.loadCelebrations()
		found := false
		for _, raw := range items {
			m := jsonutil.Map(raw)
			if jsonutil.Int(m["id"], 0) == id {
				label := cleanTextLimit(body["label"], 120)
				date := strOr(body["date"], "")
				if label == "" || !validDateOrMMDD(date) {
					s.err(w, "celebration needs MM-DD or YYYY-MM-DD date and a label", 400)
					return
				}
				m["label"] = label
				m["date"] = date
				found = true
			}
		}
		if !found {
			s.err(w, "unknown celebration id", 400)
			return
		}
		items = s.saveCelebrations(items, true)
		s.json(w, map[string]any{"items": items})
	case "/api/celebrations/delete":
		id := jsonutil.Int(body["id"], 0)
		old := s.loadCelebrations()
		keep := []any{}
		for _, raw := range old {
			if jsonutil.Int(jsonutil.Map(raw)["id"], 0) != id {
				keep = append(keep, raw)
			}
		}
		if len(keep) == len(old) {
			s.err(w, "unknown celebration id", 400)
			return
		}
		keep = s.saveCelebrations(keep, true)
		s.json(w, map[string]any{"deleted": id, "items": keep})
	}
}
